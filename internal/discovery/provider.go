package discovery

import (
	"fmt"
	"sort"
	"strings"

	"cloud-monitor/internal/productcatalog"
)

const (
	AvailabilityAvailable     = "available"
	AvailabilityNotSeen       = "not_seen"
	AvailabilityRequiresSetup = "requires_setup"
	AvailabilityUnsupported   = "unsupported"
)

type Provider interface {
	Name() string
	ServiceName() string
	Namespaces() []string
	Discover(region string, metrics []Metric, tags map[string]TagInfo, catalog productcatalog.Catalog) []Resource
}

type Registry struct {
	providers []Provider
}

func NewRegistry(providers ...Provider) Registry {
	return Registry{providers: append([]Provider(nil), providers...)}
}

func DefaultRegistry() Registry {
	return NewRegistry(
		StaticProvider{
			name:              "ec2-provider",
			serviceName:       "ec2",
			namespaces:        []string{"AWS/EC2", "CWAgent"},
			resourceDimension: "InstanceId",
		},
		StaticProvider{
			name:              "lambda-provider",
			serviceName:       "lambda",
			namespaces:        []string{"AWS/Lambda"},
			resourceDimension: "FunctionName",
		},
		StaticProvider{
			name:              "api-gateway-provider",
			serviceName:       "api-gateway",
			namespaces:        []string{"AWS/ApiGateway"},
			resourceDimension: "ApiName",
		},
		StaticProvider{
			name:              "amplify-provider",
			serviceName:       "amplify",
			namespaces:        []string{"AWS/AmplifyHosting"},
			resourceDimension: "App",
		},
		StaticProvider{
			name:              "ses-provider",
			serviceName:       "ses",
			namespaces:        []string{"AWS/SES"},
			resourceDimension: "",
			accountLevelID:    "ses",
			displayName:       "SES",
		},
		StaticProvider{
			name:              "s3-provider",
			serviceName:       "s3",
			namespaces:        []string{"AWS/S3"},
			resourceDimension: "BucketName",
		},
	)
}

func (r Registry) Providers() []Provider {
	return append([]Provider(nil), r.providers...)
}

func (r Registry) Namespaces() []string {
	seen := map[string]struct{}{}
	for _, provider := range r.providers {
		for _, namespace := range provider.Namespaces() {
			namespace = strings.TrimSpace(namespace)
			if namespace == "" {
				continue
			}
			seen[namespace] = struct{}{}
		}
	}
	namespaces := make([]string, 0, len(seen))
	for namespace := range seen {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	return namespaces
}

func (r Registry) Discover(region string, metrics []Metric, tags map[string]TagInfo, catalog productcatalog.Catalog) []Resource {
	var resources []Resource
	for _, provider := range r.providers {
		resources = append(resources, provider.Discover(region, metrics, tags, catalog)...)
	}
	sort.Slice(resources, func(i, j int) bool {
		return resourceSortKey(resources[i]) < resourceSortKey(resources[j])
	})
	return resources
}

type StaticProvider struct {
	name              string
	serviceName       string
	namespaces        []string
	resourceDimension string
	accountLevelID    string
	displayName       string
}

func (p StaticProvider) Name() string {
	return p.name
}

func (p StaticProvider) ServiceName() string {
	return p.serviceName
}

func (p StaticProvider) Namespaces() []string {
	return append([]string(nil), p.namespaces...)
}

func (p StaticProvider) Discover(region string, metrics []Metric, tags map[string]TagInfo, catalog productcatalog.Catalog) []Resource {
	catalogMetrics := catalogMetricsForService(catalog, p.serviceName)
	if len(catalogMetrics) == 0 {
		return nil
	}

	observedByResource := map[string][]Metric{}
	for _, metric := range metrics {
		if !p.supportsNamespace(metric.Namespace) {
			continue
		}
		dimensions := NormalizeDimensions(metric.Dimensions)
		resourceID := p.resourceID(dimensions)
		if resourceID == "" {
			continue
		}
		observedByResource[resourceID] = append(observedByResource[resourceID], Metric{
			Namespace:  strings.TrimSpace(metric.Namespace),
			MetricName: strings.TrimSpace(metric.MetricName),
			Dimensions: dimensions,
		})
	}

	resources := make([]Resource, 0, len(observedByResource))
	for resourceID, observed := range observedByResource {
		tagInfo := tags[resourceID]
		displayName := strings.TrimSpace(tagInfo.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(p.displayName)
		}
		if displayName == "" {
			displayName = resourceID
		}

		resource := Resource{
			ServiceName:    p.serviceName,
			Namespace:      p.namespaces[0],
			ResourceID:     resourceID,
			Region:         region,
			DisplayName:    displayName,
			Tags:           tagInfo.Tags,
			ProviderSource: p.name,
			Metrics:        p.metricCandidates(observed, catalogMetrics),
		}
		resources = append(resources, resource)
	}

	sort.Slice(resources, func(i, j int) bool {
		return resourceSortKey(resources[i]) < resourceSortKey(resources[j])
	})
	return resources
}

func (p StaticProvider) metricCandidates(observed []Metric, catalogMetrics []productcatalog.Entry) []DiscoveredMetric {
	checker := NewAvailabilityChecker(observed)
	candidates := make([]DiscoveredMetric, 0, len(catalogMetrics))
	for _, entry := range catalogMetrics {
		status, reason, dimensions := checker.Check(entry)
		if status == AvailabilityNotSeen && strings.TrimSpace(entry.Prerequisite) != "" {
			status = AvailabilityRequiresSetup
			reason = entry.Prerequisite
		}
		if len(dimensions) == 0 && len(entry.RequiredDimensions) > 0 {
			dimensions = checker.DimensionsForNames(entry.RequiredDimensions)
		}
		if len(dimensions) == 0 && len(entry.RequiredDimensions) > 0 {
			status = AvailabilityUnsupported
			reason = "required CloudWatch dimensions were not observed"
		}

		candidates = append(candidates, DiscoveredMetric{
			Namespace:          strings.TrimSpace(entry.Namespace),
			MetricName:         strings.TrimSpace(entry.MetricName),
			Dimensions:         dimensions,
			Statistic:          strings.TrimSpace(entry.Statistic),
			PeriodSeconds:      entry.PeriodSeconds,
			Unit:               strings.TrimSpace(entry.Unit),
			AvailabilityStatus: status,
			AvailabilityReason: reason,
			ProviderSource:     p.name,
			Prerequisite:       strings.TrimSpace(entry.Prerequisite),
			CostWarning:        strings.TrimSpace(entry.CostWarning),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return metricKey(candidates[i]) < metricKey(candidates[j])
	})
	return dedupeMetrics(candidates)
}

func (p StaticProvider) supportsNamespace(namespace string) bool {
	namespace = strings.TrimSpace(namespace)
	for _, supported := range p.namespaces {
		if namespace == supported {
			return true
		}
	}
	return false
}

func (p StaticProvider) resourceID(dimensions []Dimension) string {
	if strings.TrimSpace(p.accountLevelID) != "" {
		return p.accountLevelID
	}
	for _, dimension := range dimensions {
		if dimension.Name == p.resourceDimension {
			return dimension.Value
		}
	}
	return ""
}

type AvailabilityChecker struct {
	metrics []Metric
}

func NewAvailabilityChecker(metrics []Metric) AvailabilityChecker {
	normalized := make([]Metric, 0, len(metrics))
	for _, metric := range metrics {
		normalized = append(normalized, Metric{
			Namespace:  strings.TrimSpace(metric.Namespace),
			MetricName: strings.TrimSpace(metric.MetricName),
			Dimensions: NormalizeDimensions(metric.Dimensions),
		})
	}
	return AvailabilityChecker{metrics: normalized}
}

func (c AvailabilityChecker) Check(entry productcatalog.Entry) (string, string, []Dimension) {
	for _, metric := range c.metrics {
		if metric.Namespace != strings.TrimSpace(entry.Namespace) || metric.MetricName != strings.TrimSpace(entry.MetricName) {
			continue
		}
		if !containsRequiredDimensions(metric.Dimensions, entry.RequiredDimensions) {
			return AvailabilityUnsupported, "required CloudWatch dimensions were not observed", metric.Dimensions
		}
		return AvailabilityAvailable, "", filterDimensions(metric.Dimensions, entry.RequiredDimensions)
	}
	return AvailabilityNotSeen, "metric was not seen in CloudWatch ListMetrics", nil
}

func (c AvailabilityChecker) DimensionsForNames(names []string) []Dimension {
	seen := map[string]string{}
	for _, metric := range c.metrics {
		for _, dimension := range metric.Dimensions {
			if _, exists := seen[dimension.Name]; !exists {
				seen[dimension.Name] = dimension.Value
			}
		}
	}

	dimensions := make([]Dimension, 0, len(names))
	for _, name := range names {
		value := strings.TrimSpace(seen[name])
		if value == "" {
			return nil
		}
		dimensions = append(dimensions, Dimension{Name: name, Value: value})
	}
	return NormalizeDimensions(dimensions)
}

func catalogMetricsForService(catalog productcatalog.Catalog, serviceName string) []productcatalog.Entry {
	var metrics []productcatalog.Entry
	for _, metric := range catalog.Metrics {
		if strings.TrimSpace(metric.ServiceName) == serviceName {
			metrics = append(metrics, metric)
		}
	}
	sort.Slice(metrics, func(i, j int) bool {
		return fmt.Sprintf("%s\x00%s", metrics[i].Namespace, metrics[i].MetricName) <
			fmt.Sprintf("%s\x00%s", metrics[j].Namespace, metrics[j].MetricName)
	})
	return metrics
}

func containsRequiredDimensions(dimensions []Dimension, required []string) bool {
	present := map[string]struct{}{}
	for _, dimension := range dimensions {
		present[dimension.Name] = struct{}{}
	}
	for _, name := range required {
		if _, ok := present[strings.TrimSpace(name)]; !ok {
			return false
		}
	}
	return true
}

func filterDimensions(dimensions []Dimension, names []string) []Dimension {
	if len(names) == 0 {
		return nil
	}
	allowed := map[string]struct{}{}
	for _, name := range names {
		allowed[strings.TrimSpace(name)] = struct{}{}
	}

	filtered := make([]Dimension, 0, len(names))
	for _, dimension := range dimensions {
		if _, ok := allowed[dimension.Name]; ok {
			filtered = append(filtered, dimension)
		}
	}
	return NormalizeDimensions(filtered)
}

func resourceSortKey(resource Resource) string {
	return strings.Join([]string{resource.Region, resource.ServiceName, resource.ResourceID}, "\x00")
}
