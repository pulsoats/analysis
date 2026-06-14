package analysis

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/domain/run"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newRunFromRequestPb(req *analysispb.NewRunRequest) (run.NewRunRequest, error) {
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

	detector, err := detectorConfigFromProto(req.DetectorConfig)
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
		Fees:     feesFromProto(req.Fees),
	}, nil
}

func detectorConfigFromProto(rawDetector *corepb.DetectorConfig) (detect.DetectorConfig, error) {
	if rawDetector == nil {
		return detect.DetectorConfig{}, errorsx.ErrInvalidArgument
	}
	if rawDetector.Code == "" {
		return detect.DetectorConfig{}, fmt.Errorf("detector code: %w", errorsx.ErrRequired)
	}

	return detect.DetectorConfig{
		Code:      rawDetector.Code,
		Version:   rawDetector.Version,
		OptsLabel: rawDetector.OptsLabel,
		Opts:      rawDetector.Opts,
	}, nil
}

func detectorConfigToProto(det detect.DetectorConfig) *corepb.DetectorConfig {
	return &corepb.DetectorConfig{
		Code:      det.Code,
		Version:   det.Version,
		OptsLabel: det.OptsLabel,
		Opts:      det.Opts,
	}
}

func feesFromProto(f *corepb.Fees) *market.TakerMakerFees {
	if f == nil {
		return nil
	}
	return &market.TakerMakerFees{
		TakerFeeRate: f.TakerFeePpm,
		MakerFeeRate: f.MakerFeePpm,
	}
}

func feesToProto(f market.TakerMakerFees) *corepb.Fees {
	return &corepb.Fees{
		TakerFeePpm: f.TakerFeeRate,
		MakerFeePpm: f.MakerFeeRate,
	}
}

func runToProto(r run.Run) *analysispb.Run {
	runPb := &analysispb.Run{
		BaseRun: &corepb.BaseRun{
			Id: r.ID.String(),
			Status: &corepb.RunStatus{
				Code:    corepb.RunStatusCode(r.Status.Code),
				Message: r.Status.Message,
			},
			Market:   marketSpecToProto(r.Market),
			Interval: r.Interval.String(),
			Detector: detectorConfigToProto(r.Detector),
			SignalsCount: r.SignalsCount,
			CreatedBy: r.CreatedBy,
			CreatedAt: timestamppb.New(r.CreatedAt),
		},
		SumProfitPpm: func() int64 {
			if r.SumProfitPPM == nil {
				return 0
			}
			return *r.SumProfitPPM
		}(),
		AvgProfitPpm: func() int64 {
			if r.AvgProfitPPM == nil {
				return 0
			}
			return *r.AvgProfitPPM
		}(),
		Fees: feesToProto(r.Fees),
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

func runScopeFromProto(pb analysispb.RunScope) run.Scope {
	switch pb {
	case analysispb.RunScope_RUN_SCOPE_SHARED:
		return run.ScopeShared
	case analysispb.RunScope_RUN_SCOPE_ALL:
		return run.ScopeAll
	default:
		return run.ScopeMine
	}
}

func runFilterFromProto(pb *analysispb.ListRunsFilter) (*run.Filter, error) {
	if pb == nil {
		return nil, nil
	}

	if len(pb.Intervals) > 0 {
		for _, i := range pb.Intervals {
			if _, ok := market.ParseInterval(i); !ok {
				return nil, errorsx.ErrInvalidArgument
			}
		}
	}

	var statuses []int
	if len(pb.Statuses) > 0 {
		for _, s := range pb.Statuses {
			statuses = append(statuses, int(s))
		}
	}

	var minAvgProfitPPM, maxAvgProfitPPM *int64
	if pb.MinAvgProfitPpm != nil {
		v := pb.GetMinAvgProfitPpm()
		minAvgProfitPPM = &v
	}
	if pb.MaxAvgProfitPpm != nil {
		v := pb.GetMaxAvgProfitPpm()
		maxAvgProfitPPM = &v
	}

	var firstCandleFrom, lastCandleTo *time.Time
	if pb.FirstCandleFrom != nil {
		v := pb.FirstCandleFrom.AsTime()
		firstCandleFrom = &v
	}
	if pb.LastCandleTo != nil {
		v := pb.LastCandleTo.AsTime()
		lastCandleTo = &v
	}

	var createdFrom, createdTo *time.Time
	if pb.CreatedFrom != nil {
		v := pb.CreatedFrom.AsTime()
		createdFrom = &v
	}
	if pb.CreatedTo != nil {
		v := pb.CreatedTo.AsTime()
		createdTo = &v
	}

	return &run.Filter{
		Exchanges:       pb.Exchanges,
		Categories:      pb.Categories,
		Symbols:         pb.Symbols,
		Intervals:       pb.Intervals,
		DetectorCodes:   pb.DetectorCodes,
		Statuses:        statuses,
		MinSignals:      pb.MinSignals,
		MaxSignals:      pb.MaxSignals,
		MinAvgProfitPPM: minAvgProfitPPM,
		MaxAvgProfitPPM: maxAvgProfitPPM,
		FirstCandleFrom: firstCandleFrom,
		LastCandleTo:    lastCandleTo,
		CreatedFrom:     createdFrom,
		CreatedTo:       createdTo,
	}, nil
}

func listRunsPagedRequestFromProto(pb *analysispb.ListRunsPagedRequest) (run.RunsPagedRequest, error) {
	var beforeID *uuid.UUID
	if pb.BeforeId != nil {
		id, err := uuid.Parse(*pb.BeforeId)
		if err != nil {
			return run.RunsPagedRequest{}, errorsx.ErrInvalidArgument
		}
		beforeID = &id
	}

	scope := runScopeFromProto(pb.Scope)

	filter, err := runFilterFromProto(pb.Filter)
	if err != nil {
		return run.RunsPagedRequest{}, err
	}

	return run.RunsPagedRequest{
		Limit:       int(pb.GetLimit()),
		BeforeID:    beforeID,
		Scope:       scope,
		OrderDirAsc: pb.OrderDirAsc,
		Filter:      filter,
	}, nil
}

func listRunsPagedResponseToProto(resp run.RunsPagedResponse) *analysispb.ListRunsPagedResponse {
	runs := make([]*analysispb.Run, 0, len(resp.Runs))
	for _, r := range resp.Runs {
		runs = append(runs, runToProto(r))
	}

	var nextBeforeIDStr *string
	if resp.NextBeforeID != nil {
		s := resp.NextBeforeID.String()
		nextBeforeIDStr = &s
	}

	return &analysispb.ListRunsPagedResponse{
		Runs:         runs,
		HasMore:      resp.HasMore,
		NextBeforeId: nextBeforeIDStr,
	}
}

func marketSpecToProto(spec market.Spec) *corepb.MarketSpec {
	return &corepb.MarketSpec{
		Exchange: spec.Exchange,
		Category: spec.Category,
		Symbol:   spec.Symbol,
	}
}
