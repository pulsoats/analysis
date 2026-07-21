package analysis

import (
	"context"
	"fmt"
	"log/slog"

	"errors"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/application/run"
	"github.com/pulsoats/analysis/internal/transport/xgrpc/interceptor"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/errorsx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	analysispb.UnimplementedAnalysisServer
	app *run.Application
	log *slog.Logger
}

func NewServer(app *run.Application, logger *slog.Logger) (*Server, error) {
	if app == nil {
		return nil, errors.New("grpc server: run app is nil")
	}
	s := &Server{app: app}
	s.log = slog.Default()
	if logger != nil {
		s.log = logger
	}
	s.log = s.log.With("component", "grpc.analysis")
	return s, nil
}

func (s *Server) NewRun(ctx context.Context, req *analysispb.NewRunRequest) (*analysispb.Run, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "missing user-id in ctx")
	}

	runCfg, err := newRunRequestFromProto(req)
	if err != nil {
		return nil, err
	}
	runCfg.UserID = userID

	r, err := s.app.NewRun(ctx, runCfg)
	if err != nil {
		return nil, err
	}

	return runToProto(r), nil
}

// GetRun — unary. Ошибки логирует interceptor.
// Warn только для NotFound — добавляем run_id, которого у interceptor нет.
func (s *Server) GetRun(ctx context.Context, req *corepb.RunID) (*analysispb.Run, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	runID, err := parseRunID(req.GetRunId())
	if err != nil {
		return nil, fmt.Errorf("run_id: %w", errors.Join(errorsx.ErrInvalidArgument, err))
	}

	r, err := s.app.RunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, errorsx.ErrNotFound) {
			s.log.Warn("run not found", "grpc_method", "GetRun", "run_id", runID)
		}
		return nil, err
	}

	return runToProto(r), nil
}

// GetRunArchive — streaming. Конвертацию ошибок делает StreamError interceptor.
func (s *Server) GetRunArchive(req *corepb.RunID, stream analysispb.Analysis_GetRunArchiveServer) error {
	log := s.log.With("grpc_method", "GetRunArchive")

	if req == nil {
		return status.Error(codes.InvalidArgument, "nil request")
	}

	runID, err := parseRunID(req.GetRunId())
	if err != nil {
		return fmt.Errorf("run_id: %w", errors.Join(errorsx.ErrInvalidArgument, err))
	}
	log = log.With("run_id", runID)

	w := &grpcWriter{stream: stream}
	if err := s.app.StreamRunArchive(stream.Context(), runID, w); err != nil {
		if errors.Is(err, errorsx.ErrNotFound) {
			log.Warn("run not found")
		} else if errors.Is(err, errorsx.ErrInvalidArgument) {
			log.Warn("run result not ready")
			return status.Error(codes.FailedPrecondition, "run result not ready")
		} else {
			log.Error("failed to stream run archive", "err", err)
		}
		return err
	}

	log.Debug("run archive stream completed")
	return nil
}

func (s *Server) ListRunsPaged(ctx context.Context, req *analysispb.ListRunsPagedRequest) (*analysispb.ListRunsPagedResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "missing user-id in ctx")
	}

	reqFromProto, err := listRunsPagedRequestFromProto(req)
	if err != nil {
		return nil, err
	}

	reqFromProto.UserID = userID

	runsResp, err := s.app.RunsPaged(ctx, reqFromProto)
	if err != nil {
		return nil, err
	}

	return listRunsPagedResponseToProto(runsResp), nil
}

func (s *Server) ShareRun(ctx context.Context, req *corepb.RunID) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	runID, err := parseRunID(req.GetRunId())
	if err != nil {
		s.log.Warn("invalid run_id", "grpc_method", "ShareRun", "run_id", req.GetRunId())
		return nil, status.Error(codes.InvalidArgument, "invalid run_id")
	}

	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "missing user-id in ctx")
	}

	if err := s.app.ShareRun(ctx, runID, userID); err != nil {
		if errors.Is(err, errorsx.ErrNotFound) {
			s.log.Warn("run not found or not owned", "grpc_method", "ShareRun", "run_id", runID)
		}
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) DeleteRun(ctx context.Context, req *corepb.RunID) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "nil request")
	}

	runID, err := parseRunID(req.GetRunId())
	if err != nil {
		s.log.Warn("invalid run_id", "grpc_method", "DeleteRun", "run_id", req.GetRunId())
		return nil, status.Error(codes.InvalidArgument, "invalid run_id")
	}

	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "missing user-id in ctx")
	}

	if err := s.app.DeleteRun(ctx, runID, userID); err != nil {
		if errors.Is(err, errorsx.ErrNotFound) {
			s.log.Warn("run not found or not owned", "grpc_method", "DeleteRun", "run_id", runID)
		}
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func parseRunID(runID string) (uuid.UUID, error) {
	if runID == "" {
		return uuid.UUID{}, fmt.Errorf("run_id is empty")
	}
	id, err := uuid.Parse(runID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("run_id must be a valid UUID: %w", err)
	}
	return id, nil
}
