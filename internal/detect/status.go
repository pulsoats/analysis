package detect

import (
	"errors"

	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
)

var (
	ErrNoBuy  = errors.New("no buy value at BarsForBuy")
	ErrNoData = errors.New("no candles after signal")
)

type SignalStatusRequest struct {
	BarsForTrade []market.Candle // окно после сигнала (свеча сигнала не входит)
	Signal       detect.Signal
	BarsForBuy   int
	BarsForSell  int
	Fees         market.TakerMakerFees
}

type SignalStatusResponse struct {
	SignalStatus      string
	ExpectedReturnPPM int64
	BuyTime           int64
	SellTime          int64
}

func (svc *Service) SignalStatus(req SignalStatusRequest) (SignalStatusResponse, error) {
	var resp SignalStatusResponse

	if len(req.BarsForTrade) == 0 {
		return SignalStatusResponse{}, ErrNoData
	}

	buyIndex := -1
	limitBuy := min(req.BarsForBuy, len(req.BarsForTrade))

	for i := 0; i < limitBuy; i++ {
		c := req.BarsForTrade[i]
		if c.High >= req.Signal.BuyValue {
			buyIndex = i
			resp.BuyTime = c.Time
			break
		}
	}

	if buyIndex == -1 {
		return SignalStatusResponse{}, ErrNoBuy
	}

	end := buyIndex + req.BarsForSell
	if end > len(req.BarsForTrade)-1 {
		end = len(req.BarsForTrade) - 1
	}
	if end < buyIndex {
		end = buyIndex
	}

	hitTP, hitSL := false, false
	for i := buyIndex; i <= end; i++ {
		c := req.BarsForTrade[i]
		curSL := req.Signal.StopLossValue > 0 && c.Low <= req.Signal.StopLossValue
		curTP := c.High >= req.Signal.TakeProfitValue

		if curSL && curTP {
			hitSL = true
			resp.SellTime = c.Time
			break
		}
		if curSL {
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
		resp.SellTime = req.BarsForTrade[end].Time
	}

	last := req.BarsForTrade[end].Close
	resp.ExpectedReturnPPM = CalcExpectedReturnPPM(req.Signal, resp.SignalStatus, last, req.Fees)

	return resp, nil
}

func CalcExpectedReturnPPM(sig detect.Signal, status string, lastBFSPointValue int64, fees market.TakerMakerFees) int64 {
	buyWithSpread := sig.BuyValue + (sig.BuyValue*(market.SpreadPPM/2))/market.PPM
	buyWithFee := buyWithSpread + (buyWithSpread*fees.TakerFeeRate)/market.PPM

	var exitRaw int64
	switch status {
	case "PROFIT":
		exitRaw = sig.TakeProfitValue
	case "LOSS":
		exitRaw = sig.StopLossValue
	default:
		exitRaw = lastBFSPointValue
	}

	exitWithSpread := exitRaw - (exitRaw*(market.SpreadPPM/2))/market.PPM
	exitAfterFee := exitWithSpread - (exitWithSpread*fees.MakerFeeRate)/market.PPM
	pnl := exitAfterFee - buyWithFee

	return (pnl * market.PPM) / buyWithFee
}
