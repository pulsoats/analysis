package run

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/market"
	corerun "github.com/pulsoats/core/run"
)

type Filter int

const (
	FilterUnspecified Filter = iota
	FilterMine
	FilterShared
	FilterAll
)

// Run описывает агрегированный результат прогона исторического сервиса.
type Run struct {
	corerun.Base
	Fees         market.TakerMakerFees
	SignalsCount *int64
	AvgProfitPPM *int64
	IsShared     bool
	SharedAt     *time.Time
}

func (r Run) String() string {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf("Run{id=%s}", r.ID)
	}
	return string(b)
}

type Repository interface {
	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, run Run) error
	RunByID(ctx context.Context, runID uuid.UUID) (Run, error)
	ListRunsPaged(ctx context.Context, limit int, beforeID *uuid.UUID, callerID string, filter Filter) ([]Run, bool, *uuid.UUID, error)
	ShareRun(ctx context.Context, runID uuid.UUID, callerID string) error
	DeleteRun(ctx context.Context, runID uuid.UUID) error
}
