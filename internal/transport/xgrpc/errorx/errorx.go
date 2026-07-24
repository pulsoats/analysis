package errorx

import (
	"errors"

	"github.com/pulsoats/core/errorsx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToStatus maps domain errors to gRPC status codes.
func ToStatus(err error) *status.Status {
	switch {
	case errors.Is(err, errorsx.ErrUnauthorized):
		return status.New(codes.Unauthenticated, "unauthorized")
	case errors.Is(err, errorsx.ErrForbidden):
		return status.New(codes.PermissionDenied, "forbidden")
	case errors.Is(err, errorsx.ErrInternal):
		return status.New(codes.Internal, "internal server error")
	case errors.Is(err, errorsx.ErrNotFound):
		return status.New(codes.NotFound, "not found")
	case errors.Is(err, errorsx.ErrAlreadyExists):
		return status.New(codes.AlreadyExists, "already exists")
	case errors.Is(err, errorsx.ErrInvalidArgument), errors.Is(err, errorsx.ErrRequired):
		return status.New(codes.InvalidArgument, err.Error())
	default:
		return status.New(codes.Unknown, err.Error())
	}
}
