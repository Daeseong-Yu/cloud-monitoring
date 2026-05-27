package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"cloud-monitor/internal/alert"
	"cloud-monitor/internal/sanitize"
)

type ruleRow struct {
	alert.Rule
	LookbackMinutes int
}

type metricValue struct {
	Value float64
	At    time.Time
}

func main() {
	once := flag.Bool("once", true, "run one alert evaluation cycle")
	flag.Parse()
	_ = once

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "alert runner configuration error: DATABASE_URL is required")
		os.Exit(2)
	}
	webhookURL := strings.TrimSpace(os.Getenv("SLACK_WEBHOOK_URL"))

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "alert runner database error: %s\n", sanitize.Message(err.Error(), databaseURL, webhookURL))
		os.Exit(1)
	}
	defer pool.Close()

	opened, resolved, err := runOnce(ctx, pool, webhookURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "alert runner error: %s\n", sanitize.Message(err.Error(), databaseURL, webhookURL))
		os.Exit(1)
	}
	fmt.Printf("alert runner completed: opened=%d resolved=%d\n", opened, resolved)
}

func runOnce(ctx context.Context, pool *pgxpool.Pool, webhookURL string) (int, int, error) {
	rules, err := loadRules(ctx, pool)
	if err != nil {
		return 0, 0, err
	}

	var opened int
	var resolved int
	for _, rule := range rules {
		value, ok, err := latestMetricValue(ctx, pool, rule)
		if err != nil {
			return opened, resolved, err
		}
		if !ok {
			continue
		}

		evaluation, err := alert.Evaluate(rule.Rule, value.Value)
		if err != nil {
			return opened, resolved, err
		}
		openEventID, hasOpen, err := openEvent(ctx, pool, rule.ID)
		if err != nil {
			return opened, resolved, err
		}

		if evaluation.Active && !hasOpen {
			message := sanitize.Message(evaluation.Message)
			if err := insertOpenEvent(ctx, pool, rule.ID, value.Value, rule.Threshold, message); err != nil {
				return opened, resolved, err
			}
			if webhookURL != "" {
				if err := sendSlack(ctx, webhookURL, message); err != nil {
					return opened, resolved, err
				}
			}
			opened++
		}
		if !evaluation.Active && hasOpen {
			message := sanitize.Message(evaluation.Message)
			if err := resolveEvent(ctx, pool, openEventID, value.Value, message); err != nil {
				return opened, resolved, err
			}
			if webhookURL != "" {
				if err := sendSlack(ctx, webhookURL, message); err != nil {
					return opened, resolved, err
				}
			}
			resolved++
		}
	}
	return opened, resolved, nil
}

func loadRules(ctx context.Context, pool *pgxpool.Pool) ([]ruleRow, error) {
	rows, err := pool.Query(ctx, `
SELECT
    id,
    name,
    region,
    namespace,
    resource_id,
    metric_name,
    statistic,
    operator,
    threshold,
    lookback_minutes
FROM alert_rules
WHERE enabled = TRUE
ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []ruleRow
	for rows.Next() {
		var rule ruleRow
		if err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&rule.Region,
			&rule.Namespace,
			&rule.ResourceID,
			&rule.MetricName,
			&rule.Statistic,
			&rule.Operator,
			&rule.Threshold,
			&rule.LookbackMinutes,
		); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func latestMetricValue(ctx context.Context, pool *pgxpool.Pool, rule ruleRow) (metricValue, bool, error) {
	var value metricValue
	err := pool.QueryRow(ctx, `
SELECT mp.value, mp.timestamp
FROM metric_points mp
JOIN metric_definitions md ON md.id = mp.metric_definition_id
WHERE md.enabled = TRUE
  AND md.region = $1
  AND md.namespace = $2
  AND md.resource_id = $3
  AND md.metric_name = $4
  AND md.statistic = $5
  AND mp.timestamp >= now() - ($6::text || ' minutes')::interval
ORDER BY mp.timestamp DESC
LIMIT 1`,
		rule.Region,
		rule.Namespace,
		rule.ResourceID,
		rule.MetricName,
		rule.Statistic,
		rule.LookbackMinutes,
	).Scan(&value.Value, &value.At)
	if err == pgx.ErrNoRows {
		return metricValue{}, false, nil
	}
	if err != nil {
		return metricValue{}, false, err
	}
	return value, true, nil
}

func openEvent(ctx context.Context, pool *pgxpool.Pool, ruleID int64) (int64, bool, error) {
	var id int64
	err := pool.QueryRow(ctx, `
SELECT id
FROM alert_events
WHERE alert_rule_id = $1
  AND status = 'open'
LIMIT 1`, ruleID).Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func insertOpenEvent(ctx context.Context, pool *pgxpool.Pool, ruleID int64, value float64, threshold float64, message string) error {
	_, err := pool.Exec(ctx, `
INSERT INTO alert_events (alert_rule_id, status, value, threshold, message)
VALUES ($1, 'open', $2, $3, $4)
ON CONFLICT DO NOTHING`, ruleID, value, threshold, message)
	return err
}

func resolveEvent(ctx context.Context, pool *pgxpool.Pool, eventID int64, value float64, message string) error {
	_, err := pool.Exec(ctx, `
UPDATE alert_events
SET status = 'resolved',
    value = $2,
    message = $3,
    resolved_at = now()
WHERE id = $1`, eventID, value, message)
	return err
}

func sendSlack(ctx context.Context, webhookURL string, message string) error {
	payload, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned status %d", response.StatusCode)
	}
	return nil
}
