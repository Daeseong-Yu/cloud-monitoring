package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCollectorIntervalSeconds = 60
	defaultCloudWatchLookbackMin    = 15
	defaultMetricRetentionDays      = 30
)

type Config struct {
	AWSRegion           string
	DatabaseURL         string
	CollectorInterval   time.Duration
	CloudWatchLookback  time.Duration
	MetricRetentionDays int
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		AWSRegion:           strings.TrimSpace(os.Getenv("AWS_REGION")),
		DatabaseURL:         strings.TrimSpace(os.Getenv("DATABASE_URL")),
		CollectorInterval:   time.Duration(getenvInt("COLLECTOR_INTERVAL_SECONDS", defaultCollectorIntervalSeconds)) * time.Second,
		CloudWatchLookback:  time.Duration(getenvInt("CLOUDWATCH_LOOKBACK_MINUTES", defaultCloudWatchLookbackMin)) * time.Minute,
		MetricRetentionDays: getenvInt("METRIC_RETENTION_DAYS", defaultMetricRetentionDays),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	if cfg.AWSRegion == "" {
		return fmt.Errorf("AWS_REGION is required")
	}
	if cfg.CollectorInterval < time.Minute {
		return fmt.Errorf("COLLECTOR_INTERVAL_SECONDS must be at least 60")
	}
	if cfg.CloudWatchLookback < 10*time.Minute || cfg.CloudWatchLookback > 15*time.Minute {
		return fmt.Errorf("CLOUDWATCH_LOOKBACK_MINUTES must be between 10 and 15")
	}
	if cfg.MetricRetentionDays != 30 {
		return fmt.Errorf("METRIC_RETENTION_DAYS must remain 30 for MVP")
	}
	return nil
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
