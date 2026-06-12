package analysis

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/domain/run"
)

type app interface {
	NewRun(ctx context.Context, req run.NewRunRequest) (run.Run, error)
	RunByID(ctx context.Context, runID uuid.UUID) (run.Run, error)
	StreamRunArchive(ctx context.Context, runID uuid.UUID, w io.Writer) error
	RunsPaged(ctx context.Context, req run.RunsPagedRequest) (run.RunsPagedResponse, error)
	ShareRun(ctx context.Context, runID uuid.UUID, callerID string) error
	DeleteRun(ctx context.Context, runID uuid.UUID, userID string) error
}
