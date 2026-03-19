package model

import (
	"context"
	"time"

	"github.com/pulsoats/core/domain/market"
)

type CandleRepository interface {
	Upsert(ctx context.Context, spec market.CandleSpec, candles []market.Candle) error
	ListByTime(ctx context.Context, spec market.CandleSpec, from, to time.Time, priceType market.PriceType) ([]market.Candle, error)
}
