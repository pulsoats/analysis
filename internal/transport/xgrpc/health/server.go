package health

import (
	"context"
	"errors"

	healthpb "github.com/pulsoats/contracts/gen/go/health/v1"
	coreheath "github.com/pulsoats/core/health"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type app interface {
	Info() coreheath.ServiceInfo
	Metrics(ctx context.Context) (coreheath.ServiceMetrics, error)
}

type Server struct {
	healthpb.UnimplementedServiceMonitorServer
	app app
}

func NewServer(app app) (*Server, error) {
	if app == nil {
		return nil, errors.New("health grpc server: app is nil")
	}
	return &Server{app: app}, nil
}

func (s *Server) Info(_ context.Context, _ *emptypb.Empty) (*healthpb.ServiceInfo, error) {
	info := s.app.Info()
	return &healthpb.ServiceInfo{
		Id:       info.ID,
		Name:     info.Name,
		Exchange: info.Exchange,
		Account:  info.Account,
		Version:  info.Version,
	}, nil
}

func (s *Server) Metrics(ctx context.Context, _ *emptypb.Empty) (*healthpb.ServiceMetrics, error) {
	m, err := s.app.Metrics(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "collect metrics: %v", err)
	}
	return &healthpb.ServiceMetrics{
		ServiceId:     m.ServiceID,
		Status:        healthpb.ServiceStatus(m.Status),
		CpuPercent:    m.CpuPercent,
		MemoryPercent: m.MemoryPercent,
		UptimeSeconds: m.UptimeSeconds,
		ReportedAt:    timestamppb.New(m.ReportedAt),
	}, nil
}
