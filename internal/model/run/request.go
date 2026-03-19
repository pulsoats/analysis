package run

import (
	"fmt"
	"time"

	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
)

type Request struct {
	UserID    string
	Market    market.Spec
	Interval  market.Interval
	From      time.Time
	To        time.Time
	PriceType market.PriceType
	Detector  detect.DetectorConfig
	Fees      *market.TakerMakerFees
}

func (cfg *Request) Validate() error {
	var zero market.Spec
	if cfg.Market == zero {
		return fmt.Errorf("%w: market spec", derrors.ErrRequired)
	}
	if cfg.Market.Exchange == "" || cfg.Market.Category == "" {
		return fmt.Errorf("%w: market exchange or/and category", derrors.ErrRequired)
	}
	if cfg.Market.Symbol == "" {
		return fmt.Errorf("%w: market symbol", derrors.ErrRequired)
	}

	if cfg.From.IsZero() || cfg.To.IsZero() {
		return fmt.Errorf("%w: time range", derrors.ErrRequired)
	}

	if cfg.Detector.Code == "" {
		return fmt.Errorf("%w: detector code", derrors.ErrRequired)
	}

	if cfg.UserID == "" {
		return fmt.Errorf("%w: user_id", derrors.ErrRequired)
	}

	return nil
}
