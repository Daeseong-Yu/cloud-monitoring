package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"cloud-monitor/internal/config"
	"cloud-monitor/internal/sanitize"
	"cloud-monitor/internal/store"
)

type DefinitionStore interface {
	EnabledMetricDefinitions(context.Context, string) ([]store.MetricDefinition, error)
	InsertMetricPoints(context.Context, []store.MetricPoint) (int64, error)
	RecordMetricCollectionSuccess(context.Context, store.MetricCollectionStatusInput) error
	RecordMetricCollectionFailure(context.Context, store.MetricCollectionStatusInput) error
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
	Definitions        int
	SkippedDefinitions int
	Fetched            int
	Inserted           int64
	StartTime          time.Time
	EndTime            time.Time
}

type partialFetchError interface {
	error
	SkippedDefinitions() int
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
	skipped := 0
	if err != nil {
		var partial partialFetchError
		if !errors.As(err, &partial) {
			c.recordFailures(ctx, definitions, err, endTime)
			return RunResult{}, fmt.Errorf("fetch metric data: %w", err)
		}
		skipped = partial.SkippedDefinitions()
		fmt.Fprintf(c.log, "collector partial metric fetch failure: skipped_definitions=%d\n", skipped)
	}

	inserted, err := c.store.InsertMetricPoints(ctx, points)
	if err != nil {
		c.recordFailures(ctx, definitions, err, endTime)
		return RunResult{}, fmt.Errorf("store metric points: %w", err)
	}
	c.recordSuccesses(ctx, points, endTime)

	return RunResult{
		Definitions:        len(definitions),
		SkippedDefinitions: skipped,
		Fetched:            len(points),
		Inserted:           inserted,
		StartTime:          startTime,
		EndTime:            endTime,
	}, nil
}

func (c Collector) recordSuccesses(ctx context.Context, points []store.MetricPoint, collectedAt time.Time) {
	statuses := map[int64]store.MetricCollectionStatusInput{}
	for _, point := range points {
		status := statuses[point.MetricDefinitionID]
		status.MetricDefinitionID = point.MetricDefinitionID
		status.RecentPointCount++
		status.CollectedAt = collectedAt
		if point.Timestamp.After(status.LatestPointAt) {
			status.LatestPointAt = point.Timestamp
		}
		statuses[point.MetricDefinitionID] = status
	}

	for _, status := range statuses {
		if err := c.store.RecordMetricCollectionSuccess(ctx, status); err != nil {
			fmt.Fprintf(c.log, "collector status update failure: %s\n", sanitize.Message(err.Error(), ""))
		}
	}
}

func (c Collector) recordFailures(ctx context.Context, definitions []store.MetricDefinition, err error, collectedAt time.Time) {
	message := sanitize.Message(err.Error(), "")
	for _, definition := range definitions {
		input := store.MetricCollectionStatusInput{
			MetricDefinitionID: definition.ID,
			SanitizedError:     message,
			CollectedAt:        collectedAt,
		}
		if statusErr := c.store.RecordMetricCollectionFailure(ctx, input); statusErr != nil {
			fmt.Fprintf(c.log, "collector status update failure: %s\n", sanitize.Message(statusErr.Error(), ""))
		}
	}
}
