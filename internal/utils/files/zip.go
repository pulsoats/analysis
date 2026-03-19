package files

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pulsoats/analysis/internal/model"
	"github.com/pulsoats/analysis/internal/model/run"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/lib/errorsx"
)

func BuildZipResult(ctx context.Context, zipPath string, run run.Run, candles []market.Candle, signals []model.AnalysisSignal) error {
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return fmt.Errorf("build zip result: make dir: %w: %v", errorsx.ErrInternal, err)
	}

	f, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("build zip result: create file: %w: %v", errorsx.ErrInternal, err)
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
		From:     run.From,
		To:       run.To,
	})

	cw, err := zw.Create(candlesEntryName)
	if err != nil {
		return fmt.Errorf("build zip result: add candles entry: %w: %v", errorsx.ErrInternal, err)
	}
	if err := BuildCandlesCSV(ctx, cw, candles); err != nil {
		return fmt.Errorf("build zip result: candles csv: %w", err)
	}

	signalsEntryName := SignalsFilename(SignalsFileMeta{
		Exchange: run.Market.Exchange,
		Category: run.Market.Category,
		Interval: run.Interval,
		Symbol:   run.Market.Symbol,
		RunID:    strconv.FormatInt(run.ID, 10),
	})

	sw, err := zw.Create(signalsEntryName)
	if err != nil {
		return fmt.Errorf("build zip result: add signals entry: %w: %v", errorsx.ErrInternal, err)
	}
	if err := BuildSignalsCSV(ctx, sw, strconv.FormatInt(run.ID, 10), signals); err != nil {
		return fmt.Errorf("build zip result: signals csv: %w", err)
	}

	mw, err := zw.Create("meta.txt")
	if err != nil {
		return fmt.Errorf("build zip result: add meta entry: %w: %v", errorsx.ErrInternal, err)
	}

	if _, err := fmt.Fprint(mw, run.String()); err != nil {
		return fmt.Errorf("build zip result: write meta: %w: %v", errorsx.ErrInternal, err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("build zip result: close writer: %w: %v", errorsx.ErrInternal, err)
	}
	closed = true

	return nil
}
