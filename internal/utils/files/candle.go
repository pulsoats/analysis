package files

import (
	"fmt"
	"strings"
	"time"

	"github.com/pulsoats/core/market"
)

type CandlesFileMeta struct {
	Exchange string
	Category string
	Interval market.Interval
	Symbol   string
	From     time.Time
	To       time.Time
}

func CandlesFilename(m CandlesFileMeta) string {
	from := m.From.UTC().Format("20060102")
	to := m.To.UTC().Format("20060102")

	filename := strings.ToUpper(fmt.Sprintf(
		"%s_%s_%s_%s_%s-%s",
		m.Exchange,
		m.Category,
		m.Interval,
		m.Symbol,
		from,
		to,
	))

	return "candles_" + filename + ".csv"
}
