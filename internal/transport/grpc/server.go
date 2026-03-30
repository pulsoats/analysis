package grpc

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/pulsoats/analysis/internal/transport/grpc/middleware"
	"github.com/pulsoats/core/lib/logx"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func RunGRPCServer(ctx context.Context, addr string, analysisSrv analysispb.AnalysisServer, log *slog.Logger) error {
	if log == nil {
		log = logx.Discard()
	}
	log = log.With("component", "grpc.server")

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryLogger(log)),
	)
	reflection.Register(server)

	analysispb.RegisterAnalysisServer(server, analysisSrv)

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
