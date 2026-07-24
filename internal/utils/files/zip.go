package files

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/analysis/internal/domain/signal"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
)

type BuildZipRequest struct {
	ZipPath   string
	Run       run.Run
	Candles   []market.Candle
	Signals   []signal.AnalysisSignal
	RejectLog []string
}

func BuildZipResult(ctx context.Context, req BuildZipRequest) error {
	const op = "build zip result"
	if err := os.MkdirAll(filepath.Dir(req.ZipPath), 0o755); err != nil {
		return fmt.Errorf("%s: make dir: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	f, err := os.Create(req.ZipPath)
	if err != nil {
		return fmt.Errorf("%s: create file: %w", op, errors.Join(errorsx.ErrInternal, err))
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
		Exchange: req.Run.Market.Exchange,
		Category: req.Run.Market.Category,
		Interval: req.Run.Interval,
		Symbol:   req.Run.Market.Symbol,
		From:     req.Run.FirstCandleTime,
		To:       req.Run.LastCandleTime,
	})

	cw, err := zw.Create(candlesEntryName)
	if err != nil {
		return fmt.Errorf("%s: add candles entry: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if err := BuildCandlesCSV(ctx, cw, req.Candles); err != nil {
		return fmt.Errorf("%s: candles csv: %w", op, err)
	}

	signalsEntryName := SignalsFilename(SignalsFileMeta{
		Exchange: req.Run.Market.Exchange,
		Category: req.Run.Market.Category,
		Interval: req.Run.Interval,
		Symbol:   req.Run.Market.Symbol,
		RunID:    req.Run.ID.String(),
	})

	sw, err := zw.Create(signalsEntryName)
	if err != nil {
		return fmt.Errorf("%s: add signals entry: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if err := BuildSignalsCSV(ctx, sw, req.Run.ID.String(), req.Run.Market, req.Run.Interval, req.Signals); err != nil {
		return fmt.Errorf("%s: signals csv: %w", err)
	}

	mw, err := zw.Create("meta.txt")
	if err != nil {
		return fmt.Errorf("%s: add meta entry: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	if _, err := fmt.Fprint(mw, req.Run.String()); err != nil {
		return fmt.Errorf("%s: write meta: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	if len(req.RejectLog) > 0 {
		lw, err := zw.Create("rejectlog.txt")
		if err != nil {
			return fmt.Errorf("%s: add reject log entry: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		if _, err = fmt.Fprint(lw, strings.Join(req.RejectLog, "\n")); err != nil {
			return fmt.Errorf("%s: write reject log: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("%s: close writer: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	closed = true

	return nil
}
