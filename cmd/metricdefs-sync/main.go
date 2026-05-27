package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"cloud-monitor/internal/catalog"
	"cloud-monitor/internal/sanitize"
	"cloud-monitor/internal/store"
)

func main() {
	configPath := flag.String("config", "", "metric definition catalog JSON path")
	dryRun := flag.Bool("dry-run", false, "print resolved metric definitions without applying them")
	flag.Parse()

	if strings.TrimSpace(*configPath) == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}

	c, err := catalog.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric definition catalog error: %v\n", err)
		os.Exit(1)
	}

	definitions, err := c.Resolve(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric definition catalog error: %v\n", err)
		os.Exit(1)
	}

	inputs, err := metricDefinitionInputs(definitions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric definition catalog error: %v\n", err)
		os.Exit(1)
	}
	if *dryRun {
		if err := json.NewEncoder(os.Stdout).Encode(inputs); err != nil {
			fmt.Fprintf(os.Stderr, "metric definition dry-run error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}

	ctx := context.Background()
	db, err := store.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric definition sync database error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}
	defer db.Close()

	for _, input := range inputs {
		if _, err := db.UpsertMetricDefinition(ctx, input); err != nil {
			fmt.Fprintf(os.Stderr, "metric definition sync failed: %s\n", sanitize.Message(err.Error(), databaseURL))
			os.Exit(1)
		}
	}

	fmt.Printf("metric definition sync applied: definitions=%d\n", len(inputs))
}

func metricDefinitionInputs(definitions []catalog.Definition) ([]store.MetricDefinitionInput, error) {
	inputs := make([]store.MetricDefinitionInput, 0, len(definitions))
	for _, def := range definitions {
		dimensionsJSON, err := dimensionsJSON(def.Dimensions)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, store.MetricDefinitionInput{
			ServiceName:    def.ServiceName,
			Namespace:      def.Namespace,
			MetricName:     def.MetricName,
			ResourceID:     def.ResourceID,
			Region:         def.Region,
			DimensionsJSON: dimensionsJSON,
			Statistic:      def.Statistic,
			PeriodSeconds:  int32(def.PeriodSeconds),
			Unit:           def.Unit,
			Enabled:        def.Enabled,
		})
	}
	return inputs, nil
}

func dimensionsJSON(dimensions []catalog.ResolvedDimension) (string, error) {
	data, err := json.Marshal(dimensions)
	if err != nil {
		return "", fmt.Errorf("marshal metric dimensions: %w", err)
	}
	return string(data), nil
}
