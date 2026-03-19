package middleware

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLogger возвращает interceptor, который логирует запросы.
func UnaryLogger(logger func(method string, code string, duration time.Duration, err error)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		st := status.Convert(err)
		if logger != nil {
			logger(info.FullMethod, st.Code().String(), time.Since(start), err)
		}
		return resp, err
	}
}
