package store

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"official-site-system/internal/domain"
)

func TestPublished(t *testing.T) {
	for _, tc := range []struct {
		name, brand, items string
		queryErr           error
		noRows, wantErr    bool
	}{
		{"content", `{"name":"协作云"}`, `[{"id":"p1","kind":"products","title":"Product"}]`, nil, false, false},
		{"empty", `{"name":"协作云"}`, `[]`, nil, false, false},
		{"unpublished", ``, ``, nil, true, true},
		{"query failure", ``, ``, errors.New("database unavailable"), false, true},
		{"invalid brand", `{`, `[]`, nil, false, true},
		{"invalid items", `{}`, `{`, nil, false, true},
		{"invalid type", `{}`, `[{"kind":"products","position":"wrong"}]`, nil, false, true},
		{"unknown kind", `{}`, `[{"kind":"unknown"}]`, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			query := mock.ExpectQuery(regexp.QuoteMeta(publishedQuery))
			if tc.queryErr != nil {
				query.WillReturnError(tc.queryErr)
			} else {
				rows := sqlmock.NewRows([]string{"id", "version", "published_at", "brand", "items"})
				if !tc.noRows {
					rows.AddRow("release", 1, time.Now(), []byte(tc.brand), []byte(tc.items))
				}
				query.WillReturnRows(rows)
			}
			site, err := (&Postgres{DB: db}).Published(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("site=%+v err=%v", site, err)
			}
			if tc.noRows && !errors.Is(err, domain.ErrNotPublished) {
				t.Fatal(err)
			}
			if !tc.wantErr {
				if len(site.Sections) != 11 {
					t.Fatal("missing empty sections")
				}
				if tc.name == "content" {
					item := site.Sections[domain.Product][0]
					if item.Title != "Product" || item.Links == nil || item.Features == nil {
						t.Fatalf("item=%+v", item)
					}
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReadiness(t *testing.T) {
	for _, expected := range []error{nil, errors.New("offline")} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		call := mock.ExpectExec("SELECT release_id, kind FROM content_items LIMIT 0")
		if expected != nil {
			call.WillReturnError(expected)
		} else {
			call.WillReturnResult(sqlmock.NewResult(0, 0))
		}
		if err := (&Postgres{DB: db}).Ready(context.Background()); !errors.Is(err, expected) {
			t.Fatalf("got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

func TestOpenRejectsInvalidDSN(t *testing.T) {
	if repo, err := Open(context.Background(), "postgres://%/db"); err == nil || repo != nil {
		t.Fatal("invalid DSN accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if repo, err := Open(ctx, "postgres://u:p@127.0.0.1:1/db?connect_timeout=1"); err == nil || repo != nil {
		t.Fatal("cancelled connection accepted")
	}
}
