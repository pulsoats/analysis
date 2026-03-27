package run

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pulsoats/analysis/internal/model/run"
	"github.com/pulsoats/analysis/internal/utils/files"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/detect/detectors"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/lib/errorsx"
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
}

type service struct {
	repo        run.Repository
	candleRepo  model.CandleRepository
	exchanges   map[string]exchange.API
	detRegistry *detectors.Registry
	detectSvc   *dets.Service
	storageDir  string
	candlesSF   singleflight.Group
}

func NewService(cfg ServiceConfig) run.Service {
	return &service{
		repo:        cfg.Repository,
		candleRepo:  cfg.CandleRepository,
		exchanges:   cfg.Exchanges,
		detRegistry: cfg.DetectorRegistry,
		detectSvc:   cfg.DetectService,
		storageDir:  cfg.StorageDir,
	}
}

func (s *service) StartRun(ctx context.Context, req run.Request) (int64, error) {
	if err := req.Validate(); err != nil {
		return 0, fmt.Errorf("start run: %w", err)
	}

	if _, ok := s.exchanges[req.Market.Exchange]; !ok {
		return 0, fmt.Errorf("start run: %w: exchange %s", derrors.ErrNotFound, req.Market.Exchange)
	}

	record := run.Run{
		Market:    req.Market,
		Interval:  req.Interval,
		Detector:  req.Detector,
		From:      req.From.UTC(),
		To:        req.To.UTC(),
		CreatedBy: req.UserID,
	}

	if err := s.repo.CreateRun(ctx, &record, run.Status{Code: run.StatusPending}); err != nil {
		return 0, fmt.Errorf("start run: create run: %w", err)
	}

	go s.runAsync(record.ID, req)

	return record.ID, nil
}

func (s *service) runAsync(runID int64, req run.Request) {
	ctx := context.Background()
	_ = s.repo.UpdateStatus(ctx, runID, run.Status{Code: run.StatusRunning})

	if err := s.executeRun(ctx, runID, req); err != nil {
		_ = s.repo.UpdateStatus(ctx, runID, run.Status{
			Code:    run.StatusFailed,
			Message: err.Error(),
		})
		return
	}

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
		return fmt.Errorf("stream run result: %w: run %d result not ready", derrors.ErrInvalidArgument, runID)
	}

	zipPath := s.runZipPath(runID)

	f, err := os.Open(zipPath)
	if err != nil {
		return fmt.Errorf("stream run result: open archive: %w: %v", errorsx.ErrInternal, err)
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	if _, err = io.CopyBuffer(w, f, buf); err != nil {
		return fmt.Errorf("stream run result: copy archive: %w: %v", errorsx.ErrInternal, err)
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

func (s *service) ListRunsPaged(ctx context.Context, limit int, beforeID *int64) ([]run.Run, bool, *int64, error) {
	return s.repo.ListRunsPaged(ctx, limit, beforeID)
}

func (s *service) executeRun(ctx context.Context, runID int64, cfg run.Request) error {
	if cfg.Fees == nil {
		return fmt.Errorf("execute run: %w: fees config", derrors.ErrRequired)
	}

	optsAny, err := s.detRegistry.UnmarshalOpts(cfg.Detector.Code, cfg.Detector.Opts)
	if err != nil {
		return fmt.Errorf("execute run: decode detector opts: %w: %v", errorsx.ErrInternal, err)
	}

	detector, err := s.detRegistry.NewCandle(cfg.Detector.Code, cfg.Detector.Label, optsAny)
	if err != nil {
		return fmt.Errorf("execute run: build detector: %w: %v", errorsx.ErrInternal, err)
	}
	det, ok := detector.(detect.CandleDetector)
	if !ok {
		return fmt.Errorf("execute run: %w: wrong detector type", errorsx.ErrInternal)
	}

	exApi, ok := s.exchanges[cfg.Market.Exchange]
	if !ok {
		return fmt.Errorf("execute run: %w: exchange %s", derrors.ErrNotFound, cfg.Market.Exchange)
	}

	candles, err := s.fetchCandles(ctx, market.CandleSpec{Spec: cfg.Market, Interval: cfg.Interval}, cfg.From, cfg.To, cfg.PriceType)
	if err != nil {
		return fmt.Errorf("execute run: fetch candles: %w", err)
	}
	signals, err := s.detectSvc.Run(ctx, candles, *cfg.Fees, det)
	if err != nil {
		return fmt.Errorf("execute run: detect signals: %w: %v", errorsx.ErrInternal, err)
	}

	res, err := s.processResult(ctx, processRunRequest{
		signals:   signals,
		candles:   candles,
		detector:  detector,
		exApi:     exApi,
		market:    cfg.Market,
		interval:  cfg.Interval,
		priceType: cfg.PriceType,
		fees:      *cfg.Fees,
	})
	if err != nil {
		return fmt.Errorf("execute run: process result: %w", err)
	}
	fromBound, toBound := timeBounds(candles)
	analysisRun := run.Run{
		ID:           runID,
		Market:       cfg.Market,
		Interval:     cfg.Interval,
		Detector:     cfg.Detector,
		From:         fromBound,
		To:           toBound,
		SignalsCount: int64(len(res.signals)),
		AvgProfitPPM: &res.avgProfitPPM,
		CreatedBy:    cfg.UserID,
	}

	if err := s.repo.UpdateResult(ctx, analysisRun); err != nil {
		return fmt.Errorf("execute run: update result: %w", err)
	}

	zipPath := s.runZipPath(runID)
	if err := files.BuildZipResult(ctx, zipPath, analysisRun, candles, res.signals); err != nil {
		return fmt.Errorf("execute run: build archive: %w: %v", errorsx.ErrInternal, err)
	}
	return nil
}
