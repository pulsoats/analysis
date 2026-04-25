package interceptor

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func ServiceTokenUnaryInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no metadata")
		}

		values := md.Get("x-service-token")
		if len(values) == 0 || values[0] != secret {
			return nil, status.Error(codes.Unauthenticated, "invalid service token")
		}

		return handler(ctx, req)
	}
}

func ServiceTokenStreamInterceptor(secret string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "no metadata")
		}

		values := md.Get("x-service-token")
		if len(values) == 0 || values[0] != secret {
			return status.Error(codes.Unauthenticated, "invalid service token")
		}

		return handler(srv, ss)
	}
}
