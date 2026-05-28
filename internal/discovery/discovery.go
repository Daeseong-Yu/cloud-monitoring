package discovery

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Dimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Metric struct {
	Namespace  string
	MetricName string
	Dimensions []Dimension
}

type TagInfo struct {
	DisplayName string
	Tags        map[string]string
}

type Resource struct {
	ServiceName    string
	Namespace      string
	ResourceID     string
	Region         string
	DisplayName    string
	Tags           map[string]string
	ProviderSource string
	Metrics        []DiscoveredMetric
}

type DiscoveredMetric struct {
	Namespace          string
	MetricName         string
	Dimensions         []Dimension
	Statistic          string
	PeriodSeconds      int
	Unit               string
	AvailabilityStatus string
	AvailabilityReason string
	ProviderSource     string
	Prerequisite       string
	CostWarning        string
}

func ResourcesFromMetrics(region string, metrics []Metric, tags map[string]TagInfo) []Resource {
	resourcesByKey := map[string]*Resource{}

	for _, metric := range metrics {
		namespace := strings.TrimSpace(metric.Namespace)
		metricName := strings.TrimSpace(metric.MetricName)
		dimensions := NormalizeDimensions(metric.Dimensions)
		resourceID := ResourceIDFromDimensions(dimensions)
		if namespace == "" || metricName == "" || resourceID == "" {
			continue
		}

		serviceName := ServiceNameFromNamespace(namespace)
		key := strings.Join([]string{serviceName, resourceID, region}, "\x00")
		resource, ok := resourcesByKey[key]
		if !ok {
			tagInfo := tags[resourceID]
			displayName := strings.TrimSpace(tagInfo.DisplayName)
			if displayName == "" {
				displayName = resourceID
			}
			resource = &Resource{
				ServiceName: serviceName,
				Namespace:   namespace,
				ResourceID:  resourceID,
				Region:      region,
				DisplayName: displayName,
				Tags:        tagInfo.Tags,
			}
			resourcesByKey[key] = resource
		}

		resource.Metrics = append(resource.Metrics, DiscoveredMetric{
			Namespace:     namespace,
			MetricName:    metricName,
			Dimensions:    dimensions,
			Statistic:     DefaultStatistic(metricName),
			PeriodSeconds: 300,
		})
	}

	resources := make([]Resource, 0, len(resourcesByKey))
	for _, resource := range resourcesByKey {
		sort.Slice(resource.Metrics, func(i, j int) bool {
			return metricKey(resource.Metrics[i]) < metricKey(resource.Metrics[j])
		})
		resource.Metrics = dedupeMetrics(resource.Metrics)
		resources = append(resources, *resource)
	}

	sort.Slice(resources, func(i, j int) bool {
		return strings.Join([]string{resources[i].Region, resources[i].ServiceName, resources[i].ResourceID}, "\x00") <
			strings.Join([]string{resources[j].Region, resources[j].ServiceName, resources[j].ResourceID}, "\x00")
	})

	return resources
}

func NormalizeDimensions(dimensions []Dimension) []Dimension {
	normalized := make([]Dimension, 0, len(dimensions))
	for _, dimension := range dimensions {
		name := strings.TrimSpace(dimension.Name)
		value := strings.TrimSpace(dimension.Value)
		if name == "" || value == "" {
			continue
		}
		normalized = append(normalized, Dimension{Name: name, Value: value})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Name == normalized[j].Name {
			return normalized[i].Value < normalized[j].Value
		}
		return normalized[i].Name < normalized[j].Name
	})
	return normalized
}

func ResourceIDFromDimensions(dimensions []Dimension) string {
	preferred := []string{
		"InstanceId",
		"FunctionName",
		"DBInstanceIdentifier",
		"TableName",
		"BucketName",
		"LoadBalancer",
		"ApiName",
		"App",
		"QueueName",
	}
	for _, name := range preferred {
		for _, dimension := range dimensions {
			if dimension.Name == name {
				return dimension.Value
			}
		}
	}
	if len(dimensions) == 1 {
		return dimensions[0].Value
	}
	return ""
}

func ServiceNameFromNamespace(namespace string) string {
	value := strings.TrimSpace(namespace)
	value = strings.TrimPrefix(value, "AWS/")
	value = strings.TrimPrefix(value, "aws/")
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "/", "-")
	return value
}

func DefaultStatistic(metricName string) string {
	switch metricName {
	case "Errors", "Invocations", "NetworkIn", "NetworkOut", "Throttles":
		return "Sum"
	case "StatusCheckFailed":
		return "Maximum"
	default:
		return "Average"
	}
}

func DimensionsJSON(dimensions []Dimension) (string, error) {
	normalized := NormalizeDimensions(dimensions)
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal dimensions: %w", err)
	}
	return string(data), nil
}

func TagsJSON(tags map[string]string) (string, error) {
	if tags == nil {
		tags = map[string]string{}
	}
	data, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}
	return string(data), nil
}

func dedupeMetrics(metrics []DiscoveredMetric) []DiscoveredMetric {
	var deduped []DiscoveredMetric
	seen := map[string]struct{}{}
	for _, metric := range metrics {
		key := metricKey(metric)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, metric)
	}
	return deduped
}

func metricKey(metric DiscoveredMetric) string {
	dimensionsJSON, _ := DimensionsJSON(metric.Dimensions)
	return strings.Join([]string{
		metric.Namespace,
		metric.MetricName,
		dimensionsJSON,
		metric.Statistic,
		fmt.Sprintf("%d", metric.PeriodSeconds),
	}, "\x00")
}
