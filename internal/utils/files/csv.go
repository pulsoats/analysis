package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"time"

	"github.com/pulsoats/analysis/internal/domain/signal"
	"github.com/pulsoats/core/errorsx"
	corecsv "github.com/pulsoats/core/lib/csv"
	"github.com/pulsoats/core/lib/format"
	"github.com/pulsoats/core/lib/units"
	"github.com/pulsoats/core/market"
)

func BuildSignalsCSV(ctx context.Context, w io.Writer, signals []signal.AnalysisSignal) error {
	const op = "build signals csv"
	if len(signals) == 0 {
		return nil
	}

	slices.SortFunc(signals, func(a, b signal.AnalysisSignal) int {
		return a.CandleTime.Compare(b.CandleTime)
	})

	var metadataColumns []string
	for k, _ := range signals[0].Metadata {
		metadataColumns = append(metadataColumns, k)
	}
	sort.Strings(metadataColumns)

	header := corecsv.CreateHeaders(signal.AnalysisSignal{})
	header = append(header, metadataColumns...)

	sw, err := corecsv.NewWriter[signal.AnalysisSignal](
		w,
		func(sig signal.AnalysisSignal) []string {
			return encodeAnalysisSignal(metadataColumns, sig)
		},
		corecsv.WithHeader(header))
	if err != nil {
		return fmt.Errorf("%s: new writer: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	if _, err := sw.WriteAll(ctx, signals); err != nil {
		_ = sw.Close()
		return fmt.Errorf("%s: write: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return sw.Close()
}

func BuildCandlesCSV(ctx context.Context, w io.Writer, candles []market.Candle) error {
	if len(candles) == 0 {
		return fmt.Errorf("build candles csv: no candles to export: %w", errorsx.ErrInvalidArgument)
	}

	cw, err := corecsv.NewWriter[market.Candle](
		w,
		encodeCandle,
		corecsv.WithHeader(corecsv.CreateHeaders(market.Candle{})),
	)
	if err != nil {
		return fmt.Errorf("build candles csv: new writer: %w", errors.Join(errorsx.ErrInternal, err))
	}

	if _, err = cw.WriteAll(ctx, candles); err != nil {
		_ = cw.Close()
		return fmt.Errorf("build candles csv: write: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return cw.Close()
}

func encodeAnalysisSignal(metadataColumns []string, sig signal.AnalysisSignal) []string {
	var metadataRows []string
	for _, r := range metadataColumns {
		metadataRows = append(metadataRows, sig.Metadata[r])
	}
	rows := corecsv.EncodeSignal(sig.Signal)
	rows = append(rows,
		sig.Status,
		time.UnixMilli(sig.BuyTime).UTC().Format(time.RFC3339),
		time.UnixMilli(sig.SellTime).UTC().Format(time.RFC3339),
	)
	rows = append(rows, metadataRows...)
	return rows
}

func encodeCandle(candle market.Candle) []string {
	candleTime := time.UnixMilli(candle.Time).UTC().Format(time.RFC3339)

	return []string{
		candleTime,
		format.CentsToString(candle.Open),
		format.CentsToString(candle.High),
		format.CentsToString(candle.Low),
		format.CentsToString(candle.Close),
		fmt.Sprintf("%2.f", float64(candle.Volume)/float64(units.PPM)),
		fmt.Sprintf("%2.f", candle.Turnover),
	}
}
