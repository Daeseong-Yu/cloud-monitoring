package admin

import (
	"encoding/json"
	"fmt"
	"io"
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
