package files

import (
	"fmt"

	"github.com/pulsoats/core/domain/market"
)

type SignalsFileMeta struct {
	Exchange string
	Category market.Category
	Interval market.Interval
	Symbol   string
	RunID    string
}

func SignalsFilename(m SignalsFileMeta) string {
	intervalSlug, _ := formatIntervalSlug(m.Interval)

	return fmt.Sprintf(
		"signals_%s_%s_%s_%s_%s.csv",
		m.Exchange,
		m.Category,
		intervalSlug,
		m.Symbol,
		m.RunID,
	)
}
