package admin

import (
	"encoding/json"
	"fmt"
	"io"

	"cloud-monitor/internal/productcatalog"
	"cloud-monitor/internal/store"
)

type metricSetFile struct {
	Version    int         `json:"version"`
	MetricSets []MetricSet `json:"metricSets"`
}

func LoadMetricSets(r io.Reader) ([]MetricSet, error) {
	var file metricSetFile
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode recommended metric sets: %w", err)
	}
	if file.Version != 1 {
		return nil, fmt.Errorf("recommended metric set version must be 1")
	}
	for _, metricSet := range file.MetricSets {
		if metricSet.ServiceName == "" || metricSet.Namespace == "" || metricSet.Name == "" {
			return nil, fmt.Errorf("recommended metric set identity is required")
		}
		if len(metricSet.Metrics) == 0 {
			return nil, fmt.Errorf("recommended metric set %q must contain metrics", metricSet.Name)
		}
		for _, metric := range metricSet.Metrics {
			if metric.MetricName == "" || metric.Statistic == "" || metric.PeriodSeconds <= 0 {
				return nil, fmt.Errorf("recommended metric set %q contains invalid metric", metricSet.Name)
			}
		}
	}
	return file.MetricSets, nil
}

func LoadMetricSetsFromProductCatalog(r io.Reader) ([]MetricSet, error) {
	catalog, err := productcatalog.Load(r)
	if err != nil {
		return nil, err
	}

	productSets := catalog.RecommendedMetricSets()
	sets := make([]MetricSet, 0, len(productSets))
	for _, productSet := range productSets {
		metrics := make([]store.RecommendedMetric, 0, len(productSet.Metrics))
		for _, metric := range productSet.Metrics {
			metrics = append(metrics, store.RecommendedMetric{
				MetricName:    metric.MetricName,
				Statistic:     metric.Statistic,
				PeriodSeconds: int32(metric.PeriodSeconds),
				Unit:          metric.Unit,
			})
		}
		sets = append(sets, MetricSet{
			ServiceName: productSet.ServiceName,
			Namespace:   productSet.Namespace,
			Name:        productSet.Name,
			Metrics:     metrics,
		})
	}
	return sets, nil
}
