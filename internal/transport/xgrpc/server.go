package xgrpc

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/pulsoats/analysis/internal/transport/xgrpc/interceptor"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	systempb "github.com/pulsoats/contracts/gen/go/system/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func RunGRPCServer(ctx context.Context, addr string, analysisSrv analysispb.AnalysisServer, catalogSrv catalogpb.CatalogServer, ServiceMonitorSrv systempb.ServiceMonitorServer, log *slog.Logger, secret string) error {
	if log == nil {
		log = slog.Default()
	}
	log = log.With("component", "grpc.server")

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptor.ServiceTokenUnaryInterceptor(secret),
			interceptor.UserIDInterceptor(),
			interceptor.UnaryLogger(log),
			interceptor.UnaryError(log),
		),
		grpc.ChainStreamInterceptor(
			interceptor.ServiceTokenStreamInterceptor(secret),
			interceptor.StreamError(log)),
	)
	reflection.Register(server)

	analysispb.RegisterAnalysisServer(server, analysisSrv)
	catalogpb.RegisterCatalogServer(server, catalogSrv)
	systempb.RegisterServiceMonitorServer(server, ServiceMonitorSrv)

	if ctx != nil {
		go func() {
			<-ctx.Done()
			server.GracefulStop()
		}()
	}

	err = server.Serve(lis)
	if errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}
