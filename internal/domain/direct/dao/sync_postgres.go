package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConversationSyncStatus represents sync status for a conversation
type ConversationSyncStatus struct {
	ConversationID         string
	LastSyncedAt           time.Time
	NextCursor             string
	SyncComplete           bool
	OldestMessageTimestamp *time.Time
}

// AccountSyncStatus represents sync status for an account's conversations list
type AccountSyncStatus struct {
	AccountID    string
	LastSyncedAt time.Time
	NextCursor   string
	SyncComplete bool
}

// ConversationSyncPostgres implements conversation sync status repository
type ConversationSyncPostgres struct {
	pool *pgxpool.Pool
}

// NewConversationSyncPostgres creates a new conversation sync status repository
func NewConversationSyncPostgres(pool *pgxpool.Pool) *ConversationSyncPostgres {
	return &ConversationSyncPostgres{pool: pool}
}

// GetSyncStatus retrieves sync status for a conversation
func (r *ConversationSyncPostgres) GetSyncStatus(ctx context.Context, conversationID string) (*ConversationSyncStatus, error) {
	query := `
		SELECT conversation_id, last_synced_at, next_cursor, sync_complete, oldest_message_timestamp
		FROM dm_conversation_sync_status
		WHERE conversation_id = $1
	`

	var status ConversationSyncStatus
	var nextCursor *string
	var oldestTimestamp *time.Time

	err := r.pool.QueryRow(ctx, query, conversationID).Scan(
		&status.ConversationID,
		&status.LastSyncedAt,
		&nextCursor,
		&status.SyncComplete,
		&oldestTimestamp,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting conversation sync status: %w", err)
	}

	if nextCursor != nil {
		status.NextCursor = *nextCursor
	}
	status.OldestMessageTimestamp = oldestTimestamp

	return &status, nil
}

// UpdateSyncStatus updates or inserts sync status for a conversation
func (r *ConversationSyncPostgres) UpdateSyncStatus(ctx context.Context, status *ConversationSyncStatus) error {
	query := `
		INSERT INTO dm_conversation_sync_status (conversation_id, last_synced_at, next_cursor, sync_complete, oldest_message_timestamp)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (conversation_id) DO UPDATE SET
			last_synced_at = EXCLUDED.last_synced_at,
			next_cursor = EXCLUDED.next_cursor,
			sync_complete = EXCLUDED.sync_complete,
			oldest_message_timestamp = EXCLUDED.oldest_message_timestamp
	`

	var nextCursor *string
	if status.NextCursor != "" {
		nextCursor = &status.NextCursor
	}

	_, err := r.pool.Exec(ctx, query,
		status.ConversationID,
		status.LastSyncedAt,
		nextCursor,
		status.SyncComplete,
		status.OldestMessageTimestamp,
	)
	if err != nil {
		return fmt.Errorf("updating conversation sync status: %w", err)
	}

	return nil
}

// DeleteSyncStatus removes sync status for a conversation
func (r *ConversationSyncPostgres) DeleteSyncStatus(ctx context.Context, conversationID string) error {
	query := `DELETE FROM dm_conversation_sync_status WHERE conversation_id = $1`
	_, err := r.pool.Exec(ctx, query, conversationID)
	if err != nil {
		return fmt.Errorf("deleting conversation sync status: %w", err)
	}
	return nil
}

// AccountSyncPostgres implements account sync status repository
type AccountSyncPostgres struct {
	pool *pgxpool.Pool
}

// NewAccountSyncPostgres creates a new account sync status repository
func NewAccountSyncPostgres(pool *pgxpool.Pool) *AccountSyncPostgres {
	return &AccountSyncPostgres{pool: pool}
}

// GetSyncStatus retrieves sync status for an account
func (r *AccountSyncPostgres) GetSyncStatus(ctx context.Context, accountID string) (*AccountSyncStatus, error) {
	query := `
		SELECT account_id, last_synced_at, next_cursor, sync_complete
		FROM dm_account_sync_status
		WHERE account_id = $1
	`

	var status AccountSyncStatus
	var nextCursor *string

	err := r.pool.QueryRow(ctx, query, accountID).Scan(
		&status.AccountID,
		&status.LastSyncedAt,
		&nextCursor,
		&status.SyncComplete,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting account sync status: %w", err)
	}

	if nextCursor != nil {
		status.NextCursor = *nextCursor
	}

	return &status, nil
}

// UpdateSyncStatus updates or inserts sync status for an account
func (r *AccountSyncPostgres) UpdateSyncStatus(ctx context.Context, status *AccountSyncStatus) error {
	query := `
		INSERT INTO dm_account_sync_status (account_id, last_synced_at, next_cursor, sync_complete)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id) DO UPDATE SET
			last_synced_at = EXCLUDED.last_synced_at,
			next_cursor = EXCLUDED.next_cursor,
			sync_complete = EXCLUDED.sync_complete
	`

	var nextCursor *string
	if status.NextCursor != "" {
		nextCursor = &status.NextCursor
	}

	_, err := r.pool.Exec(ctx, query,
		status.AccountID,
		status.LastSyncedAt,
		nextCursor,
		status.SyncComplete,
	)
	if err != nil {
		return fmt.Errorf("updating account sync status: %w", err)
	}

	return nil
}

// GetAccountsNeedingSync returns accounts that need conversation list sync
func (r *AccountSyncPostgres) GetAccountsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	query := `
		SELECT ia.id::text
		FROM instagram_accounts ia
		LEFT JOIN dm_account_sync_status s ON ia.id = s.account_id
		WHERE s.account_id IS NULL
		   OR s.last_synced_at < $1
		ORDER BY COALESCE(s.last_synced_at, '1970-01-01'::timestamp) ASC
		LIMIT $2
	`

	threshold := time.Now().Add(-olderThan)
	rows, err := r.pool.Query(ctx, query, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("getting accounts needing sync: %w", err)
	}
	defer rows.Close()

	var accountIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning account id: %w", err)
		}
		accountIDs = append(accountIDs, id)
	}

	return accountIDs, nil
}

// GetConversationsNeedingSync returns conversations that need message sync for an account
func (r *ConversationSyncPostgres) GetConversationsNeedingSync(ctx context.Context, accountID string, olderThan time.Duration, limit int) ([]string, error) {
	query := `
		SELECT c.id
		FROM dm_conversations c
		LEFT JOIN dm_conversation_sync_status s ON c.id = s.conversation_id
		WHERE c.account_id = $1
		  AND (s.conversation_id IS NULL OR s.last_synced_at < $2)
		ORDER BY COALESCE(s.last_synced_at, '1970-01-01'::timestamp) ASC
		LIMIT $3
	`

	threshold := time.Now().Add(-olderThan)
	rows, err := r.pool.Query(ctx, query, accountID, threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("getting conversations needing sync: %w", err)
	}
	defer rows.Close()

	var conversationIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning conversation id: %w", err)
		}
		conversationIDs = append(conversationIDs, id)
	}

	return conversationIDs, nil
}
