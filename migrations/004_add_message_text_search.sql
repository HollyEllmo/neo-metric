-- +goose Up
-- +goose StatementBegin

-- Add GIN index for full-text search on message text
CREATE INDEX idx_dm_messages_text_search ON dm_messages
    USING GIN (to_tsvector('simple', COALESCE(text, '')));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_dm_messages_text_search;

-- +goose StatementEnd
