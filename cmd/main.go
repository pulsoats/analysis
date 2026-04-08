package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pulsoats/analysis/internal/application/run"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres/candles"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres/runs"
	"github.com/rs/zerolog/log"

	"github.com/pulsoats/analysis/internal/detect"
	"github.com/pulsoats/analysis/internal/logger"
	transportgrpc "github.com/pulsoats/analysis/internal/transport/grpc"
	"github.com/pulsoats/core/domain/detect/detectors"
	"github.com/pulsoats/core/exchanges"
)

func main() {
	zl := logger.Configure()
	slogLogger := logger.NewSlogLogger(zl)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPostgresPool(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("init postgres pool")
	}
	defer pool.Close()

	detectorRegistry := detectors.NewDefaultRegistry()
	detectService := detect.NewDetectService()

	exReg := exchanges.NewRegistry(exchanges.WithLogger(slogLogger))
	exchangeAPIs, err := exReg.CreateAllPublic()
	if err != nil {
		log.Fatal().Err(err).Msg("load exchange registry")
	}

	candleRepo := candles.NewRepository(pool)
	qp := postgres.NewQuerierProvider(pool)
	runRepo := runs.NewRepository(qp)

	storageDir := os.Getenv("ANALYSIS_STORAGE_DIR")
	if storageDir == "" {
		storageDir = filepath.Join("data", "runs")
	}

	cfg := run.ServiceConfig{
		Repository:       runRepo,
		CandleRepository: candleRepo,
		Exchanges:        exchangeAPIs,
		DetectService:    detectService,
		DetectorRegistry: detectorRegistry,
		StorageDir:       storageDir,
		Logger:           slogLogger,
	}

	runService := run.NewService(cfg)
	analysisSrv := transportgrpc.NewAnalysisServer(runService, transportgrpc.WithLogger(slogLogger))

	addr := os.Getenv("ANALYSIS_GRPC_ADDR")
	if addr == "" {
		addr = ":50051"
	}

	log.Info().Str("addr", addr).Msg("starting gRPC server")
	if err := transportgrpc.RunGRPCServer(ctx, addr, analysisSrv, slogLogger); err != nil {
		log.Fatal().Err(err).Msg("gRPC server stopped with error")
	}

	log.Info().Msg("analysis service stopped")
}
