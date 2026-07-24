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

type Scope string

const (
	ScopeMine   Scope = "mine"   // прогоны текущего пользователя (по умолчанию)
	ScopeShared Scope = "shared" // расшаренные другими
	ScopeAll    Scope = "all"
)

// Run описывает агрегированный результат прогона исторического сервиса.
type Run struct {
	corerun.Base
	Fees            market.TakerMakerFees
	DisableStopLoss bool
	DisableRepeats  bool
	SumProfitPPM    int64
	AvgProfitPPM    int64
	IsShared        bool
	SharedAt        *time.Time
}

func (r Run) String() string {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf("Run{id=%s}", r.ID)
	}
	return string(b)
}

// Filter — параметры выборки прогонов.
type Filter struct {
	Exchanges      []string
	Categories     []string
	Symbols        []string
	Intervals      []string
	DetectorsCodes []string
	Statuses       []int

	MinSignals      *int64
	MaxSignals      *int64
	MinAvgProfitPPM *int64
	MaxAvgProfitPPM *int64

	DisableStopLoss *bool
	DisableRepeats  *bool

	FirstCandleFrom *time.Time
	LastCandleTo    *time.Time

	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type RunsPagedRequest struct {
	Limit       int
	BeforeID    *uuid.UUID
	UserID      string
	OrderDirAsc bool
	Scope       Scope
	Filter      *Filter
}

type RunsPagedResponse struct {
	Runs         []Run
	HasMore      bool
	NextBeforeID *uuid.UUID
}

type Repository interface {
	CreateRun(ctx context.Context, run *Run) error
	UpdateRun(ctx context.Context, run Run) error
	RunByID(ctx context.Context, runID uuid.UUID) (Run, error)
	RunsPaged(ctx context.Context, req RunsPagedRequest) (RunsPagedResponse, error)
	ShareRun(ctx context.Context, runID uuid.UUID, callerID string) error
	DeleteRun(ctx context.Context, runID uuid.UUID) error
}
