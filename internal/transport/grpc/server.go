package grpc

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/pulsoats/analysis/internal/transport/grpc/middleware"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func RunGRPCServer(ctx context.Context, addr string, analysisSrv analysispb.AnalysisServer) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryLogger(func(method, code string, duration time.Duration, err error) {
			ev := log.Info()
			if err != nil {
				ev = log.Error().Err(err)
			}
			ev.Str("grpc_method", method).
				Str("grpc_code", code).
				Dur("duration", duration).
				Msg("grpc request")
		})),
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
