package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryLogger логирует все входящие unary-запросы и их ошибки.
func UnaryLogger(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		if err == nil {
			log.Info("rpc handled",
				"method", info.FullMethod,
				"duration", dur,
				"code", codes.OK,
			)
			return resp, nil
		}

		st, _ := status.FromError(err)
		code := st.Code()

		if code == codes.Internal || code == codes.Unknown {
			log.Error("rpc handled",
				"method", info.FullMethod,
				"duration", dur,
				"code", code,
			)
		} else {
			log.Info("rpc handled",
				"method", info.FullMethod,
				"duration", dur,
				"code", code,
			)
		}

		return nil, err
	}
}
