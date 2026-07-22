package analysis

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/domain/run"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newRunRequestFromProto(req *analysispb.NewRunRequest) (run.NewRunRequest, error) {
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

	det, err := detectorConfigFromProto(req.Detector)
	if err != nil {
		return run.NewRunRequest{}, err
	}

	filters, err := filtersFromProto(req.Filters)
	if err != nil {
		return run.NewRunRequest{}, err
	}

	return run.NewRunRequest{
		Market: market.Spec{
			Exchange: req.Market.Exchange,
			Category: req.Market.Category,
			Symbol:   req.Market.Symbol,
		},
		Interval:        interval,
		From:            req.From.AsTime(),
		To:              req.To.AsTime(),
		Detector:        det,
		Filters:         filters,
		Fees:            feesFromProto(req.Fees),
		DisableStopLoss: req.DisableStopLoss,
		DisableRepeats:  req.DisableRepeats,
	}, nil
}

func detectorConfigFromProto(rawDetector *corepb.DetectorConfig) (detector.Config, error) {
	if rawDetector == nil {
		return detector.Config{}, errorsx.ErrInvalidArgument
	}
	if rawDetector.Code == "" {
		return detector.Config{}, fmt.Errorf("detector code: %w", errorsx.ErrRequired)
	}

	return detector.Config{
		Code:      rawDetector.Code,
		Version:   rawDetector.Version,
		OptsLabel: rawDetector.OptsLabel,
		Opts:      rawDetector.Opts,
	}, nil
}

func detectorConfigToProto(det detector.Config) *corepb.DetectorConfig {
	return &corepb.DetectorConfig{
		Code:      det.Code,
		Version:   det.Version,
		OptsLabel: det.OptsLabel,
		Opts:      det.Opts,
	}
}

func filterConfigFromProto(rawFilter *corepb.FilterConfig) (filter.Config, error) {
	if rawFilter == nil {
		return filter.Config{}, errorsx.ErrInvalidArgument
	}
	if rawFilter.Code == "" {
		return filter.Config{}, errorsx.ErrInvalidArgument
	}
	return filter.Config{
		Code:   rawFilter.Code,
		Period: int(rawFilter.Period),
	}, nil
}

func filtersFromProto(pb []*corepb.FilterConfig) ([]filter.Config, error) {
	out := make([]filter.Config, 0, len(pb))
	for _, rawFilter := range pb {
		f, err := filterConfigFromProto(rawFilter)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func filterToProto(f filter.Config) *corepb.FilterConfig {
	return &corepb.FilterConfig{
		Code:   f.Code,
		Period: int32(f.Period),
	}
}

func filtersToProto(filters []filter.Config) []*corepb.FilterConfig {
	out := make([]*corepb.FilterConfig, 0, len(filters))
	for _, f := range filters {
		out = append(out, filterToProto(f))
	}
	return out
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
			Market:       marketSpecToProto(r.Market),
			Interval:     r.Interval.String(),
			Detector:     detectorConfigToProto(r.Detector),
			Filters:      filtersToProto(r.Filters),
			SignalsCount: r.SignalsCount,
			CreatedBy:    r.CreatedBy,
			CreatedAt:    timestamppb.New(r.CreatedAt),
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
