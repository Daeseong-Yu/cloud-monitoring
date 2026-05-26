package collector

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud-monitor/internal/config"
	"cloud-monitor/internal/store"
)

type DefinitionStore interface {
	EnabledMetricDefinitions(context.Context, string) ([]store.MetricDefinition, error)
	InsertMetricPoints(context.Context, []store.MetricPoint) (int64, error)
}

type MetricFetcher interface {
	Fetch(context.Context, []store.MetricDefinition, time.Time, time.Time) ([]store.MetricPoint, error)
}

type Collector struct {
	cfg     config.Config
	store   DefinitionStore
	fetcher MetricFetcher
	clock   func() time.Time
	log     io.Writer
}

type RunResult struct {
	Definitions int
	Fetched     int
	Inserted    int64
	StartTime   time.Time
	EndTime     time.Time
}

func New(cfg config.Config, store DefinitionStore, fetcher MetricFetcher, log io.Writer) Collector {
	return Collector{
		cfg:     cfg,
		store:   store,
		fetcher: fetcher,
		clock:   time.Now,
		log:     log,
	}
}

func (c Collector) CollectOnce(ctx context.Context) (RunResult, error) {
	endTime := c.clock().UTC()
	startTime := endTime.Add(-c.cfg.CloudWatchLookback)

	definitions, err := c.store.EnabledMetricDefinitions(ctx, c.cfg.AWSRegion)
	if err != nil {
		return RunResult{}, fmt.Errorf("load enabled metric definitions: %w", err)
	}
	if len(definitions) == 0 {
		return RunResult{StartTime: startTime, EndTime: endTime}, nil
	}

	points, err := c.fetcher.Fetch(ctx, definitions, startTime, endTime)
	if err != nil {
		return RunResult{}, fmt.Errorf("fetch metric data: %w", err)
	}

	inserted, err := c.store.InsertMetricPoints(ctx, points)
	if err != nil {
		return RunResult{}, fmt.Errorf("store metric points: %w", err)
	}

	return RunResult{
		Definitions: len(definitions),
		Fetched:     len(points),
		Inserted:    inserted,
		StartTime:   startTime,
		EndTime:     endTime,
	}, nil
}

func (c Collector) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.CollectorInterval)
	defer ticker.Stop()

	for {
		if _, err := c.CollectOnce(ctx); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
