package grpc

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/pulsoats/analysis/internal/model/run"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	commonpb "github.com/pulsoats/contracts/gen/go/common/v1"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
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
		return run.Request{}, fmt.Errorf("%w: interval %v", derrors.ErrInvalidArgument, req.Interval)
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
		return detect.DetectorConfig{}, fmt.Errorf("%w: detector code", derrors.ErrRequired)
	}

	return detect.DetectorConfig{
		Code:  rawDetector.Code,
		Label: rawDetector.Label,
		Opts:  rawDetector.Opts,
	}, nil
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

func mapRunStatusCode(code int) analysispb.RunStatus {
	switch code {
	case run.StatusPending:
		return analysispb.RunStatus_RUN_STATUS_PENDING
	case run.StatusRunning:
		return analysispb.RunStatus_RUN_STATUS_RUNNING
	case run.StatusDone:
		return analysispb.RunStatus_RUN_STATUS_DONE
	case run.StatusFailed:
		return analysispb.RunStatus_RUN_STATUS_FAILED
	default:
		return analysispb.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func mapRunMeta(r run.Run) *analysispb.RunMeta {
	meta := &analysispb.RunMeta{
		Id:           strconv.FormatInt(r.ID, 10),
		Status:       mapRunStatusCode(r.Status.Code),
		UserId:       r.CreatedBy,
		Market:       mapMarketSpec(r.Market),
		Interval:     r.Interval.String(),
		SignalsCount: r.SignalsCount,
		AvgProfitPpm: func() int64 {
			if r.AvgProfitPPM == nil {
				return 0
			}
			return *r.AvgProfitPPM
		}(),
	}
	if !r.From.IsZero() {
		meta.From = timestamppb.New(r.From)
	}
	if !r.To.IsZero() {
		meta.To = timestamppb.New(r.To)
	}
	return meta
}

func mapMarketSpec(spec market.Spec) *commonpb.MarketSpec {
	return &commonpb.MarketSpec{
		Exchange: spec.Exchange,
		Category: string(spec.Category),
		Symbol:   spec.Symbol,
	}
}
