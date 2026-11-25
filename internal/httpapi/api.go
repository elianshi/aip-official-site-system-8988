package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"official-site-system/internal/domain"
)

type Repository interface {
	Published(context.Context) (domain.Site, error)
	Ready(context.Context) error
}

type API struct {
	repo    Repository
	logger  *slog.Logger
	origins map[string]bool
}

func New(repo Repository, logger *slog.Logger, origins []string) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	a := &API{repo: repo, logger: logger, origins: make(map[string]bool)}
	for _, origin := range origins {
		a.origins[origin] = true
	}
	return a
}

type errorResponse struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if a.origins[origin] {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	kind := domain.Kind(strings.TrimPrefix(r.URL.Path, "/api/v1/contents/"))
	isSection := strings.HasPrefix(r.URL.Path, "/api/v1/contents/")
	if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && r.URL.Path != "/api/v1/site" && !isSection {
		fail(w, 404, "not_found", "Route not found")
		return
	}
	if isSection && !kind.Valid() {
		fail(w, 400, "invalid_kind", "Unknown content kind")
		return
	}
	if r.Method == http.MethodOptions {
		if !a.origins[origin] {
			fail(w, 403, "origin_not_allowed", "Origin not allowed")
			return
		}
		if method := r.Header.Get("Access-Control-Request-Method"); method != "GET" && method != "HEAD" {
			fail(w, 405, "method_not_allowed", "Only GET and HEAD are supported")
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		fail(w, 405, "method_not_allowed", "Only GET and HEAD are supported")
		return
	}
	if r.Method == http.MethodHead {
		w = headWriter{w}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	switch r.URL.Path {
	case "/healthz":
		writeJSON(w, 200, map[string]string{"status": "ok"})
	case "/readyz":
		if err := a.repo.Ready(ctx); err != nil {
			a.logger.Error("readiness failed")
			fail(w, 503, "not_ready", "Database is unavailable or migrations are missing")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	default:
		site, err := a.repo.Published(ctx)
		if errors.Is(err, domain.ErrNotPublished) {
			fail(w, 404, "not_published", "No site has been published")
			return
		}
		if err != nil {
			a.logger.Error("published content read failed")
			fail(w, 503, "content_unavailable", "Published content is temporarily unavailable")
			return
		}
		if isSection {
			items := site.Sections[kind]
			if items == nil {
				items = []domain.Content{}
			}
			writeJSON(w, 200, struct {
				Data      []domain.Content `json:"data"`
				ReleaseID string           `json:"release_id"`
				Version   int              `json:"version"`
			}{items, site.ReleaseID, site.Version})
			return
		}
		writeJSON(w, 200, struct {
			Data domain.Site `json:"data"`
		}{site})
	}
}

type headWriter struct{ http.ResponseWriter }

func (w headWriter) Write(b []byte) (int, error) { return len(b), nil }
