package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"cloud-monitor/internal/admin"
	"cloud-monitor/internal/sanitize"
	"cloud-monitor/internal/store"
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "admin configuration error: DATABASE_URL is required")
		os.Exit(2)
	}
	username := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	password := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	addr := strings.TrimSpace(os.Getenv("ADMIN_HTTP_ADDR"))
	if addr == "" {
		addr = ":8080"
	}

	ctx := context.Background()
	db, err := store.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin database error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}
	defer db.Close()

	metricSetsFile, err := os.Open("configs/product-metric-catalog.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin product metric catalog error: %v\n", err)
		os.Exit(1)
	}
	defer metricSetsFile.Close()
	metricSets, err := admin.LoadMetricSetsFromProductCatalog(metricSetsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin product metric catalog error: %v\n", err)
		os.Exit(1)
	}

	server, err := admin.NewServer(admin.Config{
		Store:      db,
		Discovery:  commandDiscoveryRunner{databaseURL: databaseURL},
		Username:   username,
		Password:   password,
		Region:     region,
		MetricSets: metricSets,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "admin configuration error: %v\n", err)
		os.Exit(2)
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Printf("cloud-monitor admin ready: addr=%s region=%s\n", addr, region)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "admin server error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}
}

type commandDiscoveryRunner struct {
	databaseURL string
}

func (r commandDiscoveryRunner) Run(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	binary := strings.TrimSpace(os.Getenv("RESOURCE_DISCOVERY_BIN"))
	if binary == "" {
		binary = "/app/resource-discovery"
	}
	if _, err := os.Stat(binary); err != nil {
		binary = "resource-discovery"
	}

	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s", sanitize.Message(message, r.databaseURL))
	}

	message := strings.TrimSpace(stdout.String())
	if message == "" {
		message = "resource discovery completed"
	}
	return sanitize.Message(message, r.databaseURL), nil
}
