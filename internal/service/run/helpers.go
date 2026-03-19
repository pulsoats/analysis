package run

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/pulsoats/core/domain/market"
)

func timeBounds(candles []market.Candle) (time.Time, time.Time) {
	if len(candles) == 0 {
		return time.Time{}, time.Time{}
	}
	minTS := candles[0].Time
	maxTS := candles[0].Time
	for i := 1; i < len(candles); i++ {
		if candles[i].Time < minTS {
			minTS = candles[i].Time
		}
		if candles[i].Time > maxTS {
			maxTS = candles[i].Time
		}
	}
	return time.UnixMilli(minTS).UTC(), time.UnixMilli(maxTS).UTC()
}

func (s *service) runZipPath(runID int64) string {
	filename := fmt.Sprintf("run_%d.zip", runID)
	return filepath.Join(s.storageDir, filename)
}
