package grpc

import (
	"errors"
	"fmt"
	"strconv"

	run2 "github.com/pulsoats/analysis/internal/model/run"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	commonpb "github.com/pulsoats/contracts/gen/go/common/v1"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapStartRunRequest(req *analysispb.StartRunRequest) (run2.Request, error) {
	if req == nil {
		return run2.Request{}, errors.New("nil request")
	}
	if req.Market == nil {
		return run2.Request{}, errors.New("market is required")
	}

	interval, ok := market.ParseInterval(req.Interval)
	if !ok {
		return run2.Request{}, fmt.Errorf("%w: interval %v", derrors.ErrInvalidArgument, req.Interval)
	}

	category := market.Category(req.Market.Category)

	detector, err := mapDetectorsRequest(req.Detector)
	if err != nil {
		return run2.Request{}, err
	}

	return run2.Request{
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
	case run2.StatusPending:
		return analysispb.RunStatus_RUN_STATUS_PENDING
	case run2.StatusRunning:
		return analysispb.RunStatus_RUN_STATUS_RUNNING
	case run2.StatusDone:
		return analysispb.RunStatus_RUN_STATUS_DONE
	case run2.StatusFailed:
		return analysispb.RunStatus_RUN_STATUS_FAILED
	default:
		return analysispb.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func mapRunMeta(r run2.Run) *analysispb.RunMeta {
	meta := &analysispb.RunMeta{
		RunId:        strconv.FormatInt(r.ID, 10),
		Status:       mapRunStatusCode(r.Status.Code),
		UserId:       r.CreatedBy,
		Market:       mapMarketSpec(r.Market),
		Interval:     r.Interval.String(),
		SignalsCount: r.SignalsCount,
		AvgProfitPpm: r.AvgProfitPPM,
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
