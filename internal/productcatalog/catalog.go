package productcatalog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Catalog struct {
	Version int     `json:"version"`
	Metrics []Entry `json:"metrics"`
}

type Entry struct {
	Key                string   `json:"key"`
	ServiceName        string   `json:"serviceName"`
	Namespace          string   `json:"namespace"`
	MetricName         string   `json:"metricName"`
	Statistic          string   `json:"statistic"`
	PeriodSeconds      int      `json:"periodSeconds"`
	Unit               string   `json:"unit"`
	RequiredDimensions []string `json:"requiredDimensions"`
	Recommended        bool     `json:"recommended"`
	Axis               Axis     `json:"axis"`
	Prerequisite       string   `json:"prerequisite"`
	CostWarning        string   `json:"costWarning"`
}

type Axis struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

type DefaultMetricSet struct {
	ServiceName string          `json:"serviceName"`
	Namespace   string          `json:"namespace"`
	Name        string          `json:"name"`
	Metrics     []DefaultMetric `json:"metrics"`
}

type DefaultMetric struct {
	MetricName    string `json:"metricName"`
	Statistic     string `json:"statistic"`
	PeriodSeconds int    `json:"periodSeconds"`
	Unit          string `json:"unit,omitempty"`
}

type RecommendedMetricSet = DefaultMetricSet
type RecommendedMetric = DefaultMetric

var validStatistics = map[string]struct{}{
	"Average":     {},
	"Sum":         {},
	"Minimum":     {},
	"Maximum":     {},
	"SampleCount": {},
}

var validPeriods = map[int]struct{}{
	60:    {},
	300:   {},
	900:   {},
	3600:  {},
	86400: {},
}

func Load(r io.Reader) (Catalog, error) {
	var c Catalog
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return Catalog{}, fmt.Errorf("decode product metric catalog: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Catalog{}, err
	}
	return c, nil
}

func LoadFile(path string) (Catalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("open product metric catalog: %w", err)
	}
	defer file.Close()

	return Load(file)
}

func (c Catalog) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("product metric catalog version must be 1")
	}
	if len(c.Metrics) == 0 {
		return fmt.Errorf("product metric catalog must contain at least one metric")
	}

	seenKeys := map[string]struct{}{}
	seenDefinitions := map[string]struct{}{}
	for _, metric := range c.Metrics {
		if err := validateEntry(metric); err != nil {
			return err
		}
		key := strings.TrimSpace(metric.Key)
		if _, exists := seenKeys[key]; exists {
			return fmt.Errorf("duplicate product metric key %q", key)
		}
		seenKeys[key] = struct{}{}

		defKey := definitionKey(metric)
		if _, exists := seenDefinitions[defKey]; exists {
			return fmt.Errorf("duplicate product metric definition %s", defKey)
		}
		seenDefinitions[defKey] = struct{}{}
	}

	return nil
}

func (c Catalog) DefaultMetricSets() []DefaultMetricSet {
	type groupKey struct {
		serviceName string
		namespace   string
	}

	groups := map[groupKey][]DefaultMetric{}
	for _, metric := range c.Metrics {
		if !metric.Recommended {
			continue
		}
		key := groupKey{
			serviceName: strings.TrimSpace(metric.ServiceName),
			namespace:   strings.TrimSpace(metric.Namespace),
		}
		groups[key] = append(groups[key], DefaultMetric{
			MetricName:    strings.TrimSpace(metric.MetricName),
			Statistic:     strings.TrimSpace(metric.Statistic),
			PeriodSeconds: metric.PeriodSeconds,
			Unit:          strings.TrimSpace(metric.Unit),
		})
	}

	keys := make([]groupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].serviceName == keys[j].serviceName {
			return keys[i].namespace < keys[j].namespace
		}
		return keys[i].serviceName < keys[j].serviceName
	})

	sets := make([]DefaultMetricSet, 0, len(keys))
	for _, key := range keys {
		metrics := groups[key]
		sort.Slice(metrics, func(i, j int) bool {
			return defaultMetricKey(metrics[i]) < defaultMetricKey(metrics[j])
		})
		sets = append(sets, DefaultMetricSet{
			ServiceName: key.serviceName,
			Namespace:   key.namespace,
			Name:        key.serviceName + "-default",
			Metrics:     metrics,
		})
	}

	return sets
}

func (c Catalog) RecommendedMetricSets() []RecommendedMetricSet {
	return c.DefaultMetricSets()
}

func validateEntry(metric Entry) error {
	if strings.TrimSpace(metric.Key) == "" {
		return fmt.Errorf("product metric key is required")
	}
	if strings.TrimSpace(metric.ServiceName) == "" {
		return fmt.Errorf("product metric %q serviceName is required", metric.Key)
	}
	if strings.TrimSpace(metric.Namespace) == "" {
		return fmt.Errorf("product metric %q namespace is required", metric.Key)
	}
	if strings.TrimSpace(metric.MetricName) == "" {
		return fmt.Errorf("product metric %q metricName is required", metric.Key)
	}
	if _, ok := validStatistics[strings.TrimSpace(metric.Statistic)]; !ok {
		return fmt.Errorf("product metric %q has invalid statistic %q", metric.Key, metric.Statistic)
	}
	if _, ok := validPeriods[metric.PeriodSeconds]; !ok {
		return fmt.Errorf("product metric %q has invalid periodSeconds %d", metric.Key, metric.PeriodSeconds)
	}
	if metric.Axis.Min != nil && metric.Axis.Max != nil && *metric.Axis.Min > *metric.Axis.Max {
		return fmt.Errorf("product metric %q axis min must not exceed max", metric.Key)
	}
	if err := validateDimensions(metric); err != nil {
		return err
	}
	return nil
}

func validateDimensions(metric Entry) error {
	seen := map[string]struct{}{}
	for _, dimension := range metric.RequiredDimensions {
		dimension = strings.TrimSpace(dimension)
		if dimension == "" {
			return fmt.Errorf("product metric %q contains empty required dimension", metric.Key)
		}
		if _, exists := seen[dimension]; exists {
			return fmt.Errorf("product metric %q contains duplicate required dimension %q", metric.Key, dimension)
		}
		seen[dimension] = struct{}{}
	}

	switch strings.TrimSpace(metric.ServiceName) {
	case "ec2":
		if strings.TrimSpace(metric.Namespace) == "AWS/EC2" {
			return requireDimensions(metric, "InstanceId")
		}
	case "lambda":
		return requireDimensions(metric, "FunctionName")
	case "api-gateway":
		return requireDimensions(metric, "ApiName", "Stage")
	case "amplify":
		return requireDimensions(metric, "App", "Branch")
	case "s3":
		if strings.TrimSpace(metric.MetricName) == "BucketSizeBytes" || strings.TrimSpace(metric.MetricName) == "NumberOfObjects" {
			return requireDimensions(metric, "BucketName", "StorageType")
		}
		return requireDimensions(metric, "BucketName")
	}

	return nil
}

func requireDimensions(metric Entry, names ...string) error {
	present := map[string]struct{}{}
	for _, dimension := range metric.RequiredDimensions {
		present[strings.TrimSpace(dimension)] = struct{}{}
	}
	for _, name := range names {
		if _, exists := present[name]; !exists {
			return fmt.Errorf("product metric %q missing required dimension %q", metric.Key, name)
		}
	}
	return nil
}

func definitionKey(metric Entry) string {
	dimensions := append([]string(nil), metric.RequiredDimensions...)
	for i := range dimensions {
		dimensions[i] = strings.TrimSpace(dimensions[i])
	}
	sort.Strings(dimensions)
	return strings.Join([]string{
		strings.TrimSpace(metric.ServiceName),
		strings.TrimSpace(metric.Namespace),
		strings.TrimSpace(metric.MetricName),
		strings.TrimSpace(metric.Statistic),
		fmt.Sprintf("%d", metric.PeriodSeconds),
		strings.Join(dimensions, ","),
	}, "\x00")
}

func defaultMetricKey(metric DefaultMetric) string {
	return strings.Join([]string{
		metric.MetricName,
		metric.Statistic,
		fmt.Sprintf("%d", metric.PeriodSeconds),
		metric.Unit,
	}, "\x00")
}
