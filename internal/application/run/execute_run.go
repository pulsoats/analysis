package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/analysis/internal/domain/signal"
	"github.com/pulsoats/analysis/internal/utils/files"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
	corerun "github.com/pulsoats/core/run"
)

func (s *Application) executeRun(r run.Run, detector detect.CandleDetector, fees market.TakerMakerFees) {
	log := s.log.With(
		"op", "execute_run",
		"run_id", r.ID,
	)
	ctx := context.Background()

	fail := func(err error) {
		if errors.Is(err, errorsx.ErrInternal) {
			log.Error("run failed", "err", err)
		} else {
			log.Warn("run failed", "err", err)
		}
		var msg string
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			msg = unwrapped.Error()
		}
		r.Status = corerun.Status{Code: corerun.StatusCodeFailed, Message: msg}
		err = s.runRepo.UpdateRun(ctx, r)
		if err != nil {
			log.Error(err.Error())
		}
	}

	r.Status = corerun.StatusRunning
	err := s.runRepo.UpdateRun(ctx, r)
	if err != nil {
		log.Error(err.Error())
	}

	log.Info("run started")

	candles, err := s.fetchCandles(ctx, r.Market, r.Interval, r.FirstCandleTime, r.LastCandleTime)
	if err != nil {
		fail(err)
		return
	}
	log.Debug("candles fetched", "count", len(candles))

	signals := make([]signal.AnalysisSignal, 0, 128)
	ws := detector.WindowSize()
	for i := ws - 1; i < len(candles); i++ {
		window := candles[i-ws+1 : i+1]
		if err := ctx.Err(); err != nil {
			fail(err)
			return
		}

		sig, ok, err := detector.Detect(ctx, window, fees)
		if err != nil {
			fail(fmt.Errorf("detect: %w", err))
			return
		}
		if !ok {
			continue
		}
		sig.Interval = r.Interval
		signals = append(signals, signal.AnalysisSignal{Signal: sig, Index: i})
	}
	log.Debug("signals detected", "count", len(signals))

	res, err := s.processResult(ctx, processRunRequest{
		signals:  signals,
		candles:  candles,
		detector: detector,
		market:   r.Market,
		interval: r.Interval,
		fees:     fees,
	})
	if err != nil {
		fail(fmt.Errorf("process result: %w", err))
		return
	}

	fromBound, toBound := timeBounds(candles)
	signalsCount := int64(len(res.signals))
	r.FirstCandleTime = fromBound
	r.LastCandleTime = toBound
	r.SignalsCount = &signalsCount
	r.AvgProfitPPM = &res.avgProfitPPM

	zipPath := s.runZipPath(r.ID)
	if err := files.BuildZipResult(ctx, zipPath, r, candles, res.signals); err != nil {
		fail(fmt.Errorf("build archive: %w", err))
		return
	}

	r.Status = corerun.StatusDone
	if err := s.runRepo.UpdateRun(ctx, r); err != nil {
		fail(err)
		return
	}

	log.Info("run completed")
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
