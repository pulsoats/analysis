package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/analysis/internal/domain/signal"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/market"
)

const minTF = market.Interval1m

type processRunRequest struct {
	run      run.Run
	signals  []signal.AnalysisSignal
	candles  []market.Candle
	detector detector.Detector
}

type processRunResult struct {
	signals      []signal.AnalysisSignal
	sumProfitPPM int64
	avgProfitPPM int64
}

func (a *Application) processResult(ctx context.Context, req processRunRequest) (processRunResult, error) {
	var sumProfit int64
	var seen map[string]struct{}
	if req.run.DisableRepeats {
		seen = make(map[string]struct{})
	}

	ratio := int(time.Duration(req.run.Interval) / time.Duration(minTF))
	if ratio <= 0 {
		ratio = 1
	}
	barsForBuy := req.detector.BarsForBuy() * ratio
	barsForSell := req.detector.BarsForSell() * ratio

	res := make([]signal.AnalysisSignal, 0, len(req.signals))

	for _, sig := range req.signals {
		if req.run.DisableRepeats {
			if _, ok := seen[sig.Fingerprint]; ok {
				continue
			}
			seen[sig.Fingerprint] = struct{}{}
		}

		signalIdx := sig.Index
		startAbs := signalIdx + 1
		if startAbs >= len(req.candles) {
			continue
		}

		from := time.UnixMilli(req.candles[startAbs].Time)
		windowLen := barsForBuy + barsForSell
		to := from.Add(time.Duration(windowLen) * time.Duration(minTF))

		tradeWindow, err := a.fetchCandles(ctx, req.run.Market, minTF, from, to)
		if err != nil {
			return processRunResult{}, fmt.Errorf("process run result: fetch candles: %w", err)
		}

		resp, err := signalStatus(signalStatusRequest{
			barsForTrade:    tradeWindow,
			signal:          sig.Signal,
			barsForBuy:      barsForBuy,
			barsForSell:     barsForSell,
			fees:            req.run.Fees,
			disableStopLoss: req.run.DisableStopLoss,
		})
		if err != nil {
			// если по сигналу не получилось купить — дроп
			if errors.Is(err, ErrNoBuy) || errors.Is(err, ErrNoData) {
				continue
			}
			return processRunResult{}, fmt.Errorf("process run result: signal status: %w", err)
		}

		sig.RunID = req.run.ID
		sig.Status = resp.SignalStatus
		sig.Signal.ExpectedReturnPPM = resp.ExpectedReturnPPM
		sig.BuyTime = resp.BuyTime
		sig.SellTime = resp.SellTime
		res = append(res, sig)
		sumProfit += sig.ExpectedReturnPPM
	}

	var avg int64
	if len(res) > 0 {
		avg = sumProfit / int64(len(res))
	}
	return processRunResult{
		signals:      res,
		sumProfitPPM: sumProfit,
		avgProfitPPM: avg,
	}, nil
}
