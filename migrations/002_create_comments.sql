-- +goose Up
-- +goose StatementBegin

-- Comments table stores Instagram comments with full sync support
CREATE TABLE comments (
    id VARCHAR(64) PRIMARY KEY,  -- Instagram comment ID
    instagram_media_id VARCHAR(255) NOT NULL,  -- Instagram media ID this comment belongs to
    parent_id VARCHAR(64) REFERENCES comments(id) ON DELETE CASCADE,  -- For replies
    username VARCHAR(255) NOT NULL,
    text TEXT NOT NULL,
    like_count INT NOT NULL DEFAULT 0,
    is_hidden BOOLEAN NOT NULL DEFAULT false,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Sync tracking table for comment synchronization
CREATE TABLE comment_sync_status (
    instagram_media_id VARCHAR(255) PRIMARY KEY,
    last_synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    next_cursor VARCHAR(512),  -- For pagination during sync
    sync_complete BOOLEAN NOT NULL DEFAULT false
);

-- Indexes for efficient queries
CREATE INDEX idx_comments_media_id ON comments(instagram_media_id);
CREATE INDEX idx_comments_parent_id ON comments(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_comments_timestamp ON comments(timestamp DESC);
CREATE INDEX idx_comments_username ON comments(username);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS comment_sync_status;
DROP TABLE IF EXISTS comments;
-- +goose StatementEnd
