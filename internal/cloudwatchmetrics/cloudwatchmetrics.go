package cloudwatchmetrics

import (
	"context"
	"errors"
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
	var skippedDefinitions int
	var failures []error
	for start := 0; start < len(definitions); start += maxMetricDataQueries {
		end := start + maxMetricDataQueries
		if end > len(definitions) {
			end = len(definitions)
		}

		batchPoints, skipped, err := f.fetchBatchTolerant(ctx, definitions[start:end], startTime, endTime)
		skippedDefinitions += skipped
		if err != nil {
			failures = append(failures, err)
		}
		points = append(points, batchPoints...)
	}

	if len(failures) > 0 {
		return points, PartialFailure{
			Skipped:   skippedDefinitions,
			FailedIDs: failedDefinitionIDsFromErrors(failures),
			Cause:     errors.Join(failures...),
		}
	}
	return points, nil
}

type PartialFailure struct {
	Skipped   int
	FailedIDs []int64
	Cause     error
}

func (e PartialFailure) Error() string {
	return fmt.Sprintf("partial cloudwatch metric fetch failure: skipped_definitions=%d", e.Skipped)
}

func (e PartialFailure) Unwrap() error {
	return e.Cause
}

func (e PartialFailure) SkippedDefinitions() int {
	return e.Skipped
}

func (e PartialFailure) FailedDefinitionIDs() []int64 {
	return append([]int64(nil), e.FailedIDs...)
}

func (f Fetcher) fetchBatchTolerant(ctx context.Context, definitions []store.MetricDefinition, startTime time.Time, endTime time.Time) ([]store.MetricPoint, int, error) {
	points, err := f.fetchBatch(ctx, definitions, startTime, endTime)
	if err == nil {
		return points, 0, nil
	}
	if len(definitions) <= 1 {
		return nil, len(definitions), err
	}

	var allPoints []store.MetricPoint
	var skipped int
	var failures []error
	for _, definition := range definitions {
		points, err := f.fetchBatch(ctx, []store.MetricDefinition{definition}, startTime, endTime)
		if err != nil {
			skipped++
			failures = append(failures, definitionFetchError{definitionID: definition.ID, cause: err})
			continue
		}
		allPoints = append(allPoints, points...)
	}
	if len(failures) > 0 {
		return allPoints, skipped, errors.Join(failures...)
	}
	return allPoints, 0, nil
}

type definitionFetchError struct {
	definitionID int64
	cause        error
}

func (e definitionFetchError) Error() string {
	return e.cause.Error()
}

func (e definitionFetchError) Unwrap() error {
	return e.cause
}

func failedDefinitionIDsFromErrors(errs []error) []int64 {
	var ids []int64
	for _, err := range errs {
		var definitionErr definitionFetchError
		if errors.As(err, &definitionErr) {
			ids = append(ids, definitionErr.definitionID)
		}
	}
	return ids
}

func (f Fetcher) fetchBatch(ctx context.Context, definitions []store.MetricDefinition, startTime time.Time, endTime time.Time) ([]store.MetricPoint, error) {
	queryToDefinitionID := make(map[string]int64, len(definitions))
	queries := make([]types.MetricDataQuery, 0, len(definitions))

	for i, definition := range definitions {
		queryID := fmt.Sprintf("m%d", i)
		queryToDefinitionID[queryID] = definition.ID
		dimensions, err := cloudWatchDimensions(definition)
		if err != nil {
			return nil, err
		}

		queries = append(queries, types.MetricDataQuery{
			Id: aws.String(queryID),
			MetricStat: &types.MetricStat{
				Metric: &types.Metric{
					Namespace:  aws.String(definition.Namespace),
					MetricName: aws.String(definition.MetricName),
					Dimensions: dimensions,
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

func cloudWatchDimensions(definition store.MetricDefinition) ([]types.Dimension, error) {
	dimensions, err := definition.Dimensions()
	if err != nil {
		return nil, err
	}

	cloudWatchDimensions := make([]types.Dimension, 0, len(dimensions))
	for _, dimension := range dimensions {
		cloudWatchDimensions = append(cloudWatchDimensions, types.Dimension{
			Name:  aws.String(dimension.Name),
			Value: aws.String(dimension.Value),
		})
	}
	return cloudWatchDimensions, nil
}
