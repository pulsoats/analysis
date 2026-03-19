package errorx

import (
	"errors"

	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/lib/errorsx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToStatus maps domain errors to gRPC status codes.
func ToStatus(err error) *status.Status {
	switch {
	case errors.Is(err, derrors.ErrNotFound):
		return status.New(codes.NotFound, "not found")
	case errors.Is(err, derrors.ErrUnauthorized):
		return status.New(codes.Unauthenticated, "unauthorized")
	case errors.Is(err, derrors.ErrAlreadyExists):
		return status.New(codes.AlreadyExists, "already exists")
	case errors.Is(err, derrors.ErrInvalidArgument), errors.Is(err, derrors.ErrRequired):
		return status.New(codes.InvalidArgument, err.Error())
	case errors.Is(err, errorsx.ErrInternal):
		return status.New(codes.Internal, "internal server error")
	default:
		return status.New(codes.Unknown, err.Error())
	}
}
