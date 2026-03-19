package files

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/pulsoats/analysis/internal/model"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/market"
	corecsv "github.com/pulsoats/core/lib/csv"
	"github.com/pulsoats/core/lib/errorsx"
	"github.com/pulsoats/core/lib/format"
	"github.com/pulsoats/core/lib/units"
)

func BuildSignalsCSV(ctx context.Context, w io.Writer, runID string, signals []model.AnalysisSignal) error {
	if len(signals) == 0 {
		return nil
	}

	slices.SortFunc(signals, func(a, b model.AnalysisSignal) int {
		return int(a.Time - b.Time)
	})

	sw, err := corecsv.NewWriter[model.AnalysisSignal](
		w,
		func(sig model.AnalysisSignal) []string {
			return encodeAnalysisSignal(runID, sig)
		},
		corecsv.WithHeader([]string{
			"id", "run_id", "status", "detector", "time", "value",
			"buy_value", "buy_time", "tp_value", "sl_value", "sell_time",
			"expected_return_percent", "left_min_time", "left_min_value",
			"max_time", "max_value", "right_min_time", "right_min_value",
		}),
	)
	if err != nil {
		return fmt.Errorf("build signals csv: new writer: %w: %v", errorsx.ErrInternal, err)
	}

	if _, err := sw.WriteAll(ctx, signals); err != nil {
		_ = sw.Close()
		return fmt.Errorf("build signals csv: write: %w: %v", errorsx.ErrInternal, err)
	}

	return sw.Close()
}

func BuildCandlesCSV(ctx context.Context, w io.Writer, candles []market.Candle) error {
	if len(candles) == 0 {
		return fmt.Errorf("build candles csv: %w: no candles to export", derrors.ErrInvalidArgument)
	}

	cw, err := corecsv.NewWriter[market.Candle](
		w,
		encodeCandle,
		corecsv.WithHeader([]string{
			"time", "open", "high", "low", "close", "volume", "turnover", "price_type",
		}),
	)
	if err != nil {
		return fmt.Errorf("build candles csv: new writer: %w: %v", errorsx.ErrInternal, err)
	}

	if _, err := cw.WriteAll(ctx, candles); err != nil {
		_ = cw.Close()
		return fmt.Errorf("build candles csv: write: %w: %v", errorsx.ErrInternal, err)
	}

	return cw.Close()
}

func encodeAnalysisSignal(runID string, sig model.AnalysisSignal) []string {
	timeStr := time.UnixMilli(sig.Time).UTC().Format(time.RFC3339)

	profitability := float64(sig.ExpectedReturnPPM) / 10_000

	return []string{
		sig.ID.String(),
		runID,
		sig.Status,
		sig.Detector,
		timeStr,
		format.FormatCents(sig.Value),
		format.FormatCents(sig.BuyValue),
		time.UnixMilli(sig.BuyTime).UTC().Format(time.RFC3339),
		format.FormatCents(sig.TakeProfitValue),
		format.FormatCents(sig.StopLossValue),
		time.UnixMilli(sig.SellTime).UTC().Format(time.RFC3339),
		strconv.FormatFloat(profitability, 'f', 4, 64) + "%",
		time.UnixMilli(sig.Extremes[0].Time).UTC().Format(time.RFC3339),
		format.FormatCents(sig.Extremes[0].Close),
		time.UnixMilli(sig.Extremes[1].Time).UTC().Format(time.RFC3339),
		format.FormatCents(sig.Extremes[1].Close),
		time.UnixMilli(sig.Extremes[2].Time).UTC().Format(time.RFC3339),
		format.FormatCents(sig.Extremes[2].Close),
	}
}

func encodeCandle(candle market.Candle) []string {
	candleTime := time.UnixMilli(candle.Time).UTC().Format(time.RFC3339)

	return []string{
		candleTime,
		format.FormatCents(candle.Open),
		format.FormatCents(candle.High),
		format.FormatCents(candle.Low),
		format.FormatCents(candle.Close),
		fmt.Sprintf("%2.f", float64(candle.Volume)/float64(units.PPM)),
		fmt.Sprintf("%2.f", candle.Turnover),
		string(candle.PriceType),
	}
}
