package run

import (
	"errors"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/lib/units"
	"github.com/pulsoats/core/market"
)

var (
	ErrNoBuy  = errors.New("no buy value at BarsForBuy")
	ErrNoData = errors.New("no candles after signal")
)

type signalStatusRequest struct {
	barsForTrade    []market.Candle // окно после сигнала (свеча сигнала не входит)
	signal          detect.Signal
	barsForBuy      int
	barsForSell     int
	fees            market.TakerMakerFees
	disableStopLoss bool
}

type signalStatusResponse struct {
	SignalStatus      string
	ExpectedReturnPPM int64
	BuyTime           int64
	SellTime          int64
}

func signalStatus(req signalStatusRequest) (signalStatusResponse, error) {
	var resp signalStatusResponse

	if len(req.barsForTrade) == 0 {
		return signalStatusResponse{}, ErrNoData
	}

	buyIndex := -1
	limitBuy := min(req.barsForBuy, len(req.barsForTrade))

	for i := 0; i < limitBuy; i++ {
		c := req.barsForTrade[i]
		if c.Low <= req.signal.BuyValue && c.High >= req.signal.BuyValue {
			buyIndex = i
			resp.BuyTime = c.Time
			break
		}
	}

	if buyIndex == -1 {
		return signalStatusResponse{}, ErrNoBuy
	}

	end := buyIndex + req.barsForSell
	if end > len(req.barsForTrade)-1 {
		end = len(req.barsForTrade) - 1
	}
	if end < buyIndex {
		end = buyIndex
	}

	hitTP, hitSL := false, false
	for i := buyIndex; i <= end; i++ {
		c := req.barsForTrade[i]
		curSL := req.signal.StopLossValue > 0 && c.Low <= req.signal.StopLossValue
		curTP := c.High >= req.signal.TakeProfitValue

		if !req.disableStopLoss && curSL {
			hitSL = true
			resp.SellTime = c.Time
			break
		}
		if curTP {
			hitTP = true
			resp.SellTime = c.Time
			break
		}
	}

	switch {
	case hitTP:
		resp.SignalStatus = "PROFIT"
	case hitSL:
		resp.SignalStatus = "LOSS"
	default:
		resp.SignalStatus = "UNKNOWN"
		resp.SellTime = req.barsForTrade[end].Time
	}

	last := req.barsForTrade[end].Close
	resp.ExpectedReturnPPM = calcExpectedReturnPPM(req.signal, resp.SignalStatus, last, req.fees)

	return resp, nil
}

func calcExpectedReturnPPM(sig detect.Signal, status string, lastBFSPointValue int64, fees market.TakerMakerFees) int64 {
	buyWithSpread := sig.BuyValue + (sig.BuyValue*(market.SpreadPPM/2))/units.PPM
	buyWithFee := buyWithSpread + (buyWithSpread*fees.TakerFeeRate)/units.PPM

	var exitRaw int64
	switch status {
	case "PROFIT":
		exitRaw = sig.TakeProfitValue
	case "LOSS":
		exitRaw = sig.StopLossValue
	default:
		exitRaw = lastBFSPointValue
	}

	exitWithSpread := exitRaw - (exitRaw*(market.SpreadPPM/2))/units.PPM
	exitAfterFee := exitWithSpread - (exitWithSpread*fees.MakerFeeRate)/units.PPM
	pnl := exitAfterFee - buyWithFee

	return (pnl * units.PPM) / buyWithFee
}
