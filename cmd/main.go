package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/pulsoats/analysis/internal/application/catalog"
	apprun "github.com/pulsoats/analysis/internal/application/run"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres/candle"
	"github.com/pulsoats/analysis/internal/infrastructure/repository/postgres/runs"
	"github.com/pulsoats/analysis/internal/transport/xgrpc/analysis"
	xrgpccatalog "github.com/pulsoats/analysis/internal/transport/xgrpc/catalog"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/detectors"
	"github.com/rs/zerolog/log"

	"github.com/pulsoats/analysis/internal/logger"
	"github.com/pulsoats/analysis/internal/transport/xgrpc"
	"github.com/pulsoats/core/exchanges"
	"github.com/pulsoats/core/tlsconfig"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	envPostgresDSN    = "POSTGRES_DSN"
	envRunsStorageDir = "RUNS_STORAGE_DIR"
	envGRPCHost       = "GRPC_HOST"
	envGRPCPort       = "GRPC_PORT"
	envTLSDisable     = "GRPC_TLS_DISABLE"
	envTLSCertFile    = "GRPC_TLS_CERT_FILE"
	envTLSKeyFile     = "GRPC_TLS_KEY_FILE"
	envTLSCAFile      = "GRPC_TLS_CA_FILE"
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

	detectorRegistry := detector.NewRegistry()
	if err = detectors.RegisterAll(detectorRegistry); err != nil {
		log.Fatal().Err(err).Msg("register detectors")
	}

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

	appRunCfg := apprun.Config{
		RunRepository:     runRepo,
		CandleRepository:  candleRepo,
		Exchanges:         exchangeAPIs,
		DetectorsRegistry: detectorRegistry,
		StorageDir:        storageDir,
		Logger:            slogLogger,
		TxManager:         txManager,
	}

	runApp, err := apprun.NewApplication(appRunCfg)
	if err != nil {
		zlog.Fatal().Err(err)
	}

	analysisSrv, err := analysis.NewServer(runApp, slogLogger)
	if err != nil {
		zlog.Fatal().Err(err)
	}

	detectorApp, err := catalog.NewApplication(detectorRegistry, exchangeAPIs)
	if err != nil {
		zlog.Fatal().Err(err)
	}
	catalogSrv, err := xrgpccatalog.NewServer(detectorApp)
	if err != nil {
		zlog.Fatal().Err(err)
	}

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	host := os.Getenv(envGRPCHost)
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv(envGRPCPort)
	if port == "" {
		port = "50051"
	}
	addr := net.JoinHostPort(host, port)

	tlsDisable, _ := strconv.ParseBool(os.Getenv(envTLSDisable))

	serverCfg := xgrpc.Config{
		Addr:           addr,
		AnalysisServer: analysisSrv,
		CatalogServer:  catalogSrv,
		HealthServer:   healthSrv,
		Logger:         slogLogger,
	}
	if tlsDisable {
		zlog.Warn().Msg("TLS disabled — insecure mode")
	} else {
		tlsProvider, err := tlsconfig.New(
			os.Getenv(envTLSCertFile),
			os.Getenv(envTLSKeyFile),
			os.Getenv(envTLSCAFile),
		)
		if err != nil {
			zlog.Fatal().Err(err).Msg("init tls provider")
		}
		serverCfg.TLSConfig = tlsProvider.ServerConfig()
	}

	zlog.Info().Str("addr", addr).Bool("tls", !tlsDisable).Msg("starting gRPC server")
	if err := xgrpc.RunGRPCServer(ctx, serverCfg); err != nil {
		zlog.Fatal().Err(err).Msg("gRPC server stopped with error")
	}

	zlog.Info().Msg("analysis service stopped")
}
