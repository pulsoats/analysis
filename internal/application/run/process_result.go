package run

import (
	"context"
	"errors"
	"fmt"
	"time"

	detsvc "github.com/pulsoats/analysis/internal/detect"
	"github.com/pulsoats/analysis/internal/domain"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/exchange"
	"github.com/pulsoats/core/domain/market"
)

const minTF = market.Interval1m

type processRunRequest struct {
	signals   []domain.AnalysisSignal
	candles   []market.Candle
	detector  detect.CandleDetector
	exApi     exchange.API
	market    market.Spec
	interval  market.Interval
	priceType market.PriceType
	fees      market.TakerMakerFees
}

type processRunResult struct {
	signals      []domain.AnalysisSignal
	avgProfitPPM int64
}

func (s *Service) processResult(ctx context.Context, req processRunRequest) (processRunResult, error) {
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

	res := make([]domain.AnalysisSignal, 0, len(req.signals))

	for _, sig := range req.signals {
		signalIdx := sig.Index
		startAbs := signalIdx + 1
		if startAbs >= len(req.candles) {
			continue
		}

		from := time.UnixMilli(req.candles[startAbs].Time)
		windowLen := barsForBuy + barsForSell
		to := from.Add(time.Duration(windowLen) * time.Duration(minTF))

		tradeWindow, err := s.fetchCandles(ctx, market.CandleSpec{Spec: req.market, Interval: minTF}, from, to, req.priceType)
		if err != nil {
			return processRunResult{}, fmt.Errorf("process run result: fetch candles: %w", err)
		}

		resp, err := s.detectSvc.SignalStatus(detsvc.SignalStatusRequest{
			BarsForTrade: tradeWindow,
			Signal:       sig.Signal,
			BarsForBuy:   barsForBuy,
			BarsForSell:  barsForSell,
			Fees:         req.fees,
		})
		if err != nil {
			// если по сигналу не получилось купить — дроп
			if errors.Is(err, detsvc.ErrNoBuy) || errors.Is(err, detsvc.ErrNoData) {
				continue
			}
			return processRunResult{}, fmt.Errorf("process run result: signal status: %w", err)
		}

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
		avgProfitPPM: avg,
	}, nil
}
