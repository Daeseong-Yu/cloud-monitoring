package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type Catalog struct {
	Version    int         `json:"version"`
	Resources  []Resource  `json:"resources"`
	MetricSets []MetricSet `json:"metricSets"`
	Bindings   []Binding   `json:"bindings"`
}

type Resource struct {
	Key           string `json:"key"`
	ServiceName   string `json:"serviceName"`
	ResourceIDEnv string `json:"resourceIdEnv"`
	RegionEnv     string `json:"regionEnv"`
	DimensionName string `json:"dimensionName,omitempty"`
}

type MetricSet struct {
	Name    string   `json:"name"`
	Metrics []Metric `json:"metrics"`
}

type Metric struct {
	ServiceName   string      `json:"serviceName,omitempty"`
	Namespace     string      `json:"namespace"`
	MetricName    string      `json:"metricName"`
	Dimensions    []Dimension `json:"dimensions,omitempty"`
	Statistic     string      `json:"statistic"`
	PeriodSeconds int         `json:"periodSeconds"`
	Unit          string      `json:"unit,omitempty"`
}

type Dimension struct {
	Name     string `json:"name"`
	Value    string `json:"value,omitempty"`
	ValueEnv string `json:"valueEnv,omitempty"`
}

type Binding struct {
	ResourceKey string `json:"resourceKey"`
	MetricSet   string `json:"metricSet"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

type Definition struct {
	ServiceName   string
	Namespace     string
	MetricName    string
	ResourceID    string
	Region        string
	Dimensions    []ResolvedDimension
	Statistic     string
	PeriodSeconds int
	Unit          string
	Enabled       bool
}

type ResolvedDimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func Load(r io.Reader) (Catalog, error) {
	var c Catalog
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return Catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Catalog{}, err
	}
	return c, nil
}

func LoadFile(path string) (Catalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("open catalog: %w", err)
	}
	defer file.Close()

	return Load(file)
}

func (c Catalog) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("catalog version must be 1")
	}
	if len(c.Resources) == 0 {
		return fmt.Errorf("catalog must contain at least one resource")
	}
	if len(c.MetricSets) == 0 {
		return fmt.Errorf("catalog must contain at least one metric set")
	}
	if len(c.Bindings) == 0 {
		return fmt.Errorf("catalog must contain at least one binding")
	}

	resourceKeys := map[string]struct{}{}
	for _, resource := range c.Resources {
		if err := validateResource(resource); err != nil {
			return err
		}
		if _, exists := resourceKeys[resource.Key]; exists {
			return fmt.Errorf("duplicate resource key %q", resource.Key)
		}
		resourceKeys[resource.Key] = struct{}{}
	}

	metricSetNames := map[string]struct{}{}
	for _, metricSet := range c.MetricSets {
		if strings.TrimSpace(metricSet.Name) == "" {
			return fmt.Errorf("metric set name is required")
		}
		if len(metricSet.Metrics) == 0 {
			return fmt.Errorf("metric set %q must contain at least one metric", metricSet.Name)
		}
		if _, exists := metricSetNames[metricSet.Name]; exists {
			return fmt.Errorf("duplicate metric set %q", metricSet.Name)
		}
		metricSetNames[metricSet.Name] = struct{}{}

		for _, metric := range metricSet.Metrics {
			if err := validateMetric(metric); err != nil {
				return fmt.Errorf("metric set %q: %w", metricSet.Name, err)
			}
		}
	}

	for _, binding := range c.Bindings {
		if _, exists := resourceKeys[binding.ResourceKey]; !exists {
			return fmt.Errorf("binding references unknown resource %q", binding.ResourceKey)
		}
		if _, exists := metricSetNames[binding.MetricSet]; !exists {
			return fmt.Errorf("binding references unknown metric set %q", binding.MetricSet)
		}
	}

	return nil
}

func (c Catalog) Resolve(getenv func(string) string) ([]Definition, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	resources := make(map[string]Resource, len(c.Resources))
	for _, resource := range c.Resources {
		resources[resource.Key] = resource
	}

	metricSets := make(map[string]MetricSet, len(c.MetricSets))
	for _, metricSet := range c.MetricSets {
		metricSets[metricSet.Name] = metricSet
	}

	var definitions []Definition
	seen := map[string]struct{}{}

	for _, binding := range c.Bindings {
		resource := resources[binding.ResourceKey]
		resourceID := strings.TrimSpace(getenv(resource.ResourceIDEnv))
		if resourceID == "" {
			return nil, fmt.Errorf("environment variable %s is required", resource.ResourceIDEnv)
		}

		region := strings.TrimSpace(getenv(resource.RegionEnv))
		if region == "" {
			return nil, fmt.Errorf("environment variable %s is required", resource.RegionEnv)
		}

		enabled := true
		if binding.Enabled != nil {
			enabled = *binding.Enabled
		}

		for _, metric := range metricSets[binding.MetricSet].Metrics {
			serviceName := strings.TrimSpace(metric.ServiceName)
			if serviceName == "" {
				serviceName = resource.ServiceName
			}

			dimensions := resolveMetricDimensions(resource, metric, getenv, resourceID)
			for _, dimension := range dimensions {
				if strings.TrimSpace(dimension.Value) == "" {
					return nil, fmt.Errorf("metric %q dimension %q value is required", metric.MetricName, dimension.Name)
				}
			}

			def := Definition{
				ServiceName:   serviceName,
				Namespace:     strings.TrimSpace(metric.Namespace),
				MetricName:    strings.TrimSpace(metric.MetricName),
				ResourceID:    resourceID,
				Region:        region,
				Dimensions:    dimensions,
				Statistic:     strings.TrimSpace(metric.Statistic),
				PeriodSeconds: metric.PeriodSeconds,
				Unit:          strings.TrimSpace(metric.Unit),
				Enabled:       enabled,
			}

			key := definitionKey(def)
			if _, exists := seen[key]; exists {
				return nil, fmt.Errorf("duplicate resolved metric definition %s", key)
			}
			seen[key] = struct{}{}
			definitions = append(definitions, def)
		}
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitionKey(definitions[i]) < definitionKey(definitions[j])
	})

	return definitions, nil
}

func validateResource(resource Resource) error {
	if strings.TrimSpace(resource.Key) == "" {
		return fmt.Errorf("resource key is required")
	}
	if strings.TrimSpace(resource.ServiceName) == "" {
		return fmt.Errorf("resource %q serviceName is required", resource.Key)
	}
	if strings.TrimSpace(resource.ResourceIDEnv) == "" {
		return fmt.Errorf("resource %q resourceIdEnv is required", resource.Key)
	}
	if strings.TrimSpace(resource.RegionEnv) == "" {
		return fmt.Errorf("resource %q regionEnv is required", resource.Key)
	}
	return nil
}

func validateMetric(metric Metric) error {
	if strings.TrimSpace(metric.Namespace) == "" {
		return fmt.Errorf("metric namespace is required")
	}
	if strings.TrimSpace(metric.MetricName) == "" {
		return fmt.Errorf("metricName is required")
	}
	if strings.TrimSpace(metric.Statistic) == "" {
		return fmt.Errorf("metric %q statistic is required", metric.MetricName)
	}
	if metric.PeriodSeconds <= 0 {
		return fmt.Errorf("metric %q periodSeconds must be positive", metric.MetricName)
	}
	for _, dimension := range metric.Dimensions {
		if strings.TrimSpace(dimension.Name) == "" {
			return fmt.Errorf("metric %q dimension name is required", metric.MetricName)
		}
		if strings.TrimSpace(dimension.Value) == "" && strings.TrimSpace(dimension.ValueEnv) == "" {
			return fmt.Errorf("metric %q dimension %q requires value or valueEnv", metric.MetricName, dimension.Name)
		}
		if strings.TrimSpace(dimension.Value) != "" && strings.TrimSpace(dimension.ValueEnv) != "" {
			return fmt.Errorf("metric %q dimension %q cannot use both value and valueEnv", metric.MetricName, dimension.Name)
		}
	}
	return nil
}

func resolveMetricDimensions(resource Resource, metric Metric, getenv func(string) string, resourceID string) []ResolvedDimension {
	var dimensions []ResolvedDimension
	if len(metric.Dimensions) > 0 {
		for _, dimension := range metric.Dimensions {
			value := strings.TrimSpace(dimension.Value)
			if value == "" {
				value = strings.TrimSpace(getenv(dimension.ValueEnv))
			}
			dimensions = append(dimensions, ResolvedDimension{
				Name:  strings.TrimSpace(dimension.Name),
				Value: value,
			})
		}
	} else {
		dimensionName := strings.TrimSpace(resource.DimensionName)
		if dimensionName == "" {
			dimensionName = "InstanceId"
		}
		dimensions = append(dimensions, ResolvedDimension{
			Name:  dimensionName,
			Value: resourceID,
		})
	}

	sort.Slice(dimensions, func(i, j int) bool {
		if dimensions[i].Name == dimensions[j].Name {
			return dimensions[i].Value < dimensions[j].Value
		}
		return dimensions[i].Name < dimensions[j].Name
	})

	return dimensions
}

func definitionKey(def Definition) string {
	dimensionParts := make([]string, 0, len(def.Dimensions))
	for _, dimension := range def.Dimensions {
		dimensionParts = append(dimensionParts, dimension.Name+"="+dimension.Value)
	}
	return strings.Join([]string{
		def.Namespace,
		def.MetricName,
		def.ResourceID,
		def.Region,
		strings.Join(dimensionParts, ","),
		def.Statistic,
		fmt.Sprintf("%d", def.PeriodSeconds),
	}, "\x00")
}
