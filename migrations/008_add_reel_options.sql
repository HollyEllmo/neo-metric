-- +goose Up
-- +goose StatementBegin

-- Add reel_options column to publications table
-- This stores optional settings for Reel publishing as JSONB
ALTER TABLE publications ADD COLUMN IF NOT EXISTS reel_options JSONB;

-- Add comment for documentation
COMMENT ON COLUMN publications.reel_options IS 'Optional settings for Reel publishing (share_to_feed, cover_url, thumb_offset, audio_name, location_id, collaborator_usernames)';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE publications DROP COLUMN IF EXISTS reel_options;

-- +goose StatementEnd
