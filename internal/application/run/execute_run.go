package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/analysis/internal/domain/signal"
	"github.com/pulsoats/analysis/internal/utils/files"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
	corerun "github.com/pulsoats/core/run"
)

func (a *Application) executeRun(r run.Run, detector detector.Detector, collectRejectLog bool) {
	Logger := a.log.With(
		"op", "execute_run",
		"run_id", r.ID,
	)
	ctx := context.Background()

	var rejectLog []string

	fail := func(err error) {
		if errors.Is(err, errorsx.ErrInternal) {
			Logger.Error("run failed", "err", err)
		} else {
			Logger.Warn("run failed", "err", err)
		}
		var msg string
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			msg = unwrapped.Error()
		}
		r.Status = corerun.Status{Code: corerun.StatusCodeFailed, Message: msg}
		err = a.runRepo.UpdateRun(ctx, r)
		if err != nil {
			Logger.Error(err.Error())
		}
	}

	r.Status = corerun.StatusRunning
	err := a.runRepo.UpdateRun(ctx, r)
	if err != nil {
		Logger.Error(err.Error())
	}

	candles, err := a.fetchCandles(ctx, r.Market, r.Interval, r.FirstCandleTime, r.LastCandleTime)
	if err != nil {
		fail(err)
		return
	}

	signals := make([]signal.AnalysisSignal, 0, 128)
	ws := detector.WindowSize()
	for i := ws - 1; i < len(candles); i++ {
		window := candles[i-ws+1 : i+1]
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}

		detRes := detector.Detect(window, r.Fees)
		if detRes.Signal == nil {
			if collectRejectLog {
				logStr := fmt.Sprintf("detector: %s: opts_label: %s: candle_time: %s reason:%s",
					detector.Code(),
					detector.OptsLabel(),
					time.UnixMilli(candles[i].Time).Format(time.RFC3339),
					detRes.RejectReason,
				)
				rejectLog = append(rejectLog, logStr)
			}
			continue
		}
		signals = append(signals, signal.AnalysisSignal{Signal: *detRes.Signal, Index: i})
	}
	Logger.Debug("signals detected", "count", len(signals))

	res, err := a.processResult(ctx, processRunRequest{
		run:      r,
		signals:  signals,
		candles:  candles,
		detector: detector,
	})
	if err != nil {
		fail(fmt.Errorf("process result: %w", err))
		return
	}

	fromBound, toBound := timeBounds(candles)
	signalsCount := int64(len(res.signals))
	r.FirstCandleTime = fromBound
	r.LastCandleTime = toBound
	r.SignalsCount = signalsCount
	r.SumProfitPPM = res.sumProfitPPM
	r.AvgProfitPPM = res.avgProfitPPM

	zipPath := a.runZipPath(r.ID)
	if err := files.BuildZipResult(ctx, files.BuildZipRequest{
		ZipPath:   zipPath,
		Run:       r,
		Candles:   candles,
		Signals:   res.signals,
		RejectLog: rejectLog,
	}); err != nil {
		fail(fmt.Errorf("build archive: %w", err))
		return
	}

	r.Status = corerun.StatusDone
	if err := a.runRepo.UpdateRun(ctx, r); err != nil {
		fail(err)
		return
	}

	Logger.Info("run completed")
}

func timeBounds(candles []market.Candle) (time.Time, time.Time) {
	if len(candles) == 0 {
		return time.Time{}, time.Time{}
	}
	minTS := candles[0].Time
	maxTS := candles[0].Time
	for i := 1; i < len(candles); i++ {
		if candles[i].Time < minTS {
			minTS = candles[i].Time
		}
		if candles[i].Time > maxTS {
			maxTS = candles[i].Time
		}
	}
	return time.UnixMilli(minTS).UTC(), time.UnixMilli(maxTS).UTC()
}
