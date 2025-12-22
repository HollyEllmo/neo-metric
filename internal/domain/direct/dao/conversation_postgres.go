package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vadim/neo-metric/internal/domain/direct/entity"
)

// ConversationPostgres implements conversation repository for PostgreSQL
type ConversationPostgres struct {
	pool *pgxpool.Pool
}

// NewConversationPostgres creates a new PostgreSQL conversation repository
func NewConversationPostgres(pool *pgxpool.Pool) *ConversationPostgres {
	return &ConversationPostgres{pool: pool}
}

// Upsert inserts or updates a conversation
func (r *ConversationPostgres) Upsert(ctx context.Context, conv *entity.Conversation) error {
	query := `
		INSERT INTO dm_conversations (
			id, account_id, participant_id, participant_username, participant_name,
			participant_avatar_url, participant_followers_count, last_message_text,
			last_message_at, last_message_is_from_me, unread_count, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			participant_username = EXCLUDED.participant_username,
			participant_name = EXCLUDED.participant_name,
			participant_avatar_url = EXCLUDED.participant_avatar_url,
			participant_followers_count = EXCLUDED.participant_followers_count,
			last_message_text = EXCLUDED.last_message_text,
			last_message_at = EXCLUDED.last_message_at,
			last_message_is_from_me = EXCLUDED.last_message_is_from_me,
			unread_count = EXCLUDED.unread_count,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	_, err := r.pool.Exec(ctx, query,
		conv.ID,
		conv.AccountID,
		conv.ParticipantID,
		conv.ParticipantUsername,
		conv.ParticipantName,
		conv.ParticipantAvatarURL,
		conv.ParticipantFollowersCount,
		conv.LastMessageText,
		conv.LastMessageAt,
		conv.LastMessageIsFromMe,
		conv.UnreadCount,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("upserting conversation: %w", err)
	}

	return nil
}

// UpsertBatch inserts or updates multiple conversations
func (r *ConversationPostgres) UpsertBatch(ctx context.Context, convs []entity.Conversation) error {
	if len(convs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO dm_conversations (
			id, account_id, participant_id, participant_username, participant_name,
			participant_avatar_url, participant_followers_count, last_message_text,
			last_message_at, last_message_is_from_me, unread_count, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (id) DO UPDATE SET
			participant_username = EXCLUDED.participant_username,
			participant_name = EXCLUDED.participant_name,
			participant_avatar_url = EXCLUDED.participant_avatar_url,
			participant_followers_count = EXCLUDED.participant_followers_count,
			last_message_text = EXCLUDED.last_message_text,
			last_message_at = EXCLUDED.last_message_at,
			last_message_is_from_me = EXCLUDED.last_message_is_from_me,
			unread_count = EXCLUDED.unread_count,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	for _, conv := range convs {
		batch.Queue(query,
			conv.ID,
			conv.AccountID,
			conv.ParticipantID,
			conv.ParticipantUsername,
			conv.ParticipantName,
			conv.ParticipantAvatarURL,
			conv.ParticipantFollowersCount,
			conv.LastMessageText,
			conv.LastMessageAt,
			conv.LastMessageIsFromMe,
			conv.UnreadCount,
			now,
			now,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range convs {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("executing batch upsert: %w", err)
		}
	}

	return nil
}

// GetByID retrieves a conversation by ID
func (r *ConversationPostgres) GetByID(ctx context.Context, id string) (*entity.Conversation, error) {
	query := `
		SELECT id, account_id, participant_id, participant_username, participant_name,
		       participant_avatar_url, participant_followers_count, last_message_text,
		       last_message_at, last_message_is_from_me, unread_count, created_at, updated_at
		FROM dm_conversations
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanConversation(row)
}

// GetByAccountID retrieves conversations for an account with pagination
func (r *ConversationPostgres) GetByAccountID(ctx context.Context, accountID string, limit, offset int) ([]entity.Conversation, error) {
	query := `
		SELECT id, account_id, participant_id, participant_username, participant_name,
		       participant_avatar_url, participant_followers_count, last_message_text,
		       last_message_at, last_message_is_from_me, unread_count, created_at, updated_at
		FROM dm_conversations
		WHERE account_id = $1
		ORDER BY last_message_at DESC NULLS LAST, updated_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying conversations: %w", err)
	}
	defer rows.Close()

	return r.scanConversations(rows)
}

// Search searches conversations by participant username or name
func (r *ConversationPostgres) Search(ctx context.Context, accountID, query string, limit, offset int) ([]entity.Conversation, error) {
	sqlQuery := `
		SELECT id, account_id, participant_id, participant_username, participant_name,
		       participant_avatar_url, participant_followers_count, last_message_text,
		       last_message_at, last_message_is_from_me, unread_count, created_at, updated_at
		FROM dm_conversations
		WHERE account_id = $1
		  AND to_tsvector('simple', COALESCE(participant_username, '') || ' ' || COALESCE(participant_name, ''))
		      @@ plainto_tsquery('simple', $2)
		ORDER BY last_message_at DESC NULLS LAST
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, sqlQuery, accountID, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("searching conversations: %w", err)
	}
	defer rows.Close()

	return r.scanConversations(rows)
}

// Delete removes a conversation
func (r *ConversationPostgres) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM dm_conversations WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting conversation: %w", err)
	}
	return nil
}

// Count returns the total count of conversations for an account
func (r *ConversationPostgres) Count(ctx context.Context, accountID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM dm_conversations WHERE account_id = $1", accountID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting conversations: %w", err)
	}
	return count, nil
}

// scanConversation scans a single conversation row
func (r *ConversationPostgres) scanConversation(row pgx.Row) (*entity.Conversation, error) {
	var conv entity.Conversation
	var lastMessageAt *time.Time

	err := row.Scan(
		&conv.ID,
		&conv.AccountID,
		&conv.ParticipantID,
		&conv.ParticipantUsername,
		&conv.ParticipantName,
		&conv.ParticipantAvatarURL,
		&conv.ParticipantFollowersCount,
		&conv.LastMessageText,
		&lastMessageAt,
		&conv.LastMessageIsFromMe,
		&conv.UnreadCount,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning conversation: %w", err)
	}

	conv.LastMessageAt = lastMessageAt
	return &conv, nil
}

// scanConversations scans multiple conversation rows
func (r *ConversationPostgres) scanConversations(rows pgx.Rows) ([]entity.Conversation, error) {
	var conversations []entity.Conversation

	for rows.Next() {
		var conv entity.Conversation
		var lastMessageAt *time.Time

		err := rows.Scan(
			&conv.ID,
			&conv.AccountID,
			&conv.ParticipantID,
			&conv.ParticipantUsername,
			&conv.ParticipantName,
			&conv.ParticipantAvatarURL,
			&conv.ParticipantFollowersCount,
			&conv.LastMessageText,
			&lastMessageAt,
			&conv.LastMessageIsFromMe,
			&conv.UnreadCount,
			&conv.CreatedAt,
			&conv.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning conversation row: %w", err)
		}

		conv.LastMessageAt = lastMessageAt
		conversations = append(conversations, conv)
	}

	return conversations, nil
}
