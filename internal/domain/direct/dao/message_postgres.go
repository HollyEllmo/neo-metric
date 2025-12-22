package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vadim/neo-metric/internal/domain/direct/entity"
)

// MessagePostgres implements message repository for PostgreSQL
type MessagePostgres struct {
	pool *pgxpool.Pool
}

// NewMessagePostgres creates a new PostgreSQL message repository
func NewMessagePostgres(pool *pgxpool.Pool) *MessagePostgres {
	return &MessagePostgres{pool: pool}
}

// Upsert inserts or updates a message
func (r *MessagePostgres) Upsert(ctx context.Context, msg *entity.Message) error {
	query := `
		INSERT INTO dm_messages (
			id, conversation_id, sender_id, message_type, text,
			media_url, media_type, is_unsent, is_from_me, timestamp, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			text = EXCLUDED.text,
			is_unsent = EXCLUDED.is_unsent
	`

	_, err := r.pool.Exec(ctx, query,
		msg.ID,
		msg.ConversationID,
		msg.SenderID,
		msg.Type,
		msg.Text,
		msg.MediaURL,
		msg.MediaType,
		msg.IsUnsent,
		msg.IsFromMe,
		msg.Timestamp,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("upserting message: %w", err)
	}

	return nil
}

// UpsertBatch inserts or updates multiple messages
func (r *MessagePostgres) UpsertBatch(ctx context.Context, msgs []entity.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO dm_messages (
			id, conversation_id, sender_id, message_type, text,
			media_url, media_type, is_unsent, is_from_me, timestamp, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			text = EXCLUDED.text,
			is_unsent = EXCLUDED.is_unsent
	`

	now := time.Now()
	for _, msg := range msgs {
		batch.Queue(query,
			msg.ID,
			msg.ConversationID,
			msg.SenderID,
			msg.Type,
			msg.Text,
			msg.MediaURL,
			msg.MediaType,
			msg.IsUnsent,
			msg.IsFromMe,
			msg.Timestamp,
			now,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range msgs {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("executing batch upsert: %w", err)
		}
	}

	return nil
}

// GetByID retrieves a message by ID
func (r *MessagePostgres) GetByID(ctx context.Context, id string) (*entity.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, message_type, text,
		       media_url, media_type, is_unsent, is_from_me, timestamp, created_at
		FROM dm_messages
		WHERE id = $1
	`

	var msg entity.Message
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&msg.ID,
		&msg.ConversationID,
		&msg.SenderID,
		&msg.Type,
		&msg.Text,
		&msg.MediaURL,
		&msg.MediaType,
		&msg.IsUnsent,
		&msg.IsFromMe,
		&msg.Timestamp,
		&msg.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning message: %w", err)
	}

	return &msg, nil
}

// GetByConversationID retrieves messages for a conversation with pagination
func (r *MessagePostgres) GetByConversationID(ctx context.Context, conversationID string, limit, offset int) ([]entity.Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, message_type, text,
		       media_url, media_type, is_unsent, is_from_me, timestamp, created_at
		FROM dm_messages
		WHERE conversation_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, conversationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying messages: %w", err)
	}
	defer rows.Close()

	var messages []entity.Message
	for rows.Next() {
		var msg entity.Message
		err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.SenderID,
			&msg.Type,
			&msg.Text,
			&msg.MediaURL,
			&msg.MediaType,
			&msg.IsUnsent,
			&msg.IsFromMe,
			&msg.Timestamp,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning message row: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// Delete removes a message
func (r *MessagePostgres) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM dm_messages WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting message: %w", err)
	}
	return nil
}

// Count returns the total count of messages in a conversation
func (r *MessagePostgres) Count(ctx context.Context, conversationID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM dm_messages WHERE conversation_id = $1", conversationID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting messages: %w", err)
	}
	return count, nil
}

// GetStatistics calculates statistics for an account over a period
func (r *MessagePostgres) GetStatistics(ctx context.Context, filter entity.StatisticsFilter) (*entity.Statistics, error) {
	query := `
		WITH msg_stats AS (
			SELECT
				m.is_from_me,
				m.timestamp,
				c.id as conv_id,
				c.created_at as conv_created_at
			FROM dm_messages m
			JOIN dm_conversations c ON m.conversation_id = c.id
			WHERE c.account_id = $1
			  AND m.timestamp >= $2
			  AND m.timestamp <= $3
		),
		dialog_stats AS (
			SELECT
				COUNT(DISTINCT conv_id) as total_dialogs,
				COUNT(DISTINCT CASE WHEN conv_created_at >= $2 THEN conv_id END) as new_dialogs,
				COUNT(DISTINCT CASE WHEN NOT is_from_me THEN conv_id END) as unique_users
			FROM msg_stats
		),
		message_counts AS (
			SELECT
				COUNT(*) FILTER (WHERE is_from_me) as sent,
				COUNT(*) FILTER (WHERE NOT is_from_me) as received
			FROM msg_stats
		),
		busiest AS (
			SELECT
				EXTRACT(DOW FROM timestamp)::int as day,
				EXTRACT(HOUR FROM timestamp)::int as hour,
				COUNT(*) as cnt
			FROM msg_stats
			GROUP BY 1, 2
			ORDER BY cnt DESC
			LIMIT 1
		)
		SELECT
			COALESCE(d.total_dialogs, 0),
			COALESCE(d.new_dialogs, 0),
			COALESCE(d.unique_users, 0),
			COALESCE(mc.sent, 0),
			COALESCE(mc.received, 0),
			COALESCE(b.day, 0),
			COALESCE(b.hour, 0)
		FROM dialog_stats d
		CROSS JOIN message_counts mc
		LEFT JOIN busiest b ON true
	`

	var stats entity.Statistics
	err := r.pool.QueryRow(ctx, query, filter.AccountID, filter.StartDate, filter.EndDate).Scan(
		&stats.TotalDialogs,
		&stats.NewDialogs,
		&stats.UniqueUsers,
		&stats.TotalMessagesSent,
		&stats.TotalMessagesReceived,
		&stats.BusiestDay,
		&stats.BusiestHour,
	)
	if err != nil {
		return nil, fmt.Errorf("getting statistics: %w", err)
	}

	// Calculate response times (simplified - would need message pairs analysis for accurate times)
	stats.FirstResponseTimeMs = 0
	stats.AvgResponseTimeMs = 0

	return &stats, nil
}

// GetHeatmap returns activity heatmap data for an account
func (r *MessagePostgres) GetHeatmap(ctx context.Context, filter entity.StatisticsFilter) (*entity.Heatmap, error) {
	query := `
		SELECT
			EXTRACT(DOW FROM m.timestamp)::int as day,
			EXTRACT(HOUR FROM m.timestamp)::int as hour,
			COUNT(*) as count
		FROM dm_messages m
		JOIN dm_conversations c ON m.conversation_id = c.id
		WHERE c.account_id = $1
		  AND m.timestamp >= $2
		  AND m.timestamp <= $3
		GROUP BY 1, 2
		ORDER BY 1, 2
	`

	rows, err := r.pool.Query(ctx, query, filter.AccountID, filter.StartDate, filter.EndDate)
	if err != nil {
		return nil, fmt.Errorf("querying heatmap: %w", err)
	}
	defer rows.Close()

	var cells []entity.HeatmapCell
	for rows.Next() {
		var cell entity.HeatmapCell
		err := rows.Scan(&cell.Day, &cell.Hour, &cell.Count)
		if err != nil {
			return nil, fmt.Errorf("scanning heatmap row: %w", err)
		}
		cells = append(cells, cell)
	}

	return &entity.Heatmap{Cells: cells}, nil
}
