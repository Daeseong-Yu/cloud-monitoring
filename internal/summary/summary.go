package summary

import "time"

func HourBucket(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}

func DayBucket(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
