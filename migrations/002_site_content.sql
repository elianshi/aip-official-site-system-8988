CREATE TABLE site_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version INTEGER NOT NULL UNIQUE CHECK (version > 0),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    brand JSONB NOT NULL CHECK (jsonb_typeof(brand) = 'object' AND brand ? 'name' AND jsonb_typeof(brand->'name') = 'string'),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status <> 'published' OR published_at IS NOT NULL)
);

-- A publish transaction must archive the old release before publishing the new one.
CREATE UNIQUE INDEX one_published_release ON site_releases (status) WHERE status = 'published';

CREATE TABLE content_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID NOT NULL REFERENCES site_releases(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('banners', 'navigation', 'heroes', 'products', 'solutions', 'cases', 'services', 'security', 'pricing', 'resources', 'footer')),
    title VARCHAR(256) NOT NULL CHECK (length(btrim(title)) > 0),
    summary TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    image JSONB CHECK (image IS NULL OR jsonb_typeof(image) = 'object'),
    cta JSONB CHECK (cta IS NULL OR jsonb_typeof(cta) = 'object'),
    links JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(links) = 'array'),
    features JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(features) = 'array'),
    price JSONB CHECK (price IS NULL OR jsonb_typeof(price) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX published_content_order ON content_items (release_id, kind, position, id) WHERE enabled = TRUE;
