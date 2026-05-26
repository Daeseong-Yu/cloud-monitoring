package cloudwatchmetrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"cloud-monitor/internal/store"
)

const maxMetricDataQueries = 500

type MetricDataClient interface {
	GetMetricData(context.Context, *cloudwatch.GetMetricDataInput, ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

type Fetcher struct {
	client MetricDataClient
}

func NewFetcher(client MetricDataClient) Fetcher {
	return Fetcher{client: client}
}

func (f Fetcher) Fetch(ctx context.Context, definitions []store.MetricDefinition, startTime time.Time, endTime time.Time) ([]store.MetricPoint, error) {
	if len(definitions) == 0 {
		return nil, nil
	}

	var points []store.MetricPoint
	for start := 0; start < len(definitions); start += maxMetricDataQueries {
		end := start + maxMetricDataQueries
		if end > len(definitions) {
			end = len(definitions)
		}

		batchPoints, err := f.fetchBatch(ctx, definitions[start:end], startTime, endTime)
		if err != nil {
			return nil, err
		}
		points = append(points, batchPoints...)
	}

	return points, nil
}

func (f Fetcher) fetchBatch(ctx context.Context, definitions []store.MetricDefinition, startTime time.Time, endTime time.Time) ([]store.MetricPoint, error) {
	queryToDefinitionID := make(map[string]int64, len(definitions))
	queries := make([]types.MetricDataQuery, 0, len(definitions))

	for i, definition := range definitions {
		queryID := fmt.Sprintf("m%d", i)
		queryToDefinitionID[queryID] = definition.ID

		queries = append(queries, types.MetricDataQuery{
			Id: aws.String(queryID),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Namespace:  aws.String(definition.Namespace),
					MetricName: aws.String(definition.MetricName),
					Dimensions: []types.Dimension{
						{
							Name:  aws.String("InstanceId"),
							Value: aws.String(definition.ResourceID),
						},
					},
				},
				Period: aws.Int32(definition.PeriodSeconds),
				Stat:   aws.String(definition.Statistic),
			},
			ReturnData: aws.Bool(true),
		})
	}

	input := &cloudwatch.GetMetricDataInput{
		StartTime:         aws.Time(startTime.UTC()),
		EndTime:           aws.Time(endTime.UTC()),
		MetricDataQueries: queries,
		ScanBy:            types.ScanByTimestampAscending,
	}

	var points []store.MetricPoint
	for {
		output, err := f.client.GetMetricData(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("get cloudwatch metric data: %w", err)
		}

		for _, result := range output.MetricDataResults {
			if result.Id == nil {
				continue
			}
			definitionID, ok := queryToDefinitionID[*result.Id]
			if !ok {
				continue
			}

			limit := len(result.Timestamps)
			if len(result.Values) < limit {
				limit = len(result.Values)
			}
			for i := 0; i < limit; i++ {
				points = append(points, store.MetricPoint{
					MetricDefinitionID: definitionID,
					Timestamp:          result.Timestamps[i].UTC(),
					Value:              result.Values[i],
				})
			}
		}

		if output.NextToken == nil || strings.TrimSpace(*output.NextToken) == "" {
			break
		}
		input.NextToken = output.NextToken
	}

	return points, nil
}
