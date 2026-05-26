package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"cloud-monitor/internal/cloudwatchmetrics"
	"cloud-monitor/internal/collector"
	"cloud-monitor/internal/config"
	"cloud-monitor/internal/sanitize"
	"cloud-monitor/internal/store"
)

func main() {
	once := flag.Bool("once", false, "run one collection cycle and exit")
	flag.Parse()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "collector configuration error: %v\n", err)
		os.Exit(2)
	}
	if cfg.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "collector configuration error: DATABASE_URL is required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collector database error: %s\n", sanitize.Message(err.Error(), cfg.DatabaseURL))
		os.Exit(1)
	}
	defer db.Close()

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		fmt.Fprintf(os.Stderr, "collector AWS configuration error: %s\n", sanitize.Message(err.Error(), cfg.DatabaseURL))
		os.Exit(1)
	}

	metricFetcher := cloudwatchmetrics.NewFetcher(cloudwatch.NewFromConfig(awsCfg))
	runner := collector.New(cfg, db, metricFetcher, os.Stderr)

	fmt.Printf("cloud-monitor collector ready: region=%s interval=%s lookback=%s retention_days=%d\n", cfg.AWSRegion, cfg.CollectorInterval, cfg.CloudWatchLookback, cfg.MetricRetentionDays)

	if *once {
		result, err := runner.CollectOnce(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "collector run error: %s\n", sanitize.Message(err.Error(), cfg.DatabaseURL))
			os.Exit(1)
		}
		printResult(result)
		return
	}

	ticker := time.NewTicker(cfg.CollectorInterval)
	defer ticker.Stop()

	for {
		result, err := runner.CollectOnce(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "collector run error: %s\n", sanitize.Message(err.Error(), cfg.DatabaseURL))
		} else {
			printResult(result)
		}

		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "collector stopped")
			return
		case <-ticker.C:
		}
	}
}

func printResult(result collector.RunResult) {
	fmt.Printf(
		"collector cycle completed: definitions=%d fetched=%d inserted=%d window_start=%s window_end=%s\n",
		result.Definitions,
		result.Fetched,
		result.Inserted,
		result.StartTime.Format(time.RFC3339),
		result.EndTime.Format(time.RFC3339),
	)
}
