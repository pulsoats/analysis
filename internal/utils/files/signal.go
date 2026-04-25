package files

import (
	"fmt"
	"strings"

	"github.com/pulsoats/core/market"
)

type SignalsFileMeta struct {
	Exchange string
	Category string
	Interval market.Interval
	Symbol   string
	RunID    string
}

func SignalsFilename(m SignalsFileMeta) string {
	filename := strings.ToUpper(fmt.Sprintf(
		"%s_%s_%s_%s",
		m.Exchange,
		m.Category,
		m.Interval,
		m.Symbol,
	))

	return "signals_" + filename + "_" + m.RunID + ".csv"
}
