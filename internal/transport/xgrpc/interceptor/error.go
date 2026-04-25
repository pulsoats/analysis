package interceptor

import (
	"context"
	"errors"
	"log/slog"

	"github.com/pulsoats/analysis/internal/transport/xgrpc/errorx"
	"github.com/pulsoats/core/errorsx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryError конвертирует domain-ошибки в gRPC-статусы.
// Перед конвертацией логирует неожиданные исходные ошибки.
func UnaryError(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		if _, isStatus := status.FromError(err); isStatus {
			return nil, err
		}

		// Логируем исходную ошибку до преобразования только для нештатных кейсов.
		switch {
		// expected business errors
		case errors.Is(err, errorsx.ErrNotFound),
			errors.Is(err, errorsx.ErrUnauthorized),
			errors.Is(err, errorsx.ErrAlreadyExists),
			errors.Is(err, errorsx.ErrInvalidArgument),
			errors.Is(err, errorsx.ErrRequired),
			errors.Is(err, errorsx.ErrForbidden):
		default:
			log.Error("rpc handler returned raw error",
				"method", info.FullMethod,
				"err", err,
			)
		}

		return nil, errorx.ToStatus(err).Err()
	}
}

// StreamError конвертирует domain-ошибки в gRPC-статусы для streaming-методов.
func StreamError(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		if err == nil {
			return nil
		}

		if _, isStatus := status.FromError(err); isStatus {
			return err
		}

		// Логируем исходную ошибку до преобразования только для нештатных кейсов.
		switch {
		// expected business errors
		case errors.Is(err, errorsx.ErrNotFound),
			errors.Is(err, errorsx.ErrUnauthorized),
			errors.Is(err, errorsx.ErrAlreadyExists),
			errors.Is(err, errorsx.ErrInvalidArgument),
			errors.Is(err, errorsx.ErrRequired),
			errors.Is(err, errorsx.ErrForbidden):
		default:
			log.Error("rpc handler returned raw error",
				"method", info.FullMethod,
				"err", err,
			)
		}

		return errorx.ToStatus(err).Err()
	}
}
