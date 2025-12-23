-- +goose Up
-- +goose StatementBegin

-- Instagram message/conversation IDs can be very long (140+ characters)
-- Need to extend VARCHAR(64) columns to TEXT

-- First, drop foreign key constraints
ALTER TABLE dm_messages DROP CONSTRAINT IF EXISTS dm_messages_conversation_id_fkey;
ALTER TABLE dm_conversation_sync_status DROP CONSTRAINT IF EXISTS dm_conversation_sync_status_conversation_id_fkey;

-- Change dm_conversations.id and participant_id to TEXT
ALTER TABLE dm_conversations ALTER COLUMN id TYPE TEXT;
ALTER TABLE dm_conversations ALTER COLUMN participant_id TYPE TEXT;

-- Change dm_messages columns to TEXT
ALTER TABLE dm_messages ALTER COLUMN id TYPE TEXT;
ALTER TABLE dm_messages ALTER COLUMN conversation_id TYPE TEXT;
ALTER TABLE dm_messages ALTER COLUMN sender_id TYPE TEXT;

-- Change dm_conversation_sync_status.conversation_id to TEXT
ALTER TABLE dm_conversation_sync_status ALTER COLUMN conversation_id TYPE TEXT;

-- Recreate foreign key constraints
ALTER TABLE dm_messages
    ADD CONSTRAINT dm_messages_conversation_id_fkey
    FOREIGN KEY (conversation_id) REFERENCES dm_conversations(id) ON DELETE CASCADE;

ALTER TABLE dm_conversation_sync_status
    ADD CONSTRAINT dm_conversation_sync_status_conversation_id_fkey
    FOREIGN KEY (conversation_id) REFERENCES dm_conversations(id) ON DELETE CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Revert back to VARCHAR(64) - note: this may fail if data is too long
ALTER TABLE dm_messages DROP CONSTRAINT IF EXISTS dm_messages_conversation_id_fkey;
ALTER TABLE dm_conversation_sync_status DROP CONSTRAINT IF EXISTS dm_conversation_sync_status_conversation_id_fkey;

ALTER TABLE dm_conversations ALTER COLUMN id TYPE VARCHAR(64);
ALTER TABLE dm_conversations ALTER COLUMN participant_id TYPE VARCHAR(64);
ALTER TABLE dm_messages ALTER COLUMN id TYPE VARCHAR(64);
ALTER TABLE dm_messages ALTER COLUMN conversation_id TYPE VARCHAR(64);
ALTER TABLE dm_messages ALTER COLUMN sender_id TYPE VARCHAR(64);
ALTER TABLE dm_conversation_sync_status ALTER COLUMN conversation_id TYPE VARCHAR(64);

ALTER TABLE dm_messages
    ADD CONSTRAINT dm_messages_conversation_id_fkey
    FOREIGN KEY (conversation_id) REFERENCES dm_conversations(id) ON DELETE CASCADE;

ALTER TABLE dm_conversation_sync_status
    ADD CONSTRAINT dm_conversation_sync_status_conversation_id_fkey
    FOREIGN KEY (conversation_id) REFERENCES dm_conversations(id) ON DELETE CASCADE;

-- +goose StatementEnd
