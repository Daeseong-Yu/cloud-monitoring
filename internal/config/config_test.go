package config

import (
	"testing"
	"time"
)

func TestValidateAcceptsMVPDefaults(t *testing.T) {
	cfg := Config{
		AWSRegion:           "us-east-1",
		CollectorInterval:   time.Minute,
		CloudWatchLookback:  15 * time.Minute,
		GrafanaRefresh:      10 * time.Minute,
		MetricRetentionDays: 30,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestValidateRejectsCollectorIntervalBelowOneMinute(t *testing.T) {
	cfg := Config{
		AWSRegion:           "us-east-1",
		CollectorInterval:   30 * time.Second,
		CloudWatchLookback:  15 * time.Minute,
		GrafanaRefresh:      10 * time.Minute,
		MetricRetentionDays: 30,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected collector interval validation error")
	}
}

func TestValidateRejectsGrafanaRefreshBelowTenMinutes(t *testing.T) {
	cfg := Config{
		AWSRegion:           "us-east-1",
		CollectorInterval:   time.Minute,
		CloudWatchLookback:  15 * time.Minute,
		GrafanaRefresh:      5 * time.Minute,
		MetricRetentionDays: 30,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected Grafana refresh validation error")
	}
}

func TestLoadFromEnvRejectsMissingAWSRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected AWS region validation error")
	}
}

func TestLoadFromEnvRejectsBlankAWSRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "   ")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected AWS region validation error")
	}
}
