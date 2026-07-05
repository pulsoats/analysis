package run

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/domain/signal"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/market"
)

const minTF = market.Interval1m

type processRunRequest struct {
	runID    uuid.UUID
	signals  []signal.AnalysisSignal
	candles  []market.Candle
	detector detect.CandleDetector
	market   market.Spec
	interval market.Interval
	fees     market.TakerMakerFees
}

type processRunResult struct {
	signals      []signal.AnalysisSignal
	sumProfitPPM int64
	avgProfitPPM int64
}

func (s *Application) processResult(ctx context.Context, req processRunRequest) (processRunResult, error) {
	var (
		sumProfit int64
	)

	// количество свечей в минимальном TF
	ratio := int(time.Duration(req.interval) / time.Duration(minTF))
	if ratio <= 0 {
		ratio = 1
	}
	barsForBuy := req.detector.BarsForBuy() * ratio
	barsForSell := req.detector.BarsForSell() * ratio

	res := make([]signal.AnalysisSignal, 0, len(req.signals))

	for _, sig := range req.signals {
		signalIdx := sig.Index
		startAbs := signalIdx + 1
		if startAbs >= len(req.candles) {
			continue
		}

		from := time.UnixMilli(req.candles[startAbs].Time)
		windowLen := barsForBuy + barsForSell
		to := from.Add(time.Duration(windowLen) * time.Duration(minTF))

		firstWindowCandleTime := sig.CandleTime.
			Add(-time.Duration(req.interval) * time.Duration(req.detector.WindowSize()-1))

		lookBackBars, err := s.fetchCandles(ctx, req.market, req.interval, firstWindowCandleTime.Add(-time.Duration(req.interval)*24), firstWindowCandleTime)
		if err != nil {
			return processRunResult{}, fmt.Errorf("process run result: fetch look back candles: %w", err)
		}

		if !checkLookBackByLow(lookBackBars, sig.StopLossValue) {
			continue
		}

		tradeWindow, err := s.fetchCandles(ctx, req.market, minTF, from, to)
		if err != nil {
			return processRunResult{}, fmt.Errorf("process run result: fetch candles: %w", err)
		}

		resp, err := signalStatus(signalStatusRequest{
			BarsForTrade: tradeWindow,
			Signal:       sig.Signal,
			BarsForBuy:   barsForBuy,
			BarsForSell:  barsForSell,
			Fees:         req.fees,
		})
		if err != nil {
			// если по сигналу не получилось купить — дроп
			if errors.Is(err, ErrNoBuy) || errors.Is(err, ErrNoData) {
				continue
			}
			return processRunResult{}, fmt.Errorf("process run result: signal status: %w", err)
		}

		sig.RunID = req.runID
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

func checkLookBackByLow(lookBackBars []market.Candle, lowestLowInWindow int64) bool {
	lowestLookBackLow := int64(math.MaxInt64)
	for i := 0; i < len(lookBackBars); i++ {
		if lookBackBars[i].Low < lowestLookBackLow {
			lowestLookBackLow = lookBackBars[i].Low
		}
	}

	if lowestLookBackLow > lowestLowInWindow {
		return false
	}

	return true
}
