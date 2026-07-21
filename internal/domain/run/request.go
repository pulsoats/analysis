package run

import (
	"time"

	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/market"
)

type NewRunRequest struct {
	Market          market.Spec
	Interval        market.Interval
	From            time.Time
	To              time.Time
	Detector        detector.Config
	Filters         []filter.Config
	Fees            *market.TakerMakerFees
	DisableStopLoss bool
	DisableRepeats  bool
	UserID          string
}
