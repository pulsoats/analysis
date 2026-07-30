package run

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/market"
)

func (a *Application) fetchCandles(ctx context.Context, spec market.Spec, interval market.Interval, from, to time.Time) ([]market.Candle, error) {
	const op = "fetch candles"
	if !from.Before(to) {
		return nil, fmt.Errorf("%s: from %s >= to %s: %w", op, from, to, errorsx.ErrInvalidArgument)
	}

	reqFrom := from.UTC().UnixMilli()
	reqTo := to.UTC().UnixMilli()
	sfKey := fmt.Sprintf("%v:%v:%v", candleKey(spec, interval), reqFrom, reqTo)

	v, err, _ := a.candlesSF.Do(sfKey, func() (any, error) {
		api, ok := a.exchanges[spec.Exchange]
		if !ok {
			return nil, fmt.Errorf("%s: exchange %s: %w", op, spec.Exchange, errorsx.ErrNotFound)
		}
		return a.loadCandlesRange(ctx, api, spec, interval, from, to)
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return v.([]market.Candle), nil
}

func (a *Application) loadCandlesRange(ctx context.Context, api exchange.PublicClient, spec market.Spec, interval market.Interval, from time.Time, to time.Time) ([]market.Candle, error) {
	const op = "load candles range"
	from = from.UTC()
	to = to.UTC()
	reqFrom := from.UnixMilli()
	reqTo := to.UnixMilli()

	dbCandles, err := a.candleRepo.ListByTime(ctx, spec, interval, from, to)
	if err != nil {
		return nil, fmt.Errorf("%s: list candles: %w", op, err)
	}

	if len(dbCandles) == 0 {
		exCandles, err := api.Candles(ctx, spec, interval, from, to)
		if err != nil {
			return nil, fmt.Errorf("%s: fetch exchange candles: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		if err := a.candleRepo.Upsert(ctx, spec, interval, exCandles); err != nil {
			return nil, fmt.Errorf("%s: upsert candles: %w", op, err)
		}
		return exCandles, nil
	}

	dbFrom := dbCandles[0].Time
	dbLast := dbCandles[len(dbCandles)-1].Time
	step := intervalMs(interval)
	rightStart := dbLast + step

	var left []market.Candle
	var right []market.Candle

	if reqFrom < dbFrom {
		leftTo := time.UnixMilli(dbFrom).UTC()
		leftFrom := from
		exLeft, err := api.Candles(ctx, spec, interval, leftFrom, leftTo)
		if err != nil {
			return nil, fmt.Errorf("%s: fetch left range: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		if err := a.candleRepo.Upsert(ctx, spec, interval, exLeft); err != nil {
			return nil, fmt.Errorf("%s: upsert left range: %w", op, err)
		}
		left = exLeft
	}

	if rightStart < reqTo {
		rightFrom := time.UnixMilli(rightStart).UTC()
		rightTo := to
		exRight, err := api.Candles(ctx, spec, interval, rightFrom, rightTo)
		if err != nil {
			return nil, fmt.Errorf("%s: fetch right range: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		if err := a.candleRepo.Upsert(ctx, spec, interval, exRight); err != nil {
			return nil, fmt.Errorf("%s: upsert right range: %w", op, err)
		}
		right = exRight
	}

	total := len(left) + len(dbCandles) + len(right)
	merged := make([]market.Candle, 0, total)
	merged = append(merged, left...)
	merged = append(merged, dbCandles...)
	merged = append(merged, right...)

	slice, ok := sliceCandles(merged, reqFrom, reqTo)
	if !ok {
		return nil, fmt.Errorf("%s: requested range missing: %w", op, errorsx.ErrInternal)
	}

	return slice, nil
}

func candleKey(spec market.Spec, interval market.Interval) string {
	return fmt.Sprintf(
		"%s:%s:%s:%s",
		spec.Exchange,
		spec.Category,
		spec.Symbol,
		interval.String(),
	)
}

func intervalMs(i market.Interval) int64 {
	return int64(time.Duration(i) / time.Millisecond)
}

func sliceCandles(data []market.Candle, from, to int64) ([]market.Candle, bool) {
	if len(data) == 0 || from >= to {
		return nil, false
	}

	i := sort.Search(len(data), func(k int) bool { return data[k].Time >= from })
	j := sort.Search(len(data), func(k int) bool { return data[k].Time >= to })

	if i < 0 || j < 0 || i >= j || i >= len(data) {
		return nil, false
	}
	return data[i:j], true
}
