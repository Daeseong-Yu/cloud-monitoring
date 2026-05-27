package cloudwatchmetrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"cloud-monitor/internal/store"
)

type fakeMetricDataClient struct {
	inputs []*cloudwatch.GetMetricDataInput
	output *cloudwatch.GetMetricDataOutput
	err    error
}

func (c *fakeMetricDataClient) GetMetricData(ctx context.Context, input *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	c.inputs = append(c.inputs, input)
	return c.output, c.err
}

type sequenceMetricDataClient struct {
	inputs  []*cloudwatch.GetMetricDataInput
	results []metricDataCall
}

type metricDataCall struct {
	output *cloudwatch.GetMetricDataOutput
	err    error
}

func (c *sequenceMetricDataClient) GetMetricData(ctx context.Context, input *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	c.inputs = append(c.inputs, input)
	call := c.results[0]
	c.results = c.results[1:]
	return call.output, call.err
}

func TestFetchBuildsBatchQueryAndConvertsPoints(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	client := &fakeMetricDataClient{
		output: &cloudwatch.GetMetricDataOutput{
			MetricDataResults: []types.MetricDataResult{
				{
					Id:         aws.String("m0"),
					Timestamps: []time.Time{now.Add(-time.Minute)},
					Values:     []float64{25.5},
				},
			},
		},
	}

	fetcher := NewFetcher(client)
	points, err := fetcher.Fetch(context.Background(), []store.MetricDefinition{
		{
			ID:            10,
			Namespace:     "AWS/EC2",
			MetricName:    "CPUUtilization",
			ResourceID:    "REPLACE_WITH_INSTANCE_ID",
			Statistic:     "Average",
			PeriodSeconds: 300,
		},
	}, now.Add(-15*time.Minute), now)
	if err != nil {
		t.Fatalf("fetch metric data: %v", err)
	}

	if got, want := len(client.inputs), 1; got != want {
		t.Fatalf("GetMetricData calls = %d, want %d", got, want)
	}
	if got, want := len(client.inputs[0].MetricDataQueries), 1; got != want {
		t.Fatalf("query count = %d, want %d", got, want)
	}
	if got, want := *client.inputs[0].MetricDataQueries[0].MetricStat.Metric.Dimensions[0].Name, "InstanceId"; got != want {
		t.Fatalf("dimension name = %q, want %q", got, want)
	}
	if got, want := len(points), 1; got != want {
		t.Fatalf("point count = %d, want %d", got, want)
	}
	if points[0].MetricDefinitionID != 10 {
		t.Fatalf("metric definition id = %d, want 10", points[0].MetricDefinitionID)
	}
	if points[0].Value != 25.5 {
		t.Fatalf("value = %f, want 25.5", points[0].Value)
	}
}

func TestFetchUsesStoredGenericDimensions(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	client := &fakeMetricDataClient{
		output: &cloudwatch.GetMetricDataOutput{},
	}

	fetcher := NewFetcher(client)
	_, err := fetcher.Fetch(context.Background(), []store.MetricDefinition{
		{
			ID:             11,
			Namespace:      "AWS/Lambda",
			MetricName:     "Errors",
			ResourceID:     "orders-api",
			DimensionsJSON: `[{"name":"FunctionName","value":"orders-api"}]`,
			Statistic:      "Sum",
			PeriodSeconds:  300,
		},
	}, now.Add(-15*time.Minute), now)
	if err != nil {
		t.Fatalf("fetch metric data: %v", err)
	}

	dimensions := client.inputs[0].MetricDataQueries[0].MetricStat.Metric.Dimensions
	if got, want := len(dimensions), 1; got != want {
		t.Fatalf("dimension count = %d, want %d", got, want)
	}
	if got, want := *dimensions[0].Name, "FunctionName"; got != want {
		t.Fatalf("dimension name = %q, want %q", got, want)
	}
	if got, want := *dimensions[0].Value, "orders-api"; got != want {
		t.Fatalf("dimension value = %q, want %q", got, want)
	}
}

func TestFetchRetriesBatchAsSingleDefinitionsAndReturnsPartialFailure(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	client := &sequenceMetricDataClient{
		results: []metricDataCall{
			{err: errors.New("batch failed")},
			{
				output: &cloudwatch.GetMetricDataOutput{
					MetricDataResults: []types.MetricDataResult{
						{
							Id:         aws.String("m0"),
							Timestamps: []time.Time{now.Add(-time.Minute)},
							Values:     []float64{1},
						},
					},
				},
			},
			{err: errors.New("single failed")},
		},
	}

	fetcher := NewFetcher(client)
	points, err := fetcher.Fetch(context.Background(), []store.MetricDefinition{
		{
			ID:            1,
			Namespace:     "AWS/EC2",
			MetricName:    "CPUUtilization",
			ResourceID:    "REPLACE_WITH_INSTANCE_ID",
			Statistic:     "Average",
			PeriodSeconds: 300,
		},
		{
			ID:             2,
			Namespace:      "AWS/Lambda",
			MetricName:     "Errors",
			ResourceID:     "orders-api",
			DimensionsJSON: `[{"name":"FunctionName","value":"orders-api"}]`,
			Statistic:      "Sum",
			PeriodSeconds:  300,
		},
	}, now.Add(-15*time.Minute), now)
	if err == nil {
		t.Fatal("expected partial failure")
	}
	var partial PartialFailure
	if !errors.As(err, &partial) {
		t.Fatalf("error type = %T, want PartialFailure", err)
	}
	if partial.SkippedDefinitions() != 1 {
		t.Fatalf("skipped definitions = %d, want 1", partial.SkippedDefinitions())
	}
	if got, want := len(points), 1; got != want {
		t.Fatalf("point count = %d, want %d", got, want)
	}
	if got, want := len(client.inputs), 3; got != want {
		t.Fatalf("GetMetricData calls = %d, want %d", got, want)
	}
}
