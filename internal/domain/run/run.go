package run

import (
	"context"
	"encoding/json"
	"fmt"
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

type Filter int

const (
	FilterUnspecified Filter = iota
	FilterMine
	FilterShared
	FilterAll
)

type Status struct {
	Code    int
	Message string
}

// Run описывает агрегированный результат прогонов исторического сервиса.
type Run struct {
	ID           int64
	Status       Status
	Market       market.Spec
	Interval     market.Interval
	PriceType    market.PriceType
	Detector     detect.DetectorConfig
	From         *time.Time
	To           *time.Time
	SignalsCount *int64
	AvgProfitPPM *int64
	CreatedBy    string
	CreatedAt    time.Time
	IsShared     bool
	SharedAt     *time.Time
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
	ListRunsPaged(ctx context.Context, limit int, beforeID *int64, callerID string, filter Filter) ([]Run, bool, *int64, error)
	ShareRun(ctx context.Context, runID int64, callerID string) error
	DeleteRun(ctx context.Context, runID int64) error
}
