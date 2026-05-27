package cloudwatchmetrics

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

type RegionClientFactory struct {
	cfg aws.Config
}

func NewRegionClientFactory(cfg aws.Config) RegionClientFactory {
	return RegionClientFactory{cfg: cfg}
}

func (f RegionClientFactory) FetcherForRegion(region string) Fetcher {
	cfg := f.cfg
	if strings.TrimSpace(region) != "" {
		cfg.Region = strings.TrimSpace(region)
	}
	return NewFetcher(cloudwatch.NewFromConfig(cfg))
}
