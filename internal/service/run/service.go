package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/pulsoats/analysis/internal/model/run"
	"github.com/pulsoats/analysis/internal/utils/files"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/detect/detectors"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/logx"
	"golang.org/x/sync/singleflight"

	dets "github.com/pulsoats/analysis/internal/detect"
	"github.com/pulsoats/analysis/internal/model"
	"github.com/pulsoats/core/domain/exchange"
)

type ServiceConfig struct {
	Repository       run.Repository
	CandleRepository model.CandleRepository
	Exchanges        map[string]exchange.API
	DetectService    *dets.Service
	DetectorRegistry *detectors.Registry
	StorageDir       string
	Logger           *slog.Logger
}

type service struct {
	repo        run.Repository
	candleRepo  model.CandleRepository
	exchanges   map[string]exchange.API
	detRegistry *detectors.Registry
	detectSvc   *dets.Service
	storageDir  string
	log         *slog.Logger
	candlesSF   singleflight.Group
}

func NewService(cfg ServiceConfig) run.Service {
	l := cfg.Logger
	if l == nil {
		l = logx.Discard()
	}
	return &service{
		repo:        cfg.Repository,
		candleRepo:  cfg.CandleRepository,
		exchanges:   cfg.Exchanges,
		detRegistry: cfg.DetectorRegistry,
		detectSvc:   cfg.DetectService,
		storageDir:  cfg.StorageDir,
		log:         l.With("component", "run.service"),
	}
}

func (s *service) StartRun(ctx context.Context, req run.Request) (int64, error) {
	if err := req.Validate(); err != nil {
		return 0, fmt.Errorf("start run: %w", err)
	}

	if _, ok := s.exchanges[req.Market.Exchange]; !ok {
		return 0, fmt.Errorf("start run: exchange %s: %w", req.Market.Exchange, errorsx.ErrNotFound)
	}

	from, to := req.From.UTC(), req.To.UTC()
	record := run.Run{
		Market:    req.Market,
		Interval:  req.Interval,
		PriceType: req.PriceType,
		Detector:  req.Detector,
		From:      &from,
		To:        &to,
		CreatedBy: req.UserID,
	}

	if err := s.repo.CreateRun(ctx, &record, run.Status{Code: run.StatusPending}); err != nil {
		return 0, fmt.Errorf("start run: create run: %w", err)
	}

	s.log.Info("run queued",
		"run_id", record.ID,
		"exchange", req.Market.Exchange,
		"symbol", req.Market.Symbol,
		"detector", req.Detector.Code,
	)

	go s.runAsync(record.ID, req)

	return record.ID, nil
}

func (s *service) runAsync(runID int64, req run.Request) {
	log := s.log.With("run_id", runID)
	ctx := context.Background()
	_ = s.repo.UpdateStatus(ctx, runID, run.Status{Code: run.StatusRunning})

	log.Info("run started")

	if err := s.executeRun(ctx, runID, req); err != nil {
		log.Error("run failed", "err", err)
		var msg string
		unwrappedError := errors.Unwrap(err)
		if unwrappedError != nil {
			msg = unwrappedError.Error()
		}
		_ = s.repo.UpdateStatus(ctx, runID, run.Status{
			Code:    run.StatusFailed,
			Message: msg,
		})
		return
	}

	log.Info("run completed")
	_ = s.repo.UpdateStatus(ctx, runID, run.Status{Code: run.StatusDone})
}

func (s *service) Status(ctx context.Context, runID int64) (run.Status, error) {
	status, err := s.repo.StatusByRunID(ctx, runID)
	if err != nil {
		return run.Status{}, fmt.Errorf("get run status: %w", err)
	}
	return status, nil
}

func (s *service) StreamRunResult(ctx context.Context, runID int64, w io.Writer) error {
	st, err := s.repo.StatusByRunID(ctx, runID)
	if err != nil {
		return fmt.Errorf("stream run result: %w", err)
	}
	if st.Code != run.StatusDone {
		return fmt.Errorf("stream run result: run %d result not ready: %w", runID, errorsx.ErrInvalidArgument)
	}

	zipPath := s.runZipPath(runID)

	f, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("stream run result: open archive: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	if _, err = io.CopyBuffer(w, f, buf); err != nil {
		return fmt.Errorf("stream run result: copy archive: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (s *service) FindByID(ctx context.Context, runID int64) (run.Run, error) {
	r, err := s.repo.RunByID(ctx, runID)
	if err != nil {
		return run.Run{}, fmt.Errorf("get run meta: %w", err)
	}
	return r, nil
}

func (s *service) ListRunsPaged(ctx context.Context, limit int, beforeID *int64, callerID string, filter run.RunFilter) ([]run.Run, bool, *int64, error) {
	return s.repo.ListRunsPaged(ctx, limit, beforeID, callerID, filter)
}

func (s *service) ShareRun(ctx context.Context, runID int64, callerID string) error {
	if err := s.repo.ShareRun(ctx, runID, callerID); err != nil {
		return fmt.Errorf("share run: %w", err)
	}
	return nil
}

func (s *service) executeRun(ctx context.Context, runID int64, cfg run.Request) error {
	optsAny, err := s.detRegistry.UnmarshalOpts(cfg.Detector.Code, cfg.Detector.Opts)
	if err != nil {
		return fmt.Errorf("execute run: decode detector opts: %w", errors.Join(errorsx.ErrInternal, err))
	}

	detector, err := s.detRegistry.NewCandle(cfg.Detector.Code, cfg.Detector.Label, optsAny)
	if err != nil {
		return fmt.Errorf("execute run: build detector: %w", errors.Join(errorsx.ErrInternal, err))
	}
	det, ok := detector.(detect.CandleDetector)
	if !ok {
		return fmt.Errorf("execute run: wrong detector type: %w", errorsx.ErrInternal)
	}

	exApi, ok := s.exchanges[cfg.Market.Exchange]
	if !ok {
		return fmt.Errorf("execute run: exchange %s: %w", cfg.Market.Exchange, errorsx.ErrNotFound)
	}

	fees := cfg.Fees
	if fees == nil {
		defaultFees, err := exApi.DefaultFees(cfg.Market.Category)
		if err != nil {
			return fmt.Errorf("execute run: get default fees: %w", err)
		}
		fees = &defaultFees
	}

	candles, err := s.fetchCandles(ctx, market.CandleSpec{Spec: cfg.Market, Interval: cfg.Interval}, cfg.From, cfg.To, cfg.PriceType)
	if err != nil {
		return fmt.Errorf("execute run: fetch candles: %w", err)
	}
	s.log.Debug("candles fetched", "run_id", runID, "count", len(candles))

	signals, err := s.detectSvc.Run(ctx, candles, *fees, det)
	if err != nil {
		return fmt.Errorf("execute run: detect signals: %w", errors.Join(errorsx.ErrInternal, err))
	}
	s.log.Debug("signals detected", "run_id", runID, "count", len(signals))

	res, err := s.processResult(ctx, processRunRequest{
		signals:   signals,
		candles:   candles,
		detector:  detector,
		exApi:     exApi,
		market:    cfg.Market,
		interval:  cfg.Interval,
		priceType: cfg.PriceType,
		fees:      *fees,
	})
	if err != nil {
		return fmt.Errorf("execute run: process result: %w", err)
	}
	fromBound, toBound := timeBounds(candles)
	signalsCount := int64(len(res.signals))
	analysisRun := run.Run{
		ID:           runID,
		Market:       cfg.Market,
		Interval:     cfg.Interval,
		Detector:     cfg.Detector,
		From:         &fromBound,
		To:           &toBound,
		SignalsCount: &signalsCount,
		AvgProfitPPM: &res.avgProfitPPM,
		CreatedBy:    cfg.UserID,
	}

	if err := s.repo.UpdateResult(ctx, analysisRun); err != nil {
		return fmt.Errorf("execute run: update result: %w", err)
	}

	zipPath := s.runZipPath(runID)
	if err := files.BuildZipResult(ctx, zipPath, analysisRun, candles, res.signals); err != nil {
		return fmt.Errorf("execute run: build archive: %w", err)
	}
	return nil
}
