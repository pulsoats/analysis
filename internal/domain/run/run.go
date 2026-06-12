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
	Fees         market.TakerMakerFees
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

// Filter — параметры выборки прогонов.
type Filter struct {
	Exchanges     []string // market.exchange IN (...)
	Categories    []string // market.category IN (...)
	Symbols       []string // market.symbol   IN (...)
	Intervals     []string // interval        IN (...)
	DetectorCodes []string // detector.code   IN (...)
	Statuses      []int    // status.code     IN (...), 0..4

	MinSignals      *int64 // signalsCount >= / <=
	MaxSignals      *int64
	MinAvgProfitPPM *int64 // avgProfitPPM >= / <= (перевести в PPM)
	MaxAvgProfitPPM *int64

	// период свечей (колонка Period = firstCandleTime → lastCandleTime)
	FirstCandleFrom *time.Time // first_candle_time >= FirstCandleFrom
	LastCandleTo    *time.Time // last_candle_time  <= LastCandleTo

	CreatedFrom *time.Time // createdAt >= / <=
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
