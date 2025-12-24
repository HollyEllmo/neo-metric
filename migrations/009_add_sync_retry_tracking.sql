-- +goose Up
-- +goose StatementBegin

-- Add retry tracking columns to comment_sync_status
ALTER TABLE comment_sync_status
ADD COLUMN retry_count INT NOT NULL DEFAULT 0,
ADD COLUMN failed BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN last_error TEXT;

-- Add retry tracking columns to dm_conversation_sync_status
ALTER TABLE dm_conversation_sync_status
ADD COLUMN retry_count INT NOT NULL DEFAULT 0,
ADD COLUMN failed BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN last_error TEXT;

-- Add retry tracking columns to dm_account_sync_status
ALTER TABLE dm_account_sync_status
ADD COLUMN retry_count INT NOT NULL DEFAULT 0,
ADD COLUMN failed BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN last_error TEXT;

-- Create indexes for efficient queries that exclude failed syncs
CREATE INDEX idx_comment_sync_status_not_failed ON comment_sync_status(last_synced_at) WHERE NOT failed;
CREATE INDEX idx_dm_conversation_sync_not_failed ON dm_conversation_sync_status(last_synced_at) WHERE NOT failed;
CREATE INDEX idx_dm_account_sync_not_failed ON dm_account_sync_status(last_synced_at) WHERE NOT failed;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_dm_account_sync_not_failed;
DROP INDEX IF EXISTS idx_dm_conversation_sync_not_failed;
DROP INDEX IF EXISTS idx_comment_sync_status_not_failed;

ALTER TABLE dm_account_sync_status
DROP COLUMN IF EXISTS last_error,
DROP COLUMN IF EXISTS failed,
DROP COLUMN IF EXISTS retry_count;

ALTER TABLE dm_conversation_sync_status
DROP COLUMN IF EXISTS last_error,
DROP COLUMN IF EXISTS failed,
DROP COLUMN IF EXISTS retry_count;

ALTER TABLE comment_sync_status
DROP COLUMN IF EXISTS last_error,
DROP COLUMN IF EXISTS failed,
DROP COLUMN IF EXISTS retry_count;

-- +goose StatementEnd
