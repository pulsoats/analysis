package candles

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulsoats/analysis/internal/model"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/lib/errorsx"
)

type repo struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) model.CandleRepository {
	return &repo{pool: pool}
}

func (r *repo) Upsert(ctx context.Context, spec market.CandleSpec, candles []market.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("upsert: begin tx: %w: %v", errorsx.ErrInternal, err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `TRUNCATE analysis.candles_staging`); err != nil {
		return fmt.Errorf("upsert: truncate staging: %w: %v", errorsx.ErrInternal, err)
	}

	rows := make([][]any, 0, len(candles))
	for i := range candles {
		c := candles[i]
		rows = append(rows, []any{
			spec.Exchange,
			spec.Category,
			spec.Symbol,
			intervalSec(spec.Interval),
			time.UnixMilli(c.Time).UTC(),
			c.Open,
			c.High,
			c.Low,
			c.Close,
			c.Volume,
			c.Turnover,
			c.PriceType,
		})
	}

	cols := []string{
		"exchange", "category", "symbol", "interval", "time",
		"open_price", "high_price", "low_price", "close_price",
		"volume", "turnover", "price_type",
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"analysis", "candles_staging"},
		cols,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("upsert: copy into staging: %w: %v", errorsx.ErrInternal, err)
	}

	const mergeSQL = `
INSERT INTO analysis.candles (
  exchange, category, symbol, interval, time,
  open_price, high_price, low_price, close_price,
  volume, turnover, price_type
)
SELECT
  exchange, category, symbol, interval, time,
  open_price, high_price, low_price, close_price,
  volume, turnover, price_type
FROM analysis.candles_staging
ON CONFLICT (exchange, category, symbol, interval, time, price_type)
DO NOTHING;
`
	if _, err := tx.Exec(ctx, mergeSQL); err != nil {
		return fmt.Errorf("upsert: merge into candles: %w: %v", errorsx.ErrInternal, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("upsert: commit: %w: %v", errorsx.ErrInternal, err)
	}
	return nil
}

func (r *repo) ListByTime(ctx context.Context, spec market.CandleSpec, from, to time.Time, priceType market.PriceType) ([]market.Candle, error) {
	const query = `
		SELECT time, open_price, high_price, low_price, close_price, volume, turnover, price_type
		FROM analysis.candles
		WHERE exchange = $1 
		  AND category = $2 
		  AND symbol = $3 
		  AND interval = $4 
		  AND time >= $5 AND time < $6
		  AND price_type = $7
		ORDER BY time;
	`

	if from.After(to) {
		return nil, fmt.Errorf("list by time: %w: from > to", derrors.ErrInvalidArgument)
	}

	rows, err := r.pool.Query(ctx, query, spec.Exchange, spec.Category, spec.Symbol, intervalSec(spec.Interval), from, to, priceType)
	if err != nil {
		return nil, fmt.Errorf("list by time: %w: %v", errorsx.ErrInternal, err)
	}
	defer rows.Close()

	candles, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (market.Candle, error) {
		var c market.Candle
		var ts time.Time
		if err := row.Scan(&ts, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume, &c.Turnover, &c.PriceType); err != nil {
			return market.Candle{}, fmt.Errorf("list by time: %w: %v", errorsx.ErrInternal, err)
		}
		c.Time = ts.UTC().UnixMilli()
		return c, nil
	})
	if err != nil {
		return nil, fmt.Errorf("list by time: %w: %v", errorsx.ErrInternal, err)
	}

	return candles, nil
}

func intervalSec(i market.Interval) int {
	return int(time.Duration(i) / time.Second)
}
