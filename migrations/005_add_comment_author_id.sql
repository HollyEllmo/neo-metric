-- +goose Up
-- +goose StatementBegin

-- Add author_id column to comments table for DM integration
ALTER TABLE comments ADD COLUMN author_id VARCHAR(64);

-- Create index for lookups by author
CREATE INDEX idx_comments_author_id ON comments(author_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_comments_author_id;
ALTER TABLE comments DROP COLUMN IF EXISTS author_id;

-- +goose StatementEnd
