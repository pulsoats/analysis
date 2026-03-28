package grpc

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/pulsoats/analysis/internal/model/run"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	commonpb "github.com/pulsoats/contracts/gen/go/common/v1"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapStartRunRequest(req *analysispb.StartRunRequest) (run.Request, error) {
	if req == nil {
		return run.Request{}, errors.New("nil request")
	}
	if req.Market == nil {
		return run.Request{}, errors.New("market is required")
	}

	interval, ok := market.ParseInterval(req.Interval)
	if !ok {
		return run.Request{}, fmt.Errorf("interval %v: %w", req.Interval, errorsx.ErrInvalidArgument)
	}

	category := market.Category(req.Market.Category)

	detector, err := mapDetectorsRequest(req.Detector)
	if err != nil {
		return run.Request{}, err
	}

	return run.Request{
		UserID: req.UserId,
		Market: market.Spec{
			Exchange: req.Market.Exchange,
			Category: category,
			Symbol:   req.Market.Symbol,
		},
		Interval:  interval,
		From:      req.From.AsTime(),
		To:        req.To.AsTime(),
		PriceType: market.PriceType(req.PriceType),
		Detector:  detector,
		Fees:      mapFeesRequest(req.Fees),
	}, nil
}

func mapDetectorsRequest(rawDetector *commonpb.DetectorConfig) (detect.DetectorConfig, error) {
	if rawDetector.Code == "" {
		return detect.DetectorConfig{}, fmt.Errorf("detector code: %w", errorsx.ErrRequired)
	}

	return detect.DetectorConfig{
		Code:  rawDetector.Code,
		Label: rawDetector.Label,
		Opts:  rawDetector.Opts,
	}, nil
}

func mapDetectorsResponse(det detect.DetectorConfig) *commonpb.DetectorConfig {
	return &commonpb.DetectorConfig{
		Code:  det.Code,
		Label: det.Label,
		Opts:  det.Opts,
	}
}

func mapFeesRequest(f *commonpb.Fees) *market.TakerMakerFees {
	if f == nil {
		return nil
	}
	return &market.TakerMakerFees{
		TakerFeeRate: f.TakerFee,
		MakerFeeRate: f.MakerFee,
	}
}

func mapRunStatusCode(code int) analysispb.RunStatusCode {
	switch code {
	case run.StatusPending:
		return analysispb.RunStatusCode_RUN_STATUS_PENDING
	case run.StatusRunning:
		return analysispb.RunStatusCode_RUN_STATUS_RUNNING
	case run.StatusDone:
		return analysispb.RunStatusCode_RUN_STATUS_DONE
	case run.StatusFailed:
		return analysispb.RunStatusCode_RUN_STATUS_FAILED
	default:
		return analysispb.RunStatusCode_RUN_STATUS_UNSPECIFIED
	}
}

func mapRunMeta(r run.Run) *analysispb.RunMeta {
	meta := &analysispb.RunMeta{
		Id: strconv.FormatInt(r.ID, 10),
		Status: &analysispb.Status{
			Code:    mapRunStatusCode(r.Status.Code),
			Message: r.Status.Message,
		},
		Market:   mapMarketSpec(r.Market),
		Interval: r.Interval.String(),
		Detector: mapDetectorsResponse(r.Detector),
		SignalsCount: func() int64 {
			if r.SignalsCount == nil {
				return 0
			}
			return *r.SignalsCount
		}(),
		AvgProfitPpm: func() int64 {
			if r.AvgProfitPPM == nil {
				return 0
			}
			return *r.AvgProfitPPM
		}(),
		CreatedBy: r.CreatedBy,
	}
	if r.From != nil {
		meta.From = timestamppb.New(*r.From)
	}
	if r.To != nil {
		meta.To = timestamppb.New(*r.To)
	}
	meta.IsShared = r.IsShared
	if r.SharedAt != nil {
		meta.SharedAt = timestamppb.New(*r.SharedAt)
	}
	return meta
}

func mapRunFilter(f analysispb.RunFilter) run.RunFilter {
	switch f {
	case analysispb.RunFilter_RUN_FILTER_MINE:
		return run.RunFilterMine
	case analysispb.RunFilter_RUN_FILTER_SHARED:
		return run.RunFilterShared
	case analysispb.RunFilter_RUN_FILTER_ALL:
		return run.RunFilterAll
	default:
		return run.RunFilterUnspecified
	}
}

func mapMarketSpec(spec market.Spec) *commonpb.MarketSpec {
	return &commonpb.MarketSpec{
		Exchange: spec.Exchange,
		Category: string(spec.Category),
		Symbol:   spec.Symbol,
	}
}
