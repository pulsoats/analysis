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
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/errorsx"
	corerun "github.com/pulsoats/core/run"
	"golang.org/x/sync/singleflight"

	"github.com/pulsoats/analysis/internal/domain"
	"github.com/pulsoats/core/exchange"
)

const (
	defaultRunsPagedLimit = 20
	maxRunsPagedLimit     = 100
)

type Config struct {
	RunRepository    run.Repository
	CandleRepository candle.Repository
	Exchanges        map[string]exchange.PublicClient
	DetectorRegistry *detector.Registry
	FilterRegistry   *filter.Registry
	StorageDir       string
	Logger           *slog.Logger
	TxManager        domain.TxManager
}

type Application struct {
	runRepo     run.Repository
	candleRepo  candle.Repository
	exchanges   map[string]exchange.PublicClient
	detRegistry *detector.Registry
	filRegistry *filter.Registry
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
	if cfg.DetectorRegistry == nil {
		return nil, errors.New("run app: nil detector registry")
	}
	if cfg.FilterRegistry == nil {
		return nil, errors.New("run app: nil filter registry")
	}
	if cfg.StorageDir == "" {
		return nil, errors.New("run app: empty storage dir")
	}
	if cfg.TxManager == nil {
		return nil, errors.New("run app: nil tx manager")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Application{
		runRepo:     cfg.RunRepository,
		candleRepo:  cfg.CandleRepository,
		exchanges:   cfg.Exchanges,
		detRegistry: cfg.DetectorRegistry,
		filRegistry: cfg.FilterRegistry,
		storageDir:  cfg.StorageDir,
		log:         cfg.Logger,
		tx:          cfg.TxManager,
	}, nil
}

func (a *Application) NewRun(ctx context.Context, req run.NewRunRequest) (run.Run, error) {
	const op = "new run"

	if req.UserID == "" {
		return run.Run{}, fmt.Errorf("%s: user_id: %w", op, errorsx.ErrRequired)
	}
	if req.Market.Exchange == "" || req.Market.Category == "" {
		return run.Run{}, fmt.Errorf("%s: market exchange or/and category: %w", op, errorsx.ErrRequired)
	}
	if req.Market.Symbol == "" {
		return run.Run{}, fmt.Errorf("%s: market symbol: %w", op, errorsx.ErrRequired)
	}
	if req.From.IsZero() || req.To.IsZero() {
		return run.Run{}, fmt.Errorf("%s: time range: %w", op, errorsx.ErrRequired)
	}
	if !req.From.Before(req.To) {
		return run.Run{}, fmt.Errorf("%s: time range: %w", op, errorsx.ErrInvalidArgument)
	}
	if req.Detector.Code == "" {
		return run.Run{}, fmt.Errorf("%s: detector code: %w", op, errorsx.ErrRequired)
	}

	exClient, ok := a.exchanges[req.Market.Exchange]
	if !ok {
		return run.Run{}, fmt.Errorf("%s: exchange %s: %w", op, req.Market.Exchange, errorsx.ErrNotFound)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return run.Run{}, fmt.Errorf("%s: uuid gen: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	fees, err := exClient.DefaultFees(req.Market.Category)
	if err != nil {
		return run.Run{}, fmt.Errorf("%s: %w", op, err)
	}
	if req.Fees != nil {
		fees = *req.Fees
	}

	optsAny, err := a.detRegistry.UnmarshalOpts(req.Detector.Code, req.Detector.Version, req.Detector.Opts)
	if err != nil {
		return run.Run{}, fmt.Errorf("%s: decode detector opts: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	det, err := a.detRegistry.New(req.Detector.Code, req.Detector.Version, req.Detector.OptsLabel, optsAny)
	if err != nil {
		return run.Run{}, fmt.Errorf("%s: build detector: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	if len(req.Filters) > 0 {
		filters := make([]filter.Filter, 0, len(req.Filters))
		for _, cfg := range req.Filters {
			f, err := filter.FilterFromConfig(a.filRegistry, cfg)
			if err != nil {
				return run.Run{}, fmt.Errorf("%s: build filter: %w", op, err)
			}
			filters = append(filters, f)
		}
		det = detector.Wrap(det, filters)
	}

	from, to := req.From.UTC(), req.To.UTC()
	r := run.Run{
		Base: corerun.Base{
			ID:              id,
			Status:          corerun.StatusPending,
			Market:          req.Market,
			Interval:        req.Interval,
			Detector:        req.Detector,
			Filters:         req.Filters,
			FirstCandleTime: from,
			LastCandleTime:  to,
			CreatedBy:       req.UserID,
		},
		Fees:            fees,
		DisableStopLoss: req.DisableStopLoss,
		DisableRepeats:  req.DisableRepeats,
	}

	if err := a.runRepo.CreateRun(ctx, &r); err != nil {
		return run.Run{}, fmt.Errorf("%s: %w", op, err)
	}

	go a.executeRun(r, det)

	return r, nil
}

func (a *Application) RunByID(ctx context.Context, runID uuid.UUID) (run.Run, error) {
	r, err := a.runRepo.RunByID(ctx, runID)
	if err != nil {
		return run.Run{}, fmt.Errorf("run by id: %w", err)
	}
	return r, nil
}

func (a *Application) StreamRunArchive(ctx context.Context, runID uuid.UUID, w io.Writer) error {
	const op = "%s"
	r, err := a.runRepo.RunByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if r.Status.Code != corerun.StatusCodeDone {
		return fmt.Errorf("%s: run %s archive is not ready: %w", op, runID, errorsx.ErrInvalidArgument)
	}

	zipPath := a.runZipPath(r.ID)

	f, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("%s: open archive: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	if _, err = io.CopyBuffer(w, f, buf); err != nil {
		return fmt.Errorf("%s: copy archive: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (a *Application) ShareRun(ctx context.Context, runID uuid.UUID, callerID string) error {
	if err := a.runRepo.ShareRun(ctx, runID, callerID); err != nil {
		return fmt.Errorf("share run: %w", err)
	}
	return nil
}

func (a *Application) DeleteRun(ctx context.Context, runID uuid.UUID, userID string) error {
	const op = "delete run"
	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		r, err := a.runRepo.RunByID(txCtx, runID)
		if err != nil {
			return err
		}

		if r.CreatedBy != userID {
			return errorsx.ErrForbidden
		}

		if err := a.runRepo.DeleteRun(txCtx, runID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	err = os.Remove(a.runZipPath(runID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("%s: delete archive: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (a *Application) RunsPaged(ctx context.Context, req run.RunsPagedRequest) (run.RunsPagedResponse, error) {
	switch {
	case req.Limit <= 0:
		req.Limit = defaultRunsPagedLimit
	case req.Limit > maxRunsPagedLimit:
		req.Limit = maxRunsPagedLimit
	}
	return a.runRepo.RunsPaged(ctx, req)
}

func (a *Application) runZipPath(runID uuid.UUID) string {
	filename := fmt.Sprintf("run_%s.zip", runID)
	return filepath.Join(a.storageDir, filename)
}
