// Command vaxbook is the single VaxBook binary (SAD §4): it applies migrations at
// startup and serves the HTTP API.
//
//	vaxbook              run the server (configured by VAXBOOK_* env vars)
//	vaxbook healthcheck  exit 0 if the local server's /healthz answers 200
//	                     (for container health checks; the image has no curl)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fugo101/vaxbook/apps/api/internal/httpapi"
	"github.com/fugo101/vaxbook/apps/api/internal/identity"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/clock"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/config"
	"github.com/fugo101/vaxbook/apps/api/internal/platform/logging"
	"github.com/fugo101/vaxbook/apps/api/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "vaxbook:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logger := logging.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := store.Migrate(ctx, db, logger); err != nil {
		return err
	}
	client := store.NewClient(db)

	var mailer identity.Mailer
	switch cfg.Mailer {
	case "console": // config allows it only in dev
		mailer = identity.ConsoleMailer{Logger: logger}
	default:
		return fmt.Errorf("mailer %q is not implemented", cfg.Mailer)
	}

	handler, err := httpapi.NewHandler(httpapi.Options{
		AppHost:  cfg.AppHost,
		Identity: identity.New(client, mailer, clock.Real{}, cfg.OTPPepper),
		Ping:     db.PingContext,
		Logger:   logger,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", string(cfg.Env)))
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// healthcheck calls /healthz on the local listener.
func healthcheck() int {
	addr := os.Getenv("VAXBOOK_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	if host == "" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// The target is this process's own listener, from its own configuration.
	url := "http://" + net.JoinHostPort(host, port) + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // G704: not user-controlled
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: not user-controlled
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}
