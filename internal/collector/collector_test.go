package collector

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"cloud-monitor/internal/config"
	"cloud-monitor/internal/store"
)

type fakeStore struct {
	definitions []store.MetricDefinition
	points      []store.MetricPoint
	successes   []store.MetricCollectionStatusInput
	failures    []store.MetricCollectionStatusInput
	inserted    int64
}

func (s *fakeStore) EnabledMetricDefinitions(ctx context.Context, region string) ([]store.MetricDefinition, error) {
	return s.definitions, nil
}

func (s *fakeStore) InsertMetricPointsDetailed(ctx context.Context, points []store.MetricPoint) (store.MetricInsertSummary, error) {
	s.points = append(s.points, points...)
	insertedByDefinition := map[int64]int64{}
	for _, point := range points {
		insertedByDefinition[point.MetricDefinitionID]++
	}
	return store.MetricInsertSummary{Inserted: s.inserted, InsertedByDefinition: insertedByDefinition}, nil
}

func (s *fakeStore) RecordMetricCollectionSuccess(ctx context.Context, input store.MetricCollectionStatusInput) error {
	s.successes = append(s.successes, input)
	return nil
}

func (s *fakeStore) RecordMetricCollectionFailure(ctx context.Context, input store.MetricCollectionStatusInput) error {
	s.failures = append(s.failures, input)
	return nil
}

type fakeFetcher struct {
	start  time.Time
	end    time.Time
	points []store.MetricPoint
	err    error
}

func (f *fakeFetcher) Fetch(ctx context.Context, definitions []store.MetricDefinition, startTime time.Time, endTime time.Time) ([]store.MetricPoint, error) {
	f.start = startTime
	f.end = endTime
	return f.points, f.err
}

type fakePartialFetchError struct {
	skipped   int
	failedIDs []int64
}

func (e fakePartialFetchError) Error() string {
	return "partial failure"
}

func (e fakePartialFetchError) SkippedDefinitions() int {
	return e.skipped
}

func (e fakePartialFetchError) FailedDefinitionIDs() []int64 {
	return e.failedIDs
}

func TestCollectOnceUsesLookbackAndStoresPoints(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	db := &fakeStore{
		definitions: []store.MetricDefinition{{ID: 1}},
		inserted:    1,
	}
	fetcher := &fakeFetcher{
		points: []store.MetricPoint{{MetricDefinitionID: 1, Timestamp: now, Value: 10}},
	}

	c := New(config.Config{
		AWSRegion:           "us-east-1",
		CloudWatchLookback:  15 * time.Minute,
		CollectorInterval:   time.Minute,
		MetricRetentionDays: 30,
	}, db, fetcher, io.Discard)
	c.clock = func() time.Time { return now }

	result, err := c.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("collect once: %v", err)
	}

	if got, want := fetcher.start, now.Add(-15*time.Minute); !got.Equal(want) {
		t.Fatalf("start time = %s, want %s", got, want)
	}
	if result.Definitions != 1 || result.Fetched != 1 || result.Inserted != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(db.points) != 1 {
		t.Fatalf("stored point count = %d, want 1", len(db.points))
	}
	if len(db.successes) != 1 || db.successes[0].FetchedPointCount != 1 || db.successes[0].InsertedPointCount != 1 {
		t.Fatalf("unexpected collection successes: %#v", db.successes)
	}
}

func TestCollectOnceStoresPointsAfterPartialFetchFailure(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	db := &fakeStore{
		definitions: []store.MetricDefinition{{ID: 1}, {ID: 2}},
		inserted:    1,
	}
	fetcher := &fakeFetcher{
		points: []store.MetricPoint{{MetricDefinitionID: 1, Timestamp: now, Value: 10}},
		err:    fakePartialFetchError{skipped: 1, failedIDs: []int64{2}},
	}

	c := New(config.Config{
		AWSRegion:           "us-east-1",
		CloudWatchLookback:  15 * time.Minute,
		CollectorInterval:   time.Minute,
		MetricRetentionDays: 30,
	}, db, fetcher, io.Discard)
	c.clock = func() time.Time { return now }

	result, err := c.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("collect once: %v", err)
	}
	if result.SkippedDefinitions != 1 {
		t.Fatalf("skipped definitions = %d, want 1", result.SkippedDefinitions)
	}
	if len(db.points) != 1 {
		t.Fatalf("stored point count = %d, want 1", len(db.points))
	}
	if len(db.successes) != 2 {
		t.Fatalf("collection successes = %d, want 2", len(db.successes))
	}
	if len(db.failures) != 1 || db.failures[0].MetricDefinitionID != 2 {
		t.Fatalf("collection failures = %#v, want failed definition 2", db.failures)
	}
}

func TestCollectOnceReturnsNonPartialFetchFailure(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	db := &fakeStore{definitions: []store.MetricDefinition{{ID: 1}}}
	fetcher := &fakeFetcher{err: errors.New("cloudwatch unavailable")}

	c := New(config.Config{
		AWSRegion:           "us-east-1",
		CloudWatchLookback:  15 * time.Minute,
		CollectorInterval:   time.Minute,
		MetricRetentionDays: 30,
	}, db, fetcher, io.Discard)
	c.clock = func() time.Time { return now }

	if _, err := c.CollectOnce(context.Background()); err == nil {
		t.Fatal("expected fetch failure")
	}
	if len(db.failures) != 1 {
		t.Fatalf("collection failures = %d, want 1", len(db.failures))
	}
}
