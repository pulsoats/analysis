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
	DetectorConfig  detector.Config
	FiltersConfigs  []filter.Config
	Fees            *market.TakerMakerFees
	DisableStopLoss bool
	DisableRepeats  bool
	UserID          string
}
