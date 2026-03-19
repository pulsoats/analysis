package grpc

import (
	"context"
	"strconv"

	run2 "github.com/pulsoats/analysis/internal/model/run"
	"github.com/pulsoats/analysis/internal/transport/grpc/errorx"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AnalysisServer struct {
	analysispb.UnimplementedAnalysisServiceServer
	runUC run2.Service
}

func NewAnalysisServer(runUC run2.Service) *AnalysisServer {
	return &AnalysisServer{runUC: runUC}
}

func (s *AnalysisServer) StartRun(ctx context.Context, req *analysispb.StartRunRequest) (*analysispb.StartRunResponse, error) {
	logger := grpcLogger("StartRunExchange")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	logger.Info().
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

func (s *AnalysisServer) GetRunStatus(ctx context.Context, req *analysispb.GetRunRequest) (*analysispb.GetRunStatusResponse, error) {
	logger := grpcLogger("GetRunStatus")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	logger = logger.With().Str("run_id", req.GetRunId()).Logger()

	runID, err := strconv.ParseInt(req.RunId, 10, 64)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid run id")
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	logger = logger.With().Str("run_id", req.RunId).Logger()

	runStatus, err := s.runUC.Status(ctx, runID)
	if err != nil {
		st := errorx.ToStatus(err)
		if st.Code() == codes.NotFound {
			logger.Warn().Err(err).Msg("run not found")
		} else {
			logger.Error().Err(err).Msg("failed to fetch run status")
		}
		return nil, st.Err()
	}

	logger.Info().
		Int("status", runStatus.Code).
		Msg("returning run status")

	return &analysispb.GetRunStatusResponse{
		Status:  mapRunStatusCode(runStatus.Code),
		Message: runStatus.Message,
	}, nil
}

func (s *AnalysisServer) GetRunMeta(ctx context.Context, req *analysispb.GetRunRequest) (*analysispb.RunMeta, error) {
	logger := grpcLogger("GetRunMeta")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	logger = logger.With().Str("run_id", req.GetRunId()).Logger()
	runID, err := strconv.ParseInt(req.RunId, 10, 64)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid run id")
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	logger = logger.With().Str("run_id", req.RunId).Logger()

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

	logger.Info().
		Str("run_id", req.GetRunId()).
		Msg("returning run meta")

	return mapRunMeta(meta), nil
}

func (s *AnalysisServer) GetRunResult(req *analysispb.GetRunRequest, stream analysispb.AnalysisService_GetRunResultServer) error {
	logger := grpcLogger("GetRunResult")
	if req == nil {
		logger.Warn().Msg("nil request received")
		return status.Error(codes.InvalidArgument, "nil request")
	}

	logger = logger.With().Str("run_id", req.GetRunId()).Logger()

	runID, err := strconv.ParseInt(req.RunId, 10, 64)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid run id")
		return status.Errorf(codes.InvalidArgument, "%v", err)
	}
	logger = logger.With().Str("run_id", req.RunId).Logger()

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
	logger.Info().
		Int("status", runStatus.Code).
		Msg("fetched run status for result streaming")

	if runStatus.Code != run2.StatusDone {
		logger.Warn().Msg("run result not ready")
		return status.Error(codes.FailedPrecondition, "run result not ready")
	}

	w := &grpcWriter{stream: stream}
	if err := s.runUC.StreamRunResult(stream.Context(), runID, w); err != nil {
		logger.Error().Err(err).Msg("failed to stream run archive")
		return status.Errorf(codes.Internal, "%v", err)
	}

	logger.Info().Msg("run result stream completed")
	return nil
}

func grpcLogger(method string) zerolog.Logger {
	return log.With().
		Str("grpc_method", method).
		Logger()
}
