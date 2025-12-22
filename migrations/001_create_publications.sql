-- +goose Up
-- +goose StatementBegin

-- Publication types enum
CREATE TYPE publication_type AS ENUM ('post', 'story', 'reel');

-- Publication status enum
CREATE TYPE publication_status AS ENUM ('draft', 'scheduled', 'published', 'error');

-- Media type enum
CREATE TYPE media_type AS ENUM ('image', 'video');

-- Publications table
CREATE TABLE publications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id BIGINT NOT NULL REFERENCES instagram_accounts(id) ON DELETE CASCADE,
    instagram_media_id VARCHAR(255),
    type publication_type NOT NULL,
    status publication_status NOT NULL DEFAULT 'draft',
    caption TEXT,
    scheduled_at TIMESTAMP,
    published_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Media items table
CREATE TABLE publication_media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    publication_id UUID NOT NULL REFERENCES publications(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    type media_type NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_publications_account_id ON publications(account_id);
CREATE INDEX idx_publications_status ON publications(status);
CREATE INDEX idx_publications_scheduled_at ON publications(scheduled_at) WHERE status = 'scheduled';
CREATE INDEX idx_publications_type ON publications(type);
CREATE INDEX idx_publication_media_publication_id ON publication_media(publication_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS publication_media;
DROP TABLE IF EXISTS publications;
DROP TYPE IF EXISTS media_type;
DROP TYPE IF EXISTS publication_status;
DROP TYPE IF EXISTS publication_type;
-- +goose StatementEnd
