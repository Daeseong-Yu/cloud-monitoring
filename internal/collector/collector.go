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
	InsertMetricPointsDetailed(context.Context, []store.MetricPoint) (store.MetricInsertSummary, error)
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

type definitionPartialFetchError interface {
	partialFetchError
	FailedDefinitionIDs() []int64
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

	points, fetchErr := c.fetcher.Fetch(ctx, definitions, startTime, endTime)
	skipped := 0
	if fetchErr != nil {
		var partial partialFetchError
		if !errors.As(fetchErr, &partial) {
			c.recordFailures(ctx, definitions, fetchErr, endTime)
			return RunResult{}, fmt.Errorf("fetch metric data: %w", fetchErr)
		}
		skipped = partial.SkippedDefinitions()
		fmt.Fprintf(c.log, "collector partial metric fetch failure: skipped_definitions=%d\n", skipped)
	}

	insertSummary, err := c.store.InsertMetricPointsDetailed(ctx, points)
	if err != nil {
		c.recordFailures(ctx, definitions, err, endTime)
		return RunResult{}, fmt.Errorf("store metric points: %w", err)
	}
	c.recordSuccesses(ctx, definitions, points, insertSummary.InsertedByDefinition, endTime)
	if fetchErr != nil {
		c.recordPartialFailures(ctx, definitions, fetchErr, endTime)
	}

	return RunResult{
		Definitions:        len(definitions),
		SkippedDefinitions: skipped,
		Fetched:            len(points),
		Inserted:           insertSummary.Inserted,
		StartTime:          startTime,
		EndTime:            endTime,
	}, nil
}

func (c Collector) recordSuccesses(ctx context.Context, definitions []store.MetricDefinition, points []store.MetricPoint, insertedByDefinition map[int64]int64, collectedAt time.Time) {
	statuses := map[int64]store.MetricCollectionStatusInput{}
	for _, definition := range definitions {
		statuses[definition.ID] = store.MetricCollectionStatusInput{
			MetricDefinitionID: definition.ID,
			CollectedAt:        collectedAt,
		}
	}
	for _, point := range points {
		status := statuses[point.MetricDefinitionID]
		status.MetricDefinitionID = point.MetricDefinitionID
		status.RecentPointCount++
		status.FetchedPointCount++
		status.CollectedAt = collectedAt
		if point.Timestamp.After(status.LatestPointAt) {
			status.LatestPointAt = point.Timestamp
		}
		statuses[point.MetricDefinitionID] = status
	}
	for definitionID, inserted := range insertedByDefinition {
		status := statuses[definitionID]
		status.MetricDefinitionID = definitionID
		status.InsertedPointCount = inserted
		status.CollectedAt = collectedAt
		statuses[definitionID] = status
	}

	for _, status := range statuses {
		if err := c.store.RecordMetricCollectionSuccess(ctx, status); err != nil {
			fmt.Fprintf(c.log, "collector status update failure: %s\n", sanitize.Message(err.Error(), ""))
		}
	}
}

func (c Collector) recordPartialFailures(ctx context.Context, definitions []store.MetricDefinition, err error, collectedAt time.Time) {
	var partial definitionPartialFetchError
	if !errors.As(err, &partial) {
		return
	}
	failed := map[int64]struct{}{}
	for _, id := range partial.FailedDefinitionIDs() {
		failed[id] = struct{}{}
	}
	if len(failed) == 0 {
		return
	}

	message := sanitize.Message(err.Error(), "")
	for _, definition := range definitions {
		if _, ok := failed[definition.ID]; !ok {
			continue
		}
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
