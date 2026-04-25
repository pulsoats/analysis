package run

import (
	"time"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/market"
)

type NewRunRequest struct {
	Market   market.Spec
	Interval market.Interval
	From     time.Time
	To       time.Time
	Detector detect.DetectorConfig
	Fees     *market.TakerMakerFees
	UserID   string
}
