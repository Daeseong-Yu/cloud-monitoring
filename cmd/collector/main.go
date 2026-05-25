package main

import (
	"fmt"
	"os"

	"cloud-monitor/internal/config"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "collector configuration error: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf(
		"cloud-monitor collector ready: region=%s interval=%s lookback=%s retention_days=%d\n",
		cfg.AWSRegion,
		cfg.CollectorInterval,
		cfg.CloudWatchLookback,
		cfg.MetricRetentionDays,
	)
}
