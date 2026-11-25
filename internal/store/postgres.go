package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"official-site-system/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Postgres struct{ DB *sql.DB }

func Open(ctx context.Context, dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return &Postgres{DB: db}, nil
}

func (p *Postgres) Ready(ctx context.Context) error {
	// Verify both connectivity and the required schema, including an empty site.
	_, err := p.DB.ExecContext(ctx, "SELECT release_id, kind FROM content_items LIMIT 0")
	return err
}

// One SELECT gives a consistent release snapshot even during a concurrent publish.
const publishedQuery = `
SELECT r.id::text, r.version, r.published_at, r.brand,
       COALESCE(jsonb_agg(jsonb_build_object(
         'id', c.id, 'kind', c.kind, 'title', c.title, 'summary', c.summary,
         'position', c.position, 'image', c.image, 'cta', c.cta,
         'links', c.links, 'features', c.features, 'price', c.price
       ) ORDER BY c.position, c.id) FILTER (WHERE c.id IS NOT NULL), '[]'::jsonb)
FROM site_releases r
LEFT JOIN content_items c ON c.release_id = r.id AND c.enabled = TRUE
WHERE r.status = 'published'
GROUP BY r.id`

func (p *Postgres) Published(ctx context.Context) (domain.Site, error) {
	var site domain.Site
	var brand, contents []byte
	err := p.DB.QueryRowContext(ctx, publishedQuery).Scan(&site.ReleaseID, &site.Version, &site.PublishedAt, &brand, &contents)
	if err == sql.ErrNoRows {
		return site, domain.ErrNotPublished
	}
	if err != nil {
		return site, fmt.Errorf("read published site: %w", err)
	}
	if err = json.Unmarshal(brand, &site.Brand); err != nil {
		return site, fmt.Errorf("decode brand: %w", err)
	}
	var items []domain.Content
	if err = json.Unmarshal(contents, &items); err != nil {
		return site, fmt.Errorf("decode content: %w", err)
	}
	site.Sections = domain.EmptySections()
	for _, item := range items {
		if !item.Kind.Valid() {
			return domain.Site{}, fmt.Errorf("invalid stored content kind: %q", item.Kind)
		}
		if item.Links == nil {
			item.Links = []domain.Link{}
		}
		if item.Features == nil {
			item.Features = []string{}
		}
		site.Sections[item.Kind] = append(site.Sections[item.Kind], item)
	}
	return site, nil
}
