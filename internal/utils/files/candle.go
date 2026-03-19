package files

import (
	"fmt"
	"time"

	"github.com/pulsoats/core/domain/market"
)

type CandlesFileMeta struct {
	Exchange string
	Category market.Category
	Interval market.Interval
	Symbol   string
	From     time.Time
	To       time.Time
}

func CandlesFilename(m CandlesFileMeta) string {
	from := m.From.UTC().Format("20060102")
	to := m.To.UTC().Format("20060102")

	intervalSlug, _ := formatIntervalSlug(m.Interval)

	return fmt.Sprintf(
		"candles_%s_%s_%s_%s_%s-%s.csv",
		m.Exchange,
		m.Category,
		intervalSlug,
		m.Symbol,
		from,
		to,
	)
}
