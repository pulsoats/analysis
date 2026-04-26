package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/google/uuid"
	"github.com/pulsoats/analysis/internal/application/catalog"
	apprun "github.com/pulsoats/analysis/internal/application/run"
	apphealth "github.com/pulsoats/analysis/internal/application/system"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres/candle"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres/runs"
	"github.com/pulsoats/analysis/internal/transport/xgrpc/analysis"
	xrgpccatalog "github.com/pulsoats/analysis/internal/transport/xgrpc/catalog"
	xgrpchealth "github.com/pulsoats/analysis/internal/transport/xgrpc/system"
	"github.com/rs/zerolog/log"

	"github.com/pulsoats/analysis/internal/logger"
	"github.com/pulsoats/analysis/internal/transport/xgrpc"
	"github.com/pulsoats/core/detect/detectors"
	"github.com/pulsoats/core/exchanges"
	coresystem "github.com/pulsoats/core/system"
)

const (
	envPostgresDSN        = "POSTGRES_DSN"
	envRunsStorageDir     = "RUNS_STORAGE_DIR"
	envGRPCHost           = "GRPC_HOST"
	envGRPCPort           = "GRPC_PORT"
	envServiceSecretToken = "SERVICE_SECRET_TOKEN"
	envServiceName        = "SERVICE_NAME"
)

var version = "dev"

func main() {
	zlog := logger.Configure()
	slogLogger := logger.NewSlogLogger(zlog)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv(envPostgresDSN)
	pool, err := postgres.NewPostgresPool(ctx, dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("init postgres pool")
	}
	defer pool.Close()

	detectorRegistry := detectors.NewDefaultRegistry()

	exReg := exchanges.NewRegistry(slogLogger)
	exchangeAPIs, err := exReg.CreateAllPublic(slogLogger)
	if err != nil {
		zlog.Fatal().Err(err).Msg("load exchange registry")
	}

	candleRepo, err := candle.NewRepository(pool)
	if err != nil {
		zlog.Fatal().Err(err).Msg("init candle repository")
	}
	qp := postgres.NewQuerierProvider(pool)
	runRepo := runs.NewRepository(qp)
	txManager := postgres.NewTxManager(pool)

	storageDir := os.Getenv(envRunsStorageDir)
	if storageDir == "" {
		storageDir = filepath.Join("data", "runs")
	}

	cfg := apprun.Config{
		RunRepository:     runRepo,
		CandleRepository:  candleRepo,
		Exchanges:         exchangeAPIs,
		DetectorsRegistry: detectorRegistry,
		StorageDir:        storageDir,
		Logger:            slogLogger,
		TxManager:         txManager,
	}

	runApp, err := apprun.NewApplication(cfg)
	if err != nil {
		zlog.Fatal().Err(err)
	}

	analysisSrv, err := analysis.NewServer(runApp, slogLogger)
	if err != nil {
		zlog.Fatal().Err(err)
	}

	detectorApp, err := catalog.NewApplication(detectorRegistry)
	if err != nil {
		zlog.Fatal().Err(err)
	}
	catalogSrv, err := xrgpccatalog.NewServer(detectorApp)
	if err != nil {
		zlog.Fatal().Err(err)
	}

	serviceID := uuid.New().String()
	serviceName := os.Getenv(envServiceName)
	if serviceName == "" {
		serviceName = "analysis_" + serviceID
	}

	healthApp := apphealth.NewApplication(coresystem.ServiceInfo{
		ID:       serviceID,
		Kind:     coresystem.ServiceKindAnalysis,
		Name:     serviceName,
		Exchange: "",
		Account:  "analysis",
		Version:  version,
	}, pool)
	healthSrv, err := xgrpchealth.NewServer(healthApp)
	if err != nil {
		zlog.Fatal().Err(err).Msg("init health server")
	}

	host := os.Getenv(envGRPCHost)
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv(envGRPCPort)
	if port == "" {
		port = "50051"
	}
	addr := net.JoinHostPort(host, port)

	secret := os.Getenv(envServiceSecretToken)
	if secret == "" {
		zlog.Fatal().Msg("SERVICE_SECRET_TOKEN is empty")
	}

	log.Info().Str("addr", addr).Msg("starting gRPC server")
	if err := xgrpc.RunGRPCServer(ctx, addr, analysisSrv, catalogSrv, healthSrv, slogLogger, secret); err != nil {
		zlog.Fatal().Err(err).Msg("gRPC server stopped with error")
	}

	zlog.Info().Msg("analysis service stopped")
}
