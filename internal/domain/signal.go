package domain

import (
	"github.com/pulsoats/core/domain/detect"
)

type AnalysisSignal struct {
	detect.Signal
	Status   string
	BuyTime  int64
	SellTime int64
	Index    int
}
