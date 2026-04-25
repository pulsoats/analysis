package interceptor

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const UserIDKey contextKey = "user-id"

func UserIDInterceptor() grpc.UnaryServerInterceptor {
	requireUserID := map[string]bool{
		"/pulsoats.analysis.v1.Analysis/NewRun":        true,
		"/pulsoats.analysis.v1.Analysis/ShareRun":      true,
		"/pulsoats.analysis.v1.Analysis/DeleteRun":     true,
		"/pulsoats.analysis.v1.Analysis/ListRunsPaged": true,
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !requireUserID[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no metadata")
		}

		values := md.Get("x-user-id")
		if len(values) == 0 || values[0] == "" {
			return nil, status.Error(codes.Unauthenticated, "x-user-id is required")
		}

		ctx = context.WithValue(ctx, UserIDKey, values[0])

		return handler(ctx, req)
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}
