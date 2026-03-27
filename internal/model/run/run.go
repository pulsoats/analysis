package run

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
)

const (
	StatusPending int = iota + 1
	StatusRunning
	StatusDone
	StatusFailed
)

type Status struct {
	Code    int
	Message string
}

// Run описывает агрегированный результат прогонов исторического сервиса.
type Run struct {
	ID           int64
	Market       market.Spec
	Interval     market.Interval
	Detector     detect.DetectorConfig
	From         time.Time
	To           time.Time
	SignalsCount int64
	AvgProfitPPM *int64
	CreatedBy    string
	CreatedAt    time.Time
	Status       Status
}

func (r Run) String() string {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf("Run{id=%d}", r.ID)
	}
	return string(b)
}

type Repository interface {
	CreateRun(ctx context.Context, run *Run, status Status) error
	UpdateStatus(ctx context.Context, runID int64, status Status) error
	UpdateResult(ctx context.Context, res Run) error
	StatusByRunID(ctx context.Context, runID int64) (Status, error)
	RunByID(ctx context.Context, runID int64) (Run, error)
	ListRunsPaged(ctx context.Context, limit int, beforeID *int64) ([]Run, bool, *int64, error)
}

type Service interface {
	StartRun(ctx context.Context, req Request) (int64, error)
	Status(ctx context.Context, runID int64) (Status, error)
	StreamRunResult(ctx context.Context, runID int64, w io.Writer) error
	FindByID(ctx context.Context, runID int64) (Run, error)
	ListRunsPaged(ctx context.Context, limit int, beforeID *int64) ([]Run, bool, *int64, error)
}
