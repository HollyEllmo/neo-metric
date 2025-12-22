-- +goose Up
-- +goose StatementBegin

-- Message type enum for DM messages
CREATE TYPE dm_message_type AS ENUM ('text', 'image', 'video', 'audio', 'link', 'story_mention', 'story_reply');

-- Template type enum (shared between direct and comments)
CREATE TYPE template_type AS ENUM ('direct', 'comment', 'both');

-- DM Conversations table
CREATE TABLE dm_conversations (
    id VARCHAR(64) PRIMARY KEY,  -- Instagram conversation/thread ID
    account_id BIGINT NOT NULL REFERENCES instagram_accounts(id) ON DELETE CASCADE,
    participant_id VARCHAR(64) NOT NULL,  -- Instagram user ID of the other participant
    participant_username VARCHAR(255),
    participant_name VARCHAR(255),
    participant_avatar_url TEXT,
    participant_followers_count INT,
    last_message_text TEXT,
    last_message_at TIMESTAMP,
    last_message_is_from_me BOOLEAN,
    unread_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- DM Messages table
CREATE TABLE dm_messages (
    id VARCHAR(64) PRIMARY KEY,  -- Instagram message ID
    conversation_id VARCHAR(64) NOT NULL REFERENCES dm_conversations(id) ON DELETE CASCADE,
    sender_id VARCHAR(64) NOT NULL,  -- Instagram user ID of sender
    message_type dm_message_type NOT NULL DEFAULT 'text',
    text TEXT,
    media_url TEXT,
    media_type VARCHAR(32),  -- Specific type for media messages (image/video/audio)
    is_unsent BOOLEAN NOT NULL DEFAULT false,
    is_from_me BOOLEAN NOT NULL DEFAULT false,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Conversation sync status (per conversation, for message sync)
CREATE TABLE dm_conversation_sync_status (
    conversation_id VARCHAR(64) PRIMARY KEY REFERENCES dm_conversations(id) ON DELETE CASCADE,
    last_synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    next_cursor VARCHAR(512),
    sync_complete BOOLEAN NOT NULL DEFAULT false,
    oldest_message_timestamp TIMESTAMP  -- For incremental sync
);

-- Account sync status (per account, for conversations list sync)
CREATE TABLE dm_account_sync_status (
    account_id BIGINT PRIMARY KEY REFERENCES instagram_accounts(id) ON DELETE CASCADE,
    last_synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    next_cursor VARCHAR(512),
    sync_complete BOOLEAN NOT NULL DEFAULT false
);

-- Shared templates table (for Direct and Comments)
CREATE TABLE templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id BIGINT NOT NULL REFERENCES instagram_accounts(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    images TEXT[],  -- Array of image URLs
    icon VARCHAR(64),  -- Emoji or icon identifier
    type template_type NOT NULL DEFAULT 'both',
    usage_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for conversations
CREATE INDEX idx_dm_conversations_account_id ON dm_conversations(account_id);
CREATE INDEX idx_dm_conversations_participant_username ON dm_conversations(participant_username);
CREATE INDEX idx_dm_conversations_last_message_at ON dm_conversations(last_message_at DESC NULLS LAST);
CREATE INDEX idx_dm_conversations_updated_at ON dm_conversations(updated_at DESC);

-- Indexes for messages
CREATE INDEX idx_dm_messages_conversation_id ON dm_messages(conversation_id);
CREATE INDEX idx_dm_messages_timestamp ON dm_messages(timestamp DESC);
CREATE INDEX idx_dm_messages_sender_id ON dm_messages(sender_id);
CREATE INDEX idx_dm_messages_is_from_me ON dm_messages(is_from_me);

-- Indexes for templates
CREATE INDEX idx_templates_account_id ON templates(account_id);
CREATE INDEX idx_templates_type ON templates(type);
CREATE INDEX idx_templates_usage_count ON templates(usage_count DESC);

-- Full-text search index on conversations (for search by participant name/username)
CREATE INDEX idx_dm_conversations_search ON dm_conversations
    USING GIN (to_tsvector('simple', COALESCE(participant_username, '') || ' ' || COALESCE(participant_name, '')));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_dm_conversations_search;
DROP INDEX IF EXISTS idx_templates_usage_count;
DROP INDEX IF EXISTS idx_templates_type;
DROP INDEX IF EXISTS idx_templates_account_id;
DROP INDEX IF EXISTS idx_dm_messages_is_from_me;
DROP INDEX IF EXISTS idx_dm_messages_sender_id;
DROP INDEX IF EXISTS idx_dm_messages_timestamp;
DROP INDEX IF EXISTS idx_dm_messages_conversation_id;
DROP INDEX IF EXISTS idx_dm_conversations_updated_at;
DROP INDEX IF EXISTS idx_dm_conversations_last_message_at;
DROP INDEX IF EXISTS idx_dm_conversations_participant_username;
DROP INDEX IF EXISTS idx_dm_conversations_account_id;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS dm_account_sync_status;
DROP TABLE IF EXISTS dm_conversation_sync_status;
DROP TABLE IF EXISTS dm_messages;
DROP TABLE IF EXISTS dm_conversations;
DROP TYPE IF EXISTS template_type;
DROP TYPE IF EXISTS dm_message_type;
-- +goose StatementEnd
