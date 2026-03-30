package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/pulsoats/analysis/internal/model/run"
	"github.com/pulsoats/analysis/internal/transport/grpc/errorx"
	"github.com/pulsoats/core/lib/logx"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	defaultListRunsLimit = 20
	maxListRunsLimit     = 100
)

type AnalysisServer struct {
	analysispb.UnimplementedAnalysisServer
	runUC run.Service
	log   *slog.Logger
}

type Option func(*AnalysisServer)

func WithLogger(l *slog.Logger) Option {
	return func(s *AnalysisServer) {
		if l == nil {
			l = logx.Discard()
		}
		s.log = l
	}
}

func NewAnalysisServer(runUC run.Service, opts ...Option) *AnalysisServer {
	s := &AnalysisServer{
		runUC: runUC,
		log:   logx.Discard(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.log = s.log.With("component", "grpc.analysis")
	return s
}

func (s *AnalysisServer) StartRun(ctx context.Context, req *analysispb.StartRunRequest) (*analysispb.StartRunResponse, error) {
	log := s.log.With("grpc_method", "StartRun")
	if req == nil {
		log.Warn("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	log.Debug("request received",
		"user_id", req.GetUserId(),
		"exchange", req.GetMarket().GetExchange(),
		"symbol", req.GetMarket().GetSymbol(),
	)

	runCfg, err := mapStartRunRequest(req)
	if err != nil {
		log.Warn("invalid request", "err", err)
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	runID, err := s.runUC.StartRun(ctx, runCfg)
	if err != nil {
		log.Error("failed to start analysis run", "err", err)
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	return &analysispb.StartRunResponse{RunId: strconv.FormatInt(runID, 10)}, nil
}

func (s *AnalysisServer) GetRunMeta(ctx context.Context, req *analysispb.GetRunRequest) (*analysispb.RunMeta, error) {
	log := s.log.With("grpc_method", "GetRunMeta")
	if req == nil {
		log.Warn("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	runIDStr := req.GetRunId()
	runID, err := parseRunID(runIDStr)
	if err != nil {
		log.Warn("invalid run_id", "run_id", runIDStr, "err", err)
		return nil, status.Error(codes.InvalidArgument, "invalid run_id")
	}
	log = log.With("run_id", runID)

	meta, err := s.runUC.FindByID(ctx, runID)
	if err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			log.Warn("run not found", "err", err)
		} else {
			log.Error("failed to fetch run meta", "err", err)
		}
		return nil, st.Err()
	}

	log.Debug("returning run meta")
	return mapRunMeta(meta), nil
}

func (s *AnalysisServer) GetRunResult(req *analysispb.GetRunRequest, stream analysispb.Analysis_GetRunResultServer) error {
	log := s.log.With("grpc_method", "GetRunResult")
	if req == nil {
		log.Warn("nil request received")
		return status.Error(codes.InvalidArgument, "nil request")
	}

	runIDStr := req.GetRunId()
	runID, err := parseRunID(runIDStr)
	if err != nil {
		log.Warn("invalid run_id", "run_id", runIDStr, "err", err)
		return status.Error(codes.InvalidArgument, "invalid run_id")
	}
	log = log.With("run_id", runID)

	runStatus, err := s.runUC.Status(stream.Context(), runID)
	if err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			log.Warn("run not found", "err", err)
		} else {
			log.Error("failed to fetch run status", "err", err)
		}
		return st.Err()
	}
	log.Debug("fetched run status for result streaming", "status", runStatus.Code)

	if runStatus.Code != run.StatusDone {
		log.Warn("run result not ready")
		return status.Error(codes.FailedPrecondition, "run result not ready")
	}

	w := &grpcWriter{stream: stream}
	if err := s.runUC.StreamRunResult(stream.Context(), runID, w); err != nil {
		log.Error("failed to stream run archive", "err", err)
		return status.Errorf(codes.Internal, "%v", err)
	}

	log.Debug("run result stream completed")
	return nil
}

func (s *AnalysisServer) ListRunsPaged(ctx context.Context, req *analysispb.ListRunsRequest) (*analysispb.ListRunsResponse, error) {
	log := s.log.With("grpc_method", "ListRunsPaged")
	if req == nil {
		log.Warn("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	limit := req.GetLimit()
	switch {
	case limit <= 0:
		limit = defaultListRunsLimit
	case limit > maxListRunsLimit:
		limit = maxListRunsLimit
	}
	log = log.With("limit", limit)

	beforeIDRaw := req.GetBeforeId()
	var beforeID *int64
	if beforeIDRaw < 0 {
		log.Warn("before_id must be positive", "before_id", beforeIDRaw)
		return nil, status.Error(codes.InvalidArgument, "before_id must be positive")
	}
	if beforeIDRaw != 0 {
		beforeID = &beforeIDRaw
		log = log.With("before_id", beforeIDRaw)
	}

	callerID, err := callerIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	filter := mapRunFilter(req.GetFilter())

	runs, hasMore, nextBeforeID, err := s.runUC.ListRunsPaged(ctx, int(limit), beforeID, callerID, filter)
	if err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			log.Warn("runs page not found", "err", err)
		} else {
			log.Error("failed to list runs", "err", err)
		}
		return nil, st.Err()
	}

	resp := &analysispb.ListRunsResponse{
		Items:   make([]*analysispb.RunMeta, 0, len(runs)),
		HasMore: hasMore,
	}
	for _, r := range runs {
		resp.Items = append(resp.Items, mapRunMeta(r))
	}
	if nextBeforeID != nil {
		resp.NextBeforeId = *nextBeforeID
	}

	log.Debug("returning paged runs",
		"items", len(resp.Items),
		"has_more", hasMore,
		"next_before_id", resp.GetNextBeforeId(),
	)
	return resp, nil
}

func (s *AnalysisServer) ShareRun(ctx context.Context, req *analysispb.ShareRunRequest) (*analysispb.ShareRunResponse, error) {
	log := s.log.With("grpc_method", "ShareRun")
	if req == nil {
		log.Warn("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	runID, err := parseRunID(req.GetRunId())
	if err != nil {
		log.Warn("invalid run_id", "run_id", req.GetRunId(), "err", err)
		return nil, status.Error(codes.InvalidArgument, "invalid run_id")
	}

	callerID, err := callerIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.runUC.ShareRun(ctx, runID, callerID); err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			log.Warn("run not found or not owned by caller", "run_id", runID, "err", err)
			return nil, status.Error(codes.NotFound, "run not found or not yours")
		}
		log.Error("failed to share run", "run_id", runID, "err", err)
		return nil, st.Err()
	}

	return &analysispb.ShareRunResponse{Success: true}, nil
}

func parseRunID(runID string) (int64, error) {
	if runID == "" {
		return 0, fmt.Errorf("run_id is empty")
	}
	parsed, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("run_id must be a numeric string: %w", err)
	}
	return parsed, nil
}

func callerIDFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	vals := md.Get("user-id")
	if len(vals) == 0 || vals[0] == "" {
		return "", status.Error(codes.Unauthenticated, "missing user-id in metadata")
	}
	return vals[0], nil
}
