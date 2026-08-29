// Command server runs the pornhub.singles backend: JSON API, uploaded media
// and the embedded Angular bundle, all from a single binary.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pornhub.singles/server/internal/config"
	"pornhub.singles/server/internal/httpx"
	"pornhub.singles/server/internal/logging"
	"pornhub.singles/server/internal/store"
)

func main() {
	// Account management runs from the same binary, because the image has
	// nothing else in it: `docker compose exec app phs-server user list`.
	if len(os.Args) > 1 && os.Args[1] == "user" {
		os.Exit(runUserCLI(os.Args[2:]))
	}

	// The container image has no shell, so the binary doubles as its own
	// Docker healthcheck: `phs-server -healthcheck`.
	healthcheck := flag.Bool("healthcheck", false, "probe the local server and exit")
	flag.Parse()

	if *healthcheck {
		if err := probe(); err != nil {
			fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		slog.Default().Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logging.New(os.Stdout, cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)

	// Cancelled on SIGINT/SIGTERM: the signal starts the graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer st.Close()

	applied, err := st.AppliedMigrations(ctx)
	if err != nil {
		return err
	}
	log.Info("database ready", "path", cfg.DatabasePath, "migrations", len(applied))

	created, reset, err := st.EnsureAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword, cfg.ForcePassword)
	if err != nil {
		return err
	}
	switch {
	case created:
		log.Info("bootstrap admin account created", "username", cfg.AdminUsername)
	case reset:
		log.Warn("admin password reset from environment", "username", cfg.AdminUsername)
	}

	srv, err := httpx.New(ctx, cfg, st, log)
	if err != nil {
		return err
	}
	go srv.Maintain(ctx)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 16,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", cfg.Addr, "public_url", cfg.PublicURL)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		stop() // restore default signal handling: a second SIGINT kills us
		log.Info("shutdown signal received", "grace", cfg.ShutdownTimeout.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed, closing connections", "error", err)
		httpServer.Close()
	}
	if err := <-serveErr; err != nil {
		return err
	}
	log.Info("server stopped")
	return nil
}

// probe performs the readiness request behind the -healthcheck flag.
func probe() error {
	addr := os.Getenv("PHS_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = strings.TrimPrefix(addr, ":")
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/api/ready")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
