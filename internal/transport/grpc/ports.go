package grpc

import (
	"context"
	"io"

	"github.com/pulsoats/analysis/internal/domain/run"
)

type app interface {
	StartRun(ctx context.Context, req run.Request) (int64, error)
	Status(ctx context.Context, runID int64) (run.Status, error)
	StreamRunResult(ctx context.Context, runID int64, w io.Writer) error
	FindByID(ctx context.Context, runID int64) (run.Run, error)
	ListRunsPaged(ctx context.Context, limit int, beforeID *int64, callerID string, filter run.Filter) ([]run.Run, bool, *int64, error)
	ShareRun(ctx context.Context, runID int64, callerID string) error
	DeleteRun(ctx context.Context, runID int64, userID string) error
}
