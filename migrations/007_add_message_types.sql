-- +goose Up
-- +goose StatementBegin

-- Add new message types to enum
ALTER TYPE dm_message_type ADD VALUE IF NOT EXISTS 'share';
ALTER TYPE dm_message_type ADD VALUE IF NOT EXISTS 'unknown';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: PostgreSQL doesn't support removing values from enums easily
-- The values will remain but won't be used

-- +goose StatementEnd
