package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"official-site-system/internal/domain"
)

type fakeRepo struct {
	site          domain.Site
	err, readyErr error
}

func (f fakeRepo) Published(context.Context) (domain.Site, error) { return f.site, f.err }
func (f fakeRepo) Ready(context.Context) error                    { return f.readyErr }

func testHandler(repo Repository) http.Handler {
	return New(repo, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"http://localhost:4101"})
}

func TestRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		repo               fakeRepo
		status             int
		code               string
	}{
		{"live", "GET", "/healthz", fakeRepo{readyErr: errors.New("db down")}, 200, ""},
		{"ready", "GET", "/readyz", fakeRepo{}, 200, ""},
		{"not ready", "GET", "/readyz", fakeRepo{readyErr: errors.New("secret connection")}, 503, "not_ready"},
		{"site", "GET", "/api/v1/site", fakeRepo{}, 200, ""},
		{"empty section", "GET", "/api/v1/contents/products", fakeRepo{}, 200, ""},
		{"no release", "GET", "/api/v1/site", fakeRepo{err: domain.ErrNotPublished}, 404, "not_published"},
		{"read failed", "GET", "/api/v1/site", fakeRepo{err: errors.New("secret connection")}, 503, "content_unavailable"},
		{"cancelled", "GET", "/api/v1/site", fakeRepo{err: context.Canceled}, 503, "content_unavailable"},
		{"unknown section", "GET", "/api/v1/contents/unknown", fakeRepo{}, 400, "invalid_kind"},
		{"empty kind", "GET", "/api/v1/contents/", fakeRepo{}, 400, "invalid_kind"},
		{"not found", "GET", "/missing", fakeRepo{}, 404, "not_found"},
		{"no admin", "GET", "/api/v1/admin", fakeRepo{}, 404, "not_found"},
		{"no writes", "POST", "/api/v1/site", fakeRepo{}, 405, "method_not_allowed"},
		{"no deletes", "DELETE", "/api/v1/contents/products", fakeRepo{}, 405, "method_not_allowed"},
		{"head", "HEAD", "/healthz", fakeRepo{}, 200, ""},
		{"head error", "HEAD", "/readyz", fakeRepo{readyErr: errors.New("down")}, 503, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			testHandler(tc.repo).ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if !strings.Contains(w.Header().Get("Content-Type"), "application/json") || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("headers=%v", w.Header())
			}
			if tc.code != "" {
				var response errorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Error.Code != tc.code {
					t.Fatalf("error=%+v", response)
				}
			}
			if strings.Contains(w.Body.String(), "secret") {
				t.Fatal("internal error leaked")
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD returned body")
			}
			if tc.name == "empty section" && !strings.Contains(w.Body.String(), `"data":[]`) {
				t.Fatal("empty array expected")
			}
			if tc.status == 405 && w.Header().Get("Allow") != "GET, HEAD, OPTIONS" {
				t.Fatal("Allow header missing")
			}
		})
	}
}

func TestPublishedContract(t *testing.T) {
	site := domain.Site{ReleaseID: "release", Version: 2, PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Brand: domain.Brand{Name: "协作云"}, Sections: domain.EmptySections()}
	site.Sections[domain.Product] = []domain.Content{{ID: "p1", Kind: domain.Product, Title: "产品", Links: []domain.Link{}, Features: []string{}}}
	h := testHandler(fakeRepo{site: site})
	for _, kind := range domain.Kinds() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/contents/"+string(kind), nil))
		var result struct {
			Data      []domain.Content
			ReleaseID string `json:"release_id"`
			Version   int
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || result.Data == nil || result.Version != 2 || result.ReleaseID != "release" {
			t.Fatalf("%s: %s", kind, w.Body)
		}
		if kind == domain.Product && (len(result.Data) != 1 || result.Data[0].Title != "产品") {
			t.Fatal("product missing")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/site", nil))
	var result struct{ Data domain.Site }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Brand.Name != site.Brand.Name || len(result.Data.Sections) != 11 {
		t.Fatalf("site=%+v", result)
	}
}

func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		name, method, origin, requested string
		status                          int
		allowed                         bool
	}{
		{"allowed", "GET", "http://localhost:4101", "", 200, true},
		{"unlisted", "GET", "https://untrusted.test", "", 200, false},
		{"no origin", "GET", "", "", 200, false},
		{"preflight", "OPTIONS", "http://localhost:4101", "GET", 204, true},
		{"head preflight", "OPTIONS", "http://localhost:4101", "HEAD", 204, true},
		{"preflight denied", "OPTIONS", "https://untrusted.test", "GET", 403, false},
		{"write preflight", "OPTIONS", "http://localhost:4101", "POST", 405, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/api/v1/site", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Access-Control-Request-Method", tc.requested)
			w := httptest.NewRecorder()
			testHandler(fakeRepo{}).ServeHTTP(w, r)
			if w.Code != tc.status || (w.Header().Get("Access-Control-Allow-Origin") != "") != tc.allowed {
				t.Fatalf("status=%d headers=%v", w.Code, w.Header())
			}
			if tc.allowed && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("origin mismatch")
			}
			if tc.status == 204 && (w.Body.Len() != 0 || w.Header().Get("Access-Control-Allow-Methods") == "") {
				t.Fatal("bad preflight")
			}
		})
	}
}

type contextRepo struct{ t *testing.T }

func (f contextRepo) Published(ctx context.Context) (domain.Site, error) {
	f.t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 3*time.Second {
		f.t.Error("missing bounded deadline")
	}
	return domain.Site{}, ctx.Err()
}
func (f contextRepo) Ready(ctx context.Context) error { _, err := f.Published(ctx); return err }

func TestRequestCancellationAndConcurrency(t *testing.T) {
	h := New(contextRepo{t}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/site", nil).WithContext(ctx))
	if w.Code != 503 {
		t.Fatalf("cancel status=%d", w.Code)
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/readyz", nil))
		}()
	}
	wg.Wait()
}
