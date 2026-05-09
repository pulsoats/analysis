package xgrpc

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"

	"github.com/pulsoats/analysis/internal/transport/xgrpc/interceptor"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	systempb "github.com/pulsoats/contracts/gen/go/system/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type Config struct {
	Addr                 string
	AnalysisServer       analysispb.AnalysisServer
	CatalogServer        catalogpb.CatalogServer
	ServiceMonitorServer systempb.ServiceMonitorServer
	Logger               *slog.Logger
	TLSConfig            *tls.Config
}

func RunGRPCServer(ctx context.Context, cfg Config) error {
	if cfg.AnalysisServer == nil {
		return errors.New("grpc: analysis server is nil")
	}
	if cfg.CatalogServer == nil {
		return errors.New("grpc: analysis server is nil")
	}
	log := cfg.Logger
	if cfg.Logger == nil {
		log = slog.Default()
	}
	log = log.With("component", "grpc.server")

	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}

	var serverOpts []grpc.ServerOption
	if cfg.TLSConfig != nil {
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(cfg.TLSConfig)))
	}
	serverOpts = append(serverOpts,
		grpc.ChainUnaryInterceptor(
			interceptor.UserIDInterceptor(),
			interceptor.UnaryLogger(log),
			interceptor.UnaryError(log),
		),
		grpc.ChainStreamInterceptor(
			interceptor.StreamError(log)),
	)
	server := grpc.NewServer(serverOpts...)
	reflection.Register(server)

	analysispb.RegisterAnalysisServer(server, cfg.AnalysisServer)
	catalogpb.RegisterCatalogServer(server, cfg.CatalogServer)
	systempb.RegisterServiceMonitorServer(server, cfg.ServiceMonitorServer)

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
