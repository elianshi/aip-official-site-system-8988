package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"official-site-system/internal/config"
	"official-site-system/internal/httpapi"
	"official-site-system/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, logger); err != nil {
		// DSNs may contain secrets, so driver errors are never logged verbatim.
		logger.Error("API stopped", "reason", err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	repo, err := store.Open(startup, cfg.DatabaseURL)
	if err != nil {
		return errors.New("database connection failed; check DATABASE_URL and database availability")
	}
	defer repo.DB.Close()
	if err = repo.Ready(startup); err != nil {
		return errors.New("database schema unavailable; run migrations first")
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return errors.New("HTTP listener failed; check HTTP_ADDR and port availability")
	}
	server := &http.Server{
		Handler:           httpapi.New(repo, logger, cfg.AllowedOrigins),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	logger.Info("API listening", "address", listener.Addr().String())
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed")
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return errors.New("HTTP shutdown timed out")
		}
		<-done
		logger.Info("API stopped gracefully")
		return nil
	}
}
