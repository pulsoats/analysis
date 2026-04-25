package analysis

import (
	"fmt"

	"github.com/pulsoats/analysis/internal/domain/run"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/units"
	"github.com/pulsoats/core/market"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapToNewRunRequest(req *analysispb.NewRunRequest) (run.NewRunRequest, error) {
	if req == nil {
		return run.NewRunRequest{}, errorsx.ErrInvalidArgument
	}
	if req.Market == nil {
		return run.NewRunRequest{}, errorsx.ErrInvalidArgument
	}
	if req.From == nil || req.To == nil {
		return run.NewRunRequest{}, fmt.Errorf("time range: %w", errorsx.ErrRequired)
	}

	interval, ok := market.ParseInterval(req.Interval)
	if !ok {
		return run.NewRunRequest{}, fmt.Errorf("interval %v: %w", req.Interval, errorsx.ErrInvalidArgument)
	}

	detector, err := mapToDetectorConfig(req.Detector)
	if err != nil {
		return run.NewRunRequest{}, err
	}

	return run.NewRunRequest{
		Market: market.Spec{
			Exchange: req.Market.Exchange,
			Category: req.Market.Category,
			Symbol:   req.Market.Symbol,
		},
		Interval: interval,
		From:     req.From.AsTime(),
		To:       req.To.AsTime(),
		Detector: detector,
		Fees:     mapToFees(req.Fees),
	}, nil
}

func mapToDetectorConfig(rawDetector *corepb.DetectorConfig) (detect.DetectorConfig, error) {
	if rawDetector == nil {
		return detect.DetectorConfig{}, errorsx.ErrInvalidArgument
	}
	if rawDetector.Code == "" {
		return detect.DetectorConfig{}, fmt.Errorf("detector code: %w", errorsx.ErrRequired)
	}

	return detect.DetectorConfig{
		Code:      rawDetector.Code,
		OptsLabel: rawDetector.OptsLabel,
		Opts:      rawDetector.Opts,
	}, nil
}

func mapToDetectorsPb(det detect.DetectorConfig) *corepb.DetectorConfig {
	return &corepb.DetectorConfig{
		Code:      det.Code,
		OptsLabel: det.OptsLabel,
		Opts:      det.Opts,
	}
}

func mapToFees(f *corepb.Fees) *market.TakerMakerFees {
	if f == nil {
		return nil
	}
	return &market.TakerMakerFees{
		TakerFeeRate: f.TakerFee,
		MakerFeeRate: f.MakerFee,
	}
}

func mapToRunPb(r run.Run) *analysispb.Run {
	runPb := &analysispb.Run{
		BaseRun: &corepb.BaseRun{
			Id: r.ID.String(),
			Status: &corepb.RunStatus{
				Code:    corepb.RunStatusCode(r.Status.Code),
				Message: r.Status.Message,
			},
			Market:   mapToMarketSpecPb(r.Market),
			Interval: r.Interval.String(),
			Detector: mapToDetectorsPb(r.Detector),
			SignalsCount: func() int64 {
				if r.SignalsCount == nil {
					return 0
				}
				return *r.SignalsCount
			}(),
			CreatedBy: r.CreatedBy,
			CreatedAt: timestamppb.New(r.CreatedAt),
		},
		AvgProfitPercent: func() float64 {
			if r.AvgProfitPPM == nil {
				return 0
			}

			v := float64(*r.AvgProfitPPM) / float64(units.PPM) * 100
			return v
		}(),
	}
	if !r.FirstCandleTime.IsZero() {
		runPb.BaseRun.FirstCandleTime = timestamppb.New(r.FirstCandleTime)
	}
	if !r.LastCandleTime.IsZero() {
		runPb.BaseRun.LastCandleTime = timestamppb.New(r.LastCandleTime)
	}
	runPb.IsShared = r.IsShared
	if r.SharedAt != nil {
		runPb.SharedAt = timestamppb.New(*r.SharedAt)
	}
	return runPb
}

func mapToRunFilter(f analysispb.RunFilter) run.Filter {
	switch f {
	case analysispb.RunFilter_RUN_FILTER_MINE:
		return run.FilterMine
	case analysispb.RunFilter_RUN_FILTER_SHARED:
		return run.FilterShared
	case analysispb.RunFilter_RUN_FILTER_ALL:
		return run.FilterAll
	default:
		return run.FilterUnspecified
	}
}

func mapToMarketSpecPb(spec market.Spec) *corepb.MarketSpec {
	return &corepb.MarketSpec{
		Exchange: spec.Exchange,
		Category: spec.Category,
		Symbol:   spec.Symbol,
	}
}
