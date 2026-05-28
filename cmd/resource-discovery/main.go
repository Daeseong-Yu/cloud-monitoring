package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"

	"cloud-monitor/internal/discovery"
	"cloud-monitor/internal/productcatalog"
	"cloud-monitor/internal/sanitize"
	"cloud-monitor/internal/store"
)

func main() {
	defaultRegistry := discovery.DefaultRegistry()
	namespaces := flag.String("namespaces", strings.Join(defaultRegistry.Namespaces(), ","), "comma-separated CloudWatch namespaces to discover")
	catalogPath := flag.String("catalog", "configs/product-metric-catalog.json", "product metric catalog JSON path")
	dryRun := flag.Bool("dry-run", false, "print discovery candidates without writing to PostgreSQL")
	flag.Parse()

	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		fmt.Fprintln(os.Stderr, "AWS_REGION is required")
		os.Exit(2)
	}

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource discovery AWS configuration error: %v\n", err)
		os.Exit(1)
	}

	catalog, err := productcatalog.LoadFile(*catalogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource discovery product catalog error: %v\n", err)
		os.Exit(1)
	}

	cloudWatchMetrics, err := listMetrics(ctx, cloudwatch.NewFromConfig(awsCfg), splitList(*namespaces))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource discovery CloudWatch error: %s\n", sanitize.Message(err.Error(), ""))
		os.Exit(1)
	}

	tags, err := listResourceTags(ctx, resourcegroupstaggingapi.NewFromConfig(awsCfg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource discovery tag enrichment error: %s\n", sanitize.Message(err.Error(), ""))
		os.Exit(1)
	}

	resources := defaultRegistry.Discover(region, cloudWatchMetrics, tags, catalog)
	if *dryRun {
		printResources(resources)
		return
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}

	db, err := store.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource discovery database error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}
	defer db.Close()

	resourceCount, metricCount, err := db.UpsertDiscoveredResources(ctx, resources)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resource discovery upsert error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}

	fmt.Printf("resource discovery completed: resources=%d metrics=%d region=%s\n", resourceCount, metricCount, region)
}

type metricLister interface {
	ListMetrics(context.Context, *cloudwatch.ListMetricsInput, ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
}

func listMetrics(ctx context.Context, client metricLister, namespaces []string) ([]discovery.Metric, error) {
	var metrics []discovery.Metric
	for _, namespace := range namespaces {
		input := &cloudwatch.ListMetricsInput{Namespace: aws.String(namespace)}
		for {
			output, err := client.ListMetrics(ctx, input)
			if err != nil {
				return nil, err
			}

			for _, metric := range output.Metrics {
				metrics = append(metrics, convertMetric(metric))
			}

			if output.NextToken == nil || strings.TrimSpace(*output.NextToken) == "" {
				break
			}
			input.NextToken = output.NextToken
		}
	}
	return metrics, nil
}

func convertMetric(metric cloudwatchtypes.Metric) discovery.Metric {
	dimensions := make([]discovery.Dimension, 0, len(metric.Dimensions))
	for _, dimension := range metric.Dimensions {
		dimensions = append(dimensions, discovery.Dimension{
			Name:  aws.ToString(dimension.Name),
			Value: aws.ToString(dimension.Value),
		})
	}
	return discovery.Metric{
		Namespace:  aws.ToString(metric.Namespace),
		MetricName: aws.ToString(metric.MetricName),
		Dimensions: dimensions,
	}
}

type tagLister interface {
	GetResources(context.Context, *resourcegroupstaggingapi.GetResourcesInput, ...func(*resourcegroupstaggingapi.Options)) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}

func listResourceTags(ctx context.Context, client tagLister) (map[string]discovery.TagInfo, error) {
	tags := map[string]discovery.TagInfo{}
	input := &resourcegroupstaggingapi.GetResourcesInput{}

	for {
		output, err := client.GetResources(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, mapping := range output.ResourceTagMappingList {
			resourceID := resourceIDFromARN(aws.ToString(mapping.ResourceARN))
			if resourceID == "" {
				continue
			}

			tagMap := map[string]string{}
			displayName := ""
			for _, tag := range mapping.Tags {
				key := aws.ToString(tag.Key)
				value := aws.ToString(tag.Value)
				if key == "" {
					continue
				}
				tagMap[key] = value
				if key == "Name" || key == "name" {
					displayName = value
				}
			}
			tags[resourceID] = discovery.TagInfo{
				DisplayName: displayName,
				Tags:        tagMap,
			}
		}

		if output.PaginationToken == nil || strings.TrimSpace(*output.PaginationToken) == "" {
			break
		}
		input.PaginationToken = output.PaginationToken
	}

	return tags, nil
}

func resourceIDFromARN(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 {
		return ""
	}
	resource := parts[5]
	for _, separator := range []string{"/", ":"} {
		if strings.Contains(resource, separator) {
			segments := strings.Split(resource, separator)
			return segments[len(segments)-1]
		}
	}
	return resource
}

func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	sort.Strings(items)
	return items
}

func printResources(resources []discovery.Resource) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(resources); err != nil {
		fmt.Fprintf(os.Stderr, "encode discovery output: %v\n", err)
		os.Exit(1)
	}
}
