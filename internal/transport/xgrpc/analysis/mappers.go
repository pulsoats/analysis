package analysis

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/domain/run"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/xgrpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newRunRequestFromProto(req *analysispb.NewRunRequest) (run.NewRunRequest, error) {
	const op = "new run request from proto"
	if req == nil {
		return run.NewRunRequest{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInvalidArgument)
	}
	if req.Market == nil {
		return run.NewRunRequest{}, fmt.Errorf("%s: market is nil: %w", op, errorsx.ErrInvalidArgument)
	}
	if req.From == nil || req.To == nil {
		return run.NewRunRequest{}, fmt.Errorf("%s: from/to time is nil: %w", op, errorsx.ErrInvalidArgument)
	}

	interval, err := market.ParseInterval(req.Interval)
	if err != nil {
		return run.NewRunRequest{}, fmt.Errorf("%s: %w", op, err)
	}

	detectorConfig, err := xgrpc.DetectorConfigFromProto(req.DetectorConfig)
	if err != nil {
		return run.NewRunRequest{}, err
	}

	filtersConfigs := make([]filter.Config, 0, len(req.FiltersConfigs))
	for _, pb := range req.FiltersConfigs {
		f, err := xgrpc.FilterConfigFromProto(pb)
		if err != nil {
			return run.NewRunRequest{}, fmt.Errorf("%s: %w", op, err)
		}
		filtersConfigs = append(filtersConfigs, f)
	}

	return run.NewRunRequest{
		Market: market.Spec{
			Exchange: req.Market.Exchange,
			Category: req.Market.Category,
			Symbol:   req.Market.Symbol,
		},
		Interval:         interval,
		From:             req.From.AsTime(),
		To:               req.To.AsTime(),
		DetectorConfig:   detectorConfig,
		FiltersConfigs:   filtersConfigs,
		Fees:             xgrpc.FeesFromProto(req.Fees),
		DisableStopLoss:  req.DisableStopLoss,
		DisableRepeats:   req.DisableRepeats,
		CollectRejectLog: req.CollectRejectLog,
	}, nil
}

func runToProto(r run.Run) *analysispb.Run {
	runPb := &analysispb.Run{
		BaseRun: &corepb.BaseRun{
			Id: r.ID.String(),
			Status: &corepb.RunStatus{
				Code:    corepb.RunStatusCode(r.Status.Code),
				Message: r.Status.Message,
			},
			Market:         xgrpc.MarketSpecToProto(r.Market),
			Interval:       r.Interval.String(),
			DetectorConfig: xgrpc.DetectorConfigToProto(r.DetectorConfig),
			FiltersConfigs: filtersToProto(r.FiltersConfigs),
			SignalsCount:   r.SignalsCount,
			CreatedBy:      r.CreatedBy,
			CreatedAt:      timestamppb.New(r.CreatedAt),
		},
		SumProfitPpm: r.SumProfitPPM,
		AvgProfitPpm: r.AvgProfitPPM,
		Fees:         xgrpc.FeesToProto(&r.Fees),
	}
	runPb.DisableStopLoss = r.DisableStopLoss
	runPb.DisableRepeats = r.DisableRepeats
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
	const op = "run filter from proto"
	if pb == nil {
		return nil, nil
	}

	if len(pb.Intervals) > 0 {
		for _, i := range pb.Intervals {
			if _, err := market.ParseInterval(i); err != nil {
				return nil, fmt.Errorf("%s: %w", op, err)
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
		DetectorsCodes:  pb.DetectorCodes,
		Statuses:        statuses,
		MinSignals:      pb.MinSignals,
		MaxSignals:      pb.MaxSignals,
		MinAvgProfitPPM: minAvgProfitPPM,
		MaxAvgProfitPPM: maxAvgProfitPPM,
		DisableStopLoss: pb.DisableStopLoss,
		DisableRepeats:  pb.DisableRepeats,
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

	f, err := runFilterFromProto(pb.Filter)
	if err != nil {
		return run.RunsPagedRequest{}, err
	}

	return run.RunsPagedRequest{
		Limit:       int(pb.GetLimit()),
		BeforeID:    beforeID,
		Scope:       scope,
		OrderDirAsc: pb.OrderDirAsc,
		Filter:      f,
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

func filtersToProto(filters []filter.Config) []*corepb.FilterConfig {
	out := make([]*corepb.FilterConfig, 0, len(filters))
	for _, f := range filters {
		out = append(out, xgrpc.FilterConfigToProto(f))
	}
	return out
}
