package middleware

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLogger logs every unary RPC at Info level with method, gRPC status code,
// and duration. Errors are expected to be logged with full context inside each
// handler, so the interceptor does not duplicate error logging.
func UnaryLogger(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		log.Info("grpc request",
			"method", info.FullMethod,
			"code", status.Code(err).String(),
			"dur", time.Since(start),
		)
		return resp, err
	}
}
