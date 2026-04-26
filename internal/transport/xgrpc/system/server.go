package system

import (
	"context"
	"errors"

	systempb "github.com/pulsoats/contracts/gen/go/system/v1"
	coresystem "github.com/pulsoats/core/system"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type app interface {
	Info() coresystem.ServiceInfo
	Metrics(ctx context.Context) (coresystem.ServiceMetrics, error)
}

type Server struct {
	systempb.UnimplementedServiceMonitorServer
	app app
}

func NewServer(app app) (*Server, error) {
	if app == nil {
		return nil, errors.New("health grpc server: app is nil")
	}
	return &Server{app: app}, nil
}

func (s *Server) Info(_ context.Context, _ *emptypb.Empty) (*systempb.ServiceInfo, error) {
	info := s.app.Info()
	return &systempb.ServiceInfo{
		Id:       info.ID.String(),
		Kind:     systempb.ServiceKind_SERVICE_KIND_ANALYSIS,
		Name:     info.Name,
		Exchange: info.Exchange,
		Account:  info.Account,
		Version:  info.Version,
	}, nil
}

func (s *Server) Metrics(ctx context.Context, _ *emptypb.Empty) (*systempb.ServiceMetrics, error) {
	m, err := s.app.Metrics(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "collect metrics: %v", err)
	}
	return &systempb.ServiceMetrics{
		ServiceId:     m.ServiceID.String(),
		Status:        systempb.ServiceStatus(m.Status),
		CpuPercent:    m.CpuPercent,
		MemoryPercent: m.MemoryPercent,
		UptimeSeconds: m.UptimeSeconds,
		ReportedAt:    timestamppb.New(m.ReportedAt),
	}, nil
}
