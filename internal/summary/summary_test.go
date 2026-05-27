package summary

import (
	"testing"
	"time"
)

func TestBucketsUseUTC(t *testing.T) {
	location := time.FixedZone("test", 9*60*60)
	input := time.Date(2026, 5, 27, 12, 34, 56, 0, location)

	if got := HourBucket(input); got.Location() != time.UTC || got.Minute() != 0 || got.Second() != 0 {
		t.Fatalf("bad hour bucket: %s", got)
	}
	if got := DayBucket(input); got.Location() != time.UTC || got.Hour() != 0 || got.Minute() != 0 {
		t.Fatalf("bad day bucket: %s", got)
	}
}
