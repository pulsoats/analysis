package model

import "github.com/pulsoats/core/domain/detect"

type AnalysisSignal struct {
	detect.Signal
	BuyTime  int64
	SellTime int64
	Index    int
}
