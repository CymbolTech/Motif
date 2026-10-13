package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/admin"
	"github.com/motifengine/motifengine/internal/adminauth"
	"github.com/motifengine/motifengine/internal/bootstrap"
	"github.com/motifengine/motifengine/internal/config"
	"github.com/motifengine/motifengine/internal/events"
	"github.com/motifengine/motifengine/internal/health"
	"github.com/motifengine/motifengine/internal/observability"
	"github.com/motifengine/motifengine/internal/points"
	"github.com/motifengine/motifengine/internal/rewards"
	"github.com/motifengine/motifengine/internal/runtime"
	"github.com/motifengine/motifengine/internal/transport"
	"github.com/nats-io/nats.go"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := runtime.SignalContext(context.Background())
	defer stop()

	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := pgxpool.New(pctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(pctx); err != nil {
		return err
	}
	if err := bootstrap.EnsureIntegrationCredential(pctx, db, cfg.BootstrapTenant, cfg.BootstrapEnv, cfg.BootstrapAPIKey); err != nil {
		return err
	}
	if err := bootstrap.EnsureSuperAdmin(pctx, db, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		return err
	}

	nc, err := nats.Connect(cfg.NATSURL, nats.Name("motifengine-api"))
	if err != nil {
		return err
	}
	defer nc.Drain()

	mux := http.NewServeMux()
	metrics := observability.NewMetrics()
	if cfg.MetricsEnabled {
		mux.Handle("/metrics", metrics.Handler())
	}
	mux.Handle("/health/", health.NewHandler(health.Dependencies{
		Database: func() error { return db.Ping(context.Background()) },
		NATS:     func() error { return nc.FlushTimeout(2 * time.Second) },
	}))
	eventHandler := events.NewHandler(db, metrics)
	mux.Handle("/v1/events", eventHandler)
	mux.Handle("/v1/events/", eventHandler)
	mux.Handle("/v1/users/", points.NewHandler(db))
	mux.Handle("/v1/points/conversions", points.NewConversionHandler(db))
	rewardProvider := rewards.NewProvider(rewards.ProviderConfig{
		Kind:      cfg.RewardProvider,
		URL:       cfg.RewardProviderURL,
		StatusURL: cfg.RewardProviderStatusURL,
		Token:     cfg.RewardProviderToken,
		Timeout:   time.Duration(cfg.RewardProviderTimeoutSeconds) * time.Second,
	})
	rewardResolver := rewards.ScopeProviderResolver{Store: rewards.NewConfigStore(db, cfg.RewardProviderEncryptionKey), Fallback: rewardProvider}
	rewardHandler := rewards.NewHandlerWithResolver(db, rewardResolver, metrics)
	mux.Handle("/v1/rewards/", rewardHandler)
	mux.Handle("/v1/redemptions/", rewardHandler)
	mux.Handle("/v1/admin/", admin.NewHandlerWithProviderConfig(db, cfg.AdminAPIKey, adminauth.NewStore(db), rewards.NewConfigStore(db, cfg.RewardProviderEncryptionKey)))
	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           transport.Protect(mux, transport.ProtectionConfig{Logger: logger, AllowedOrigins: cfg.CORSOrigins, Limiter: transport.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst), Metrics: metrics}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "addr", cfg.APIAddr)
		if serveErr := server.ListenAndServe(); !errors.Is(serveErr, http.ErrServerClosed) {
			serverErr <- serveErr
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverErr:
		return err
	}
}
