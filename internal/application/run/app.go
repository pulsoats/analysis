package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/domain/candle"
	"github.com/pulsoats/analysis/internal/domain/run"
	"github.com/pulsoats/core/detect/detectors"
	"github.com/pulsoats/core/errorsx"
	corerun "github.com/pulsoats/core/run"
	"golang.org/x/sync/singleflight"

	"github.com/pulsoats/analysis/internal/domain"
	"github.com/pulsoats/core/exchange"
)

type Config struct {
	RunRepository     run.Repository
	CandleRepository  candle.Repository
	Exchanges         map[string]exchange.PublicClient
	DetectorsRegistry *detectors.Registry
	StorageDir        string
	Logger            *slog.Logger
	TxManager         domain.TxManager
}

type Application struct {
	runRepo     run.Repository
	candleRepo  candle.Repository
	exchanges   map[string]exchange.PublicClient
	detRegistry *detectors.Registry
	storageDir  string
	log         *slog.Logger
	candlesSF   singleflight.Group
	tx          domain.TxManager
}

func NewApplication(cfg Config) (*Application, error) {
	if cfg.RunRepository == nil {
		return nil, errors.New("run app: nil run repository")
	}
	if cfg.CandleRepository == nil {
		return nil, errors.New("run app: nil candle repository")
	}
	if len(cfg.Exchanges) == 0 {
		return nil, errors.New("run app: empty exchange clients map")
	}
	if cfg.DetectorsRegistry == nil {
		return nil, errors.New("run app: nil detectors registry")
	}
	if cfg.StorageDir == "" {
		return nil, errors.New("run app: empty storage dir")
	}
	if cfg.TxManager == nil {
		return nil, errors.New("run app: nil tx manager")
	}

	logger := slog.Default()
	if cfg.Logger != nil {
		logger = cfg.Logger
	}

	return &Application{
		runRepo:     cfg.RunRepository,
		candleRepo:  cfg.CandleRepository,
		exchanges:   cfg.Exchanges,
		detRegistry: cfg.DetectorsRegistry,
		storageDir:  cfg.StorageDir,
		log:         logger.With("component", "run.Application"),
		tx:          cfg.TxManager,
	}, nil
}

func (s *Application) NewRun(ctx context.Context, req run.NewRunRequest) (run.Run, error) {
	if req.UserID == "" {
		return run.Run{}, fmt.Errorf("new run: user_id: %w", errorsx.ErrRequired)
	}
	if req.Market.Exchange == "" || req.Market.Category == "" {
		return run.Run{}, fmt.Errorf("new run: market exchange or/and category: %w", errorsx.ErrRequired)
	}
	if req.Market.Symbol == "" {
		return run.Run{}, fmt.Errorf("new run: market symbol: %w", errorsx.ErrRequired)
	}
	if req.From.IsZero() || req.To.IsZero() {
		return run.Run{}, fmt.Errorf("new run: time range: %w", errorsx.ErrRequired)
	}
	if !req.From.Before(req.To) {
		return run.Run{}, fmt.Errorf("new run: time range: %w", errorsx.ErrInvalidArgument)
	}
	if req.Detector.Code == "" {
		return run.Run{}, fmt.Errorf("new run: detector code: %w", errorsx.ErrRequired)
	}

	exClient, ok := s.exchanges[req.Market.Exchange]
	if !ok {
		return run.Run{}, fmt.Errorf("new run: exchange %s: %w", req.Market.Exchange, errorsx.ErrNotFound)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return run.Run{}, fmt.Errorf("new run: uuid gen: %w", errors.Join(errorsx.ErrInternal, err))
	}

	fees, err := exClient.DefaultFees(req.Market.Category)
	if err != nil {
		return run.Run{}, fmt.Errorf("new run: %w", err)
	}
	if req.Fees != nil {
		fees = *req.Fees
	}

	optsAny, err := s.detRegistry.UnmarshalOpts(req.Detector.Code, req.Detector.Opts)
	if err != nil {
		return run.Run{}, fmt.Errorf("decode detector opts: %w", errors.Join(errorsx.ErrInternal, err))
	}

	detector, err := s.detRegistry.NewCandle(req.Detector.Code, req.Detector.OptsLabel, optsAny)
	if err != nil {
		return run.Run{}, fmt.Errorf("build detector: %w", errors.Join(errorsx.ErrInternal, err))
	}

	from, to := req.From.UTC(), req.To.UTC()
	r := run.Run{
		Base: corerun.Base{
			ID:              id,
			Status:          corerun.StatusPending,
			Market:          req.Market,
			Interval:        req.Interval,
			Detector:        req.Detector,
			FirstCandleTime: from,
			LastCandleTime:  to,
			CreatedBy:       req.UserID,
		},
		Fees: fees,
	}

	if err := s.runRepo.CreateRun(ctx, &r); err != nil {
		return run.Run{}, fmt.Errorf("new run: %w", err)
	}

	go s.executeRun(r, detector, fees)

	return r, nil
}

func (s *Application) RunByID(ctx context.Context, runID uuid.UUID) (run.Run, error) {
	r, err := s.runRepo.RunByID(ctx, runID)
	if err != nil {
		return run.Run{}, fmt.Errorf("run by id: %w", err)
	}
	return r, nil
}

func (s *Application) StreamRunArchive(ctx context.Context, runID uuid.UUID, w io.Writer) error {
	r, err := s.runRepo.RunByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("stream run archive: %w", err)
	}
	if r.Status.Code != corerun.StatusCodeDone {
		return fmt.Errorf("stream run archive: run %s archive is not ready: %w", runID, errorsx.ErrInvalidArgument)
	}

	zipPath := s.runZipPath(r.ID)

	f, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("stream run archive: open archive: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	if _, err = io.CopyBuffer(w, f, buf); err != nil {
		return fmt.Errorf("stream run result: copy archive: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (s *Application) ShareRun(ctx context.Context, runID uuid.UUID, callerID string) error {
	if err := s.runRepo.ShareRun(ctx, runID, callerID); err != nil {
		return fmt.Errorf("share run: %w", err)
	}
	return nil
}

func (s *Application) DeleteRun(ctx context.Context, runID uuid.UUID, userID string) error {
	err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := s.runRepo.RunByID(txCtx, runID)
		if err != nil {
			return err
		}

		if r.CreatedBy != userID {
			return errorsx.ErrForbidden
		}

		if err := s.runRepo.DeleteRun(txCtx, runID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}

	err = os.Remove(s.runZipPath(runID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete run: delete archive: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (s *Application) ListRunsPaged(ctx context.Context, limit int, beforeID *uuid.UUID, callerID string, filter run.Filter) ([]run.Run, bool, *uuid.UUID, error) {
	if limit <= 0 {
		return nil, false, nil, fmt.Errorf("list runs paged: limit %d: %w", limit, errorsx.ErrInvalidArgument)
	}
	return s.runRepo.ListRunsPaged(ctx, limit, beforeID, callerID, filter)
}

func (s *Application) runZipPath(runID uuid.UUID) string {
	filename := fmt.Sprintf("run_%s.zip", runID)
	return filepath.Join(s.storageDir, filename)
}
