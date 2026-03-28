package grpc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pulsoats/analysis/internal/model/run"
	"github.com/pulsoats/analysis/internal/transport/grpc/errorx"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
}

func NewAnalysisServer(runUC run.Service) *AnalysisServer {
	return &AnalysisServer{runUC: runUC}
}

func (s *AnalysisServer) StartRun(ctx context.Context, req *analysispb.StartRunRequest) (*analysispb.StartRunResponse, error) {
	logger := grpcLogger("StartRunExchange")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	logger.Debug().
		Str("user_id", req.GetUserId()).
		Str("exchange", req.GetMarket().GetExchange()).
		Str("symbol", req.GetMarket().GetSymbol()).
		Msg("start run exchange request received")

	runCfg, err := mapStartRunRequest(req)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid start run request")
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	runID, err := s.runUC.StartRun(ctx, runCfg)
	if err != nil {
		logger.Error().Err(err).Msg("failed to start analysis run")
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	return &analysispb.StartRunResponse{RunId: strconv.FormatInt(runID, 10)}, nil
}

func (s *AnalysisServer) GetRunMeta(ctx context.Context, req *analysispb.GetRunRequest) (*analysispb.RunMeta, error) {
	logger := grpcLogger("GetRunMeta")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	runIDStr := req.GetRunId()
	runID, err := parseRunID(runIDStr)
	if err != nil {
		logger.Warn().
			Str("run_id", runIDStr).
			Err(err).
			Msg("invalid run_id received")
		return nil, status.Error(codes.InvalidArgument, "invalid run_id")
	}
	logger = logger.With().Int64("run_id", runID).Logger()

	meta, err := s.runUC.FindByID(ctx, runID)
	if err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			logger.Warn().Err(err).Msg("run not found")
		} else {
			logger.Error().Err(err).Msg("failed to fetch run meta")
		}
		return nil, st.Err()
	}

	logger.Debug().
		Int64("run_id", runID).
		Msg("returning run meta")

	return mapRunMeta(meta), nil
}

func (s *AnalysisServer) GetRunResult(req *analysispb.GetRunRequest, stream analysispb.Analysis_GetRunResultServer) error {
	logger := grpcLogger("GetRunResult")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return status.Error(codes.InvalidArgument, "nil request")
	}

	runIDStr := req.GetRunId()
	runID, err := parseRunID(runIDStr)
	if err != nil {
		logger.Warn().
			Str("run_id", runIDStr).
			Err(err).
			Msg("invalid run_id received")
		return status.Error(codes.InvalidArgument, "invalid run_id")
	}
	logger = logger.With().Int64("run_id", runID).Logger()

	runStatus, err := s.runUC.Status(stream.Context(), runID)
	if err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			logger.Warn().Err(err).Msg("run not found")
		} else {
			logger.Error().Err(err).Msg("failed to fetch run status")
		}
		return st.Err()
	}
	logger.Debug().
		Int("status", runStatus.Code).
		Msg("fetched run status for result streaming")

	if runStatus.Code != run.StatusDone {
		logger.Warn().Msg("run result not ready")
		return status.Error(codes.FailedPrecondition, "run result not ready")
	}

	w := &grpcWriter{stream: stream}
	if err := s.runUC.StreamRunResult(stream.Context(), runID, w); err != nil {
		logger.Error().Err(err).Msg("failed to stream run archive")
		return status.Errorf(codes.Internal, "%v", err)
	}

	logger.Debug().Msg("run result stream completed")
	return nil
}

func grpcLogger(method string) zerolog.Logger {
	return log.With().
		Str("grpc_method", method).
		Logger()
}

func (s *AnalysisServer) ListRunsPaged(ctx context.Context, req *analysispb.ListRunsRequest) (*analysispb.ListRunsResponse, error) {
	logger := grpcLogger("ListRunsPaged")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	limit := req.GetLimit()
	switch {
	case limit <= 0:
		limit = defaultListRunsLimit
	case limit > maxListRunsLimit:
		limit = maxListRunsLimit
	}
	logger = logger.With().Int32("limit", limit).Logger()

	beforeIDRaw := req.GetBeforeId()
	var beforeID *int64
	if beforeIDRaw < 0 {
		logger.Warn().Int64("before_id", beforeIDRaw).Msg("before_id must be positive")
		return nil, status.Error(codes.InvalidArgument, "before_id must be positive")
	}
	if beforeIDRaw != 0 {
		beforeID = &beforeIDRaw
		logger = logger.With().Int64("before_id", beforeIDRaw).Logger()
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
			logger.Warn().Err(err).Msg("runs page not found")
		} else {
			logger.Error().Err(err).Msg("failed to list runs")
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

	logger.Debug().
		Int("items", len(resp.Items)).
		Bool("has_more", hasMore).
		Int64("next_before_id", resp.GetNextBeforeId()).
		Msg("returning paged runs")
	return resp, nil
}

func (s *AnalysisServer) ShareRun(ctx context.Context, req *analysispb.ShareRunRequest) (*analysispb.ShareRunResponse, error) {
	logger := grpcLogger("ShareRun")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	runID, err := parseRunID(req.GetRunId())
	if err != nil {
		logger.Warn().Str("run_id", req.GetRunId()).Err(err).Msg("invalid run_id")
		return nil, status.Error(codes.InvalidArgument, "invalid run_id")
	}

	callerID, err := callerIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.runUC.ShareRun(ctx, runID, callerID); err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			logger.Warn().Int64("run_id", runID).Err(err).Msg("run not found or not owned by caller")
			return nil, status.Error(codes.NotFound, "run not found or not yours")
		}
		logger.Error().Int64("run_id", runID).Err(err).Msg("failed to share run")
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
