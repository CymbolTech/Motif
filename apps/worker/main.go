package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/config"
	"github.com/motifengine/motifengine/internal/health"
	"github.com/motifengine/motifengine/internal/processing"
	"github.com/motifengine/motifengine/internal/rewards"
	"github.com/motifengine/motifengine/internal/runtime"
	"github.com/nats-io/nats.go"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker stopped", "error", err)
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
	nc, err := nats.Connect(cfg.NATSURL, nats.Name("motifengine-worker"))
	if err != nil {
		return err
	}
	defer nc.Drain()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	if err := processing.New(db, js, logger).Start(ctx); err != nil {
		return err
	}
	rewardProvider := rewards.NewProvider(rewards.ProviderConfig{
		Kind:      cfg.RewardProvider,
		URL:       cfg.RewardProviderURL,
		StatusURL: cfg.RewardProviderStatusURL,
		Token:     cfg.RewardProviderToken,
		Timeout:   time.Duration(cfg.RewardProviderTimeoutSeconds) * time.Second,
	})
	rewardResolver := rewards.ScopeProviderResolver{Store: rewards.NewConfigStore(db, cfg.RewardProviderEncryptionKey), Fallback: rewardProvider}
	rewards.NewReconcilerWithResolver(db, rewardResolver, logger).Start(ctx)

	server := &http.Server{Addr: cfg.WorkerHealthAddr, Handler: health.NewHandler(health.Dependencies{
		Database: func() error { return db.Ping(context.Background()) },
		NATS:     func() error { return nc.FlushTimeout(2 * time.Second) },
	}), ReadHeaderTimeout: 5 * time.Second}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("worker ready", "health_addr", cfg.WorkerHealthAddr, "stream", "MOTIF_EVENTS")
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
