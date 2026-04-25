package candle

import (
	"context"
	"time"

	"github.com/pulsoats/core/market"
)

type Repository interface {
	Upsert(ctx context.Context, spec market.Spec, interval market.Interval, candles []market.Candle) error
	ListByTime(ctx context.Context, spec market.Spec, interval market.Interval, from, to time.Time) ([]market.Candle, error)
}
