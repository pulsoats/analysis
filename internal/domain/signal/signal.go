package signal

import (
	"github.com/pulsoats/core/detect"
)

type AnalysisSignal struct {
	detect.Signal
	Status   string
	BuyTime  int64
	SellTime int64
	Index    int
}
