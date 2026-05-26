package cloudwatchmetrics

import (
	"context"
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
