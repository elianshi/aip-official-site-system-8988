package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"official-site-system/internal/store"
	"official-site-system/migrations"
)

func TestRunConfigurationAndConnectionErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	t.Setenv("HTTP_ADDR", "127.0.0.1:4101")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("DATABASE_URL", "")
	if err := run(context.Background(), logger); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("invalid config: %v", err)
	}
	t.Setenv("DATABASE_URL", "postgres://u:private-password@127.0.0.1:1/cms?connect_timeout=1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, logger); err == nil || strings.Contains(err.Error(), "private-password") {
		t.Fatalf("connection error: %v", err)
	}
}

func TestRunLifecycleIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for API startup and shutdown integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Close()
	schema := fmt.Sprintf("api_test_%d", time.Now().UnixNano())
	if _, err := admin.DB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.DB.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	t.Setenv("DATABASE_URL", u.String())
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("HTTP_ADDR", "127.0.0.1:4101")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := run(ctx, logger); err == nil || !strings.Contains(err.Error(), "schema unavailable") {
		t.Fatalf("schema error: %v", err)
	}
	repo, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.DB.Close()
	if err := migrations.Apply(ctx, repo.DB, true); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	t.Setenv("HTTP_ADDR", address)
	if err := run(ctx, logger); err == nil || !strings.Contains(err.Error(), "listener failed") {
		listener.Close()
		t.Fatalf("bind error: %v", err)
	}
	listener.Close()
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- run(runCtx, logger) }()
	client := &http.Client{Timeout: time.Second}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		response, err := client.Get("http://" + address + "/readyz")
		if err == nil {
			response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatalf("ready=%d", response.StatusCode)
			}
			break
		}
		select {
		case err := <-done:
			t.Fatalf("early exit: %v", err)
		case <-ctx.Done():
			t.Fatal("startup timed out")
		case <-ticker.C:
		}
	}
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
