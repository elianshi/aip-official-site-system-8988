package store

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"official-site-system/internal/domain"
	"official-site-system/migrations"
)

// Each test owns a separate schema; never truncates or modifies existing tables.
func integrationDB(t *testing.T) (*Postgres, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("test database connection failed: %v", err)
	}
	schema := fmt.Sprintf("sprint1_test_%d", time.Now().UnixNano())
	if _, err = admin.DB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.DB.Close()
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	repo, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		repo.DB.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.DB.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("cleanup: %v", err)
		}
		admin.DB.Close()
	})
	return repo, u.String()
}

func TestPostgresIntegration(t *testing.T) {
	repo, dsn := integrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := repo.Ready(ctx); err == nil {
		t.Fatal("unmigrated schema reported ready")
	}
	if err := migrations.Apply(ctx, repo.DB, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Published(ctx); err != domain.ErrNotPublished {
		t.Fatalf("unpublished: %v", err)
	}
	// Concurrent initializations must produce exactly one complete seed release.
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := migrations.Apply(ctx, repo.DB, true); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	site, err := repo.Published(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if site.Version != 1 || site.Brand.Name != "协作云" || len(site.Sections) != 11 || len(site.Sections[domain.Product]) != 2 {
		t.Fatalf("seed: %+v", site)
	}
	for _, kind := range domain.Kinds() {
		if len(site.Sections[kind]) == 0 {
			t.Errorf("no seed for %s", kind)
		}
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := repo.DB.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO site_releases(id,version,status,brand) VALUES ('20000000-0000-0000-0000-000000000001',2,'draft','{"name":"PRIVATE DRAFT"}')`)
	exec(`INSERT INTO content_items(release_id,kind,title) VALUES ('20000000-0000-0000-0000-000000000001','heroes','PRIVATE DRAFT')`)
	exec(`INSERT INTO content_items(release_id,kind,title,enabled) VALUES ($1,'heroes','DISABLED',false)`, site.ReleaseID)
	exec(`INSERT INTO content_items(release_id,kind,title,position) VALUES ($1,'products','Last',99)`, site.ReleaseID)
	exec(`INSERT INTO content_items(id,release_id,kind,title,position) VALUES ('00000000-0000-0000-0000-000000000002',$1,'products','Tie B',50),('00000000-0000-0000-0000-000000000001',$1,'products','Tie A',50)`, site.ReleaseID)
	site, err = repo.Published(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if site.Brand.Name == "PRIVATE DRAFT" || len(site.Sections[domain.Hero]) != 2 {
		t.Fatal("draft or disabled data leaked")
	}
	products := site.Sections[domain.Product]
	if len(products) != 5 || products[2].Title != "Tie A" || products[3].Title != "Tie B" || products[4].Title != "Last" {
		t.Fatalf("unstable ordering: %+v", products)
	}
	// Reopening the storage connection and rerunning seeds preserves edited data.
	exec(`UPDATE content_items SET title='Persisted title' WHERE id=$1`, products[0].ID)
	reopened, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.DB.Close()
	if err = migrations.Apply(ctx, reopened.DB, true); err != nil {
		t.Fatal(err)
	}
	reloaded, err := reopened.Published(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Sections[domain.Product][0].Title != "Persisted title" {
		t.Fatal("data did not persist or seed overwrote edits")
	}
	// Database invariants reject invalid kinds, positions, states and double publish.
	for _, query := range []string{
		`INSERT INTO content_items(release_id,kind,title) VALUES ('10000000-0000-0000-0000-000000000001','unknown','invalid')`,
		`INSERT INTO content_items(release_id,kind,title,position) VALUES ('10000000-0000-0000-0000-000000000001','products','invalid',-1)`,
		`INSERT INTO content_items(release_id,kind,title) VALUES ('10000000-0000-0000-0000-000000000001','products',' ')`,
		`UPDATE site_releases SET status='unknown' WHERE version=2`,
		`UPDATE site_releases SET status='published' WHERE version=2`,
		`UPDATE site_releases SET status='published',published_at=NOW() WHERE version=2`,
	} {
		if _, err := repo.DB.ExecContext(ctx, query); err == nil {
			t.Errorf("constraint accepted %s", query)
		}
	}
	// A transaction swaps release snapshots; old content becomes invisible.
	tx, err := repo.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE site_releases SET status='archived' WHERE version=1; UPDATE site_releases SET status='published',published_at=NOW() WHERE version=2`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	site, err = repo.Published(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if site.Version != 2 || len(site.Sections[domain.Product]) != 0 || len(site.Sections[domain.Hero]) != 1 {
		t.Fatal("release swap leaked old content")
	}
	exec(`UPDATE content_items SET enabled=false WHERE release_id=$1`, site.ReleaseID)
	site, err = repo.Published(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, items := range site.Sections {
		if items == nil || len(items) != 0 {
			t.Fatal("empty release must return arrays")
		}
	}
	exec(`UPDATE schema_migrations SET checksum='tampered' WHERE name='002_site_content.sql'`)
	if err := migrations.Apply(ctx, repo.DB, false); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("checksum error=%v", err)
	}
}

func TestMigrationRollbackIntegration(t *testing.T) {
	repo, _ := integrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := repo.DB.ExecContext(ctx, `CREATE TABLE site_releases (id INT)`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, repo.DB, false); err == nil {
		t.Fatal("expected migration conflict")
	}
	var exists bool
	if err := repo.DB.QueryRowContext(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("failed migration left partial schema")
	}
}
