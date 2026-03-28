package run

import (
	"fmt"
	"time"

	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
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
		return fmt.Errorf("market spec: %w", errorsx.ErrRequired)
	}
	if cfg.Market.Exchange == "" || cfg.Market.Category == "" {
		return fmt.Errorf("market exchange or/and category: %w", errorsx.ErrRequired)
	}
	if cfg.Market.Symbol == "" {
		return fmt.Errorf("market symbol: %w", errorsx.ErrRequired)
	}

	if cfg.From.IsZero() || cfg.To.IsZero() {
		return fmt.Errorf("time range: %w", errorsx.ErrRequired)
	}

	if cfg.Detector.Code == "" {
		return fmt.Errorf("detector code: %w", errorsx.ErrRequired)
	}

	if cfg.UserID == "" {
		return fmt.Errorf("user_id: %w", errorsx.ErrRequired)
	}

	return nil
}
