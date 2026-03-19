package files

import (
	"fmt"
	"time"

	"github.com/pulsoats/core/domain/market"
)

var intervalToSlug = map[market.Interval]string{
	market.Interval1m:  "1m",
	market.Interval3m:  "3m",
	market.Interval5m:  "5m",
	market.Interval15m: "15m",
	market.Interval30m: "30m",
	market.Interval1h:  "1h",
	market.Interval2h:  "2h",
	market.Interval4h:  "4h",
	market.Interval6h:  "6h",
	market.Interval12h: "12h",
	market.Interval1d:  "1d",
	market.Interval3d:  "3d",
	market.Interval1w:  "1w",
	market.Interval1M:  "1M",
}

var slugToInterval = func() map[string]market.Interval {
	m := make(map[string]market.Interval, len(intervalToSlug))
	for iv, slug := range intervalToSlug {
		m[slug] = iv
	}
	return m
}()

func formatIntervalSlug(iv market.Interval) (string, error) {
	if slug, ok := intervalToSlug[iv]; ok {
		return slug, nil
	}
	if iv == 0 {
		return "", fmt.Errorf("interval must be set")
	}
	return "", fmt.Errorf("unsupported interval: %s", time.Duration(iv))
}

func parseIntervalSlug(slug string) (market.Interval, error) {
	if iv, ok := slugToInterval[slug]; ok {
		return iv, nil
	}
	return 0, fmt.Errorf("unsupported interval slug: %s", slug)
}
