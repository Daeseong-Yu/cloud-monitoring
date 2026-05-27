package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"cloud-monitor/internal/catalog"
)

func main() {
	configPath := flag.String("config", "", "metric definition catalog JSON path")
	dryRun := flag.Bool("dry-run", false, "print generated SQL without applying it")
	psqlBin := flag.String("psql-bin", "psql", "psql executable path")
	flag.Parse()

	if strings.TrimSpace(*configPath) == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}

	c, err := catalog.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric definition catalog error: %v\n", err)
		os.Exit(1)
	}

	definitions, err := c.Resolve(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric definition catalog error: %v\n", err)
		os.Exit(1)
	}

	sql := buildUpsertSQL(definitions)
	if *dryRun {
		fmt.Print(sql)
		return
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}

	cmd := exec.Command(*psqlBin, "--set", "ON_ERROR_STOP=1", databaseURL)
	cmd.Stdin = strings.NewReader(sql)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		fmt.Fprintf(os.Stderr, "metric definition sync failed: %s\n", message)
		os.Exit(1)
	}

	fmt.Printf("metric definition sync applied: definitions=%d\n", len(definitions))
}

func buildUpsertSQL(definitions []catalog.Definition) string {
	var b strings.Builder
	b.WriteString("BEGIN;\n\n")

	for _, def := range definitions {
		fmt.Fprintf(
			&b,
			"INSERT INTO metric_definitions (service_name, namespace, metric_name, resource_id, region, dimensions, statistic, period_seconds, unit, enabled)\n"+
				"VALUES (%s, %s, %s, %s, %s, %s::jsonb, %s, %d, %s, %s)\n"+
				"ON CONFLICT ON CONSTRAINT metric_definitions_unique_metric DO UPDATE SET\n"+
				"    service_name = EXCLUDED.service_name,\n"+
				"    dimensions = EXCLUDED.dimensions,\n"+
				"    unit = EXCLUDED.unit,\n"+
				"    enabled = EXCLUDED.enabled,\n"+
				"    updated_at = now();\n\n",
			sqlQuote(def.ServiceName),
			sqlQuote(def.Namespace),
			sqlQuote(def.MetricName),
			sqlQuote(def.ResourceID),
			sqlQuote(def.Region),
			sqlQuote(dimensionsJSON(def.Dimensions)),
			sqlQuote(def.Statistic),
			def.PeriodSeconds,
			sqlQuote(def.Unit),
			sqlBool(def.Enabled),
		)
	}

	b.WriteString("COMMIT;\n")
	return b.String()
}

func dimensionsJSON(dimensions []catalog.ResolvedDimension) string {
	data, err := json.Marshal(dimensions)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func sqlBool(value bool) string {
	if value {
		return "TRUE"
	}
	return "FALSE"
}
