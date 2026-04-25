package files

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/analysis/internal/domain/signal"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
)

func BuildZipResult(ctx context.Context, zipPath string, run run.Run, candles []market.Candle, signals []signal.AnalysisSignal) error {
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return fmt.Errorf("build zip result: make dir: %w", errors.Join(errorsx.ErrInternal, err))
	}

	f, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("build zip result: create file: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	closed := false
	defer func() {
		if !closed {
			_ = zw.Close()
		}
	}()

	candlesEntryName := CandlesFilename(CandlesFileMeta{
		Exchange: run.Market.Exchange,
		Category: run.Market.Category,
		Interval: run.Interval,
		Symbol:   run.Market.Symbol,
		From:     run.FirstCandleTime,
		To:       run.LastCandleTime,
	})

	cw, err := zw.Create(candlesEntryName)
	if err != nil {
		return fmt.Errorf("build zip result: add candles entry: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if err := BuildCandlesCSV(ctx, cw, candles); err != nil {
		return fmt.Errorf("build zip result: candles csv: %w", err)
	}

	signalsEntryName := SignalsFilename(SignalsFileMeta{
		Exchange: run.Market.Exchange,
		Category: run.Market.Category,
		Interval: run.Interval,
		Symbol:   run.Market.Symbol,
		RunID:    run.ID.String(),
	})

	sw, err := zw.Create(signalsEntryName)
	if err != nil {
		return fmt.Errorf("build zip result: add signals entry: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if err := BuildSignalsCSV(ctx, sw, run.ID.String(), signals); err != nil {
		return fmt.Errorf("build zip result: signals csv: %w", err)
	}

	mw, err := zw.Create("meta.txt")
	if err != nil {
		return fmt.Errorf("build zip result: add meta entry: %w", errors.Join(errorsx.ErrInternal, err))
	}

	if _, err := fmt.Fprint(mw, run.String()); err != nil {
		return fmt.Errorf("build zip result: write meta: %w", errors.Join(errorsx.ErrInternal, err))
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("build zip result: close writer: %w", errors.Join(errorsx.ErrInternal, err))
	}
	closed = true

	return nil
}
