package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/motifengine/motifengine/internal/config"
	"github.com/motifengine/motifengine/internal/mcpserver"
	"github.com/motifengine/motifengine/internal/runtime"
)

const version = "0.2.0"

func main() {
	stdio := flag.Bool("stdio", false, "serve one session over standard input and output, authenticated by MOTIF_ACCESS_TOKEN")
	flag.Parse()
	// Standard output carries the protocol in stdio mode, so logs always go to standard error.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger, *stdio); err != nil {
		logger.Error("mcp stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, stdio bool) error {
	cfg, err := config.LoadMCP()
	if err != nil {
		return err
	}
	ctx, stop := runtime.SignalContext(context.Background())
	defer stop()
	opts := mcpserver.Options{
		APIURL: cfg.APIURL, ConsoleURL: cfg.ConsoleURL, PublicAPIURL: cfg.PublicAPIURL, Version: version, Logger: logger,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
	}
	if stdio {
		token := os.Getenv("MOTIF_ACCESS_TOKEN")
		if token == "" {
			return errors.New("MOTIF_ACCESS_TOKEN is required with -stdio")
		}
		return mcpserver.RunStdio(ctx, opts, token)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.NewHTTPHandler(opts))
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := apiReady(r.Context(), opts.HTTPClient, cfg.APIURL); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Test events wait for processing, so responses can take several seconds.
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("mcp listening", "addr", cfg.Addr, "api", cfg.APIURL)
		if serveErr := server.ListenAndServe(); !errors.Is(serveErr, http.ErrServerClosed) {
			serverErr <- serveErr
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverErr:
		return err
	}
}

func apiReady(ctx context.Context, client *http.Client, apiURL string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/health/ready", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("api unreachable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("api not ready: %d", response.StatusCode)
	}
	return nil
}
