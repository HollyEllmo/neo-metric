package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vadim/neo-metric/internal/domain/publication/entity"
)

// MediaPostgres implements MediaRepository for PostgreSQL
type MediaPostgres struct {
	pool *pgxpool.Pool
}

// NewMediaPostgres creates a new PostgreSQL media repository
func NewMediaPostgres(pool *pgxpool.Pool) *MediaPostgres {
	return &MediaPostgres{pool: pool}
}

// Create inserts a new media item
func (r *MediaPostgres) Create(ctx context.Context, publicationID string, media *entity.MediaItem) error {
	if media.ID == "" {
		media.ID = uuid.New().String()
	}
	if media.CreatedAt.IsZero() {
		media.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO publication_media (id, publication_id, url, type, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.pool.Exec(ctx, query,
		media.ID,
		publicationID,
		media.URL,
		media.Type,
		media.Order,
		media.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting media: %w", err)
	}

	return nil
}

// GetByPublicationID retrieves all media items for a publication
func (r *MediaPostgres) GetByPublicationID(ctx context.Context, publicationID string) ([]entity.MediaItem, error) {
	query := `
		SELECT id, url, type, sort_order, created_at
		FROM publication_media
		WHERE publication_id = $1
		ORDER BY sort_order ASC
	`

	rows, err := r.pool.Query(ctx, query, publicationID)
	if err != nil {
		return nil, fmt.Errorf("querying media: %w", err)
	}
	defer rows.Close()

	var items []entity.MediaItem
	for rows.Next() {
		var item entity.MediaItem
		err := rows.Scan(&item.ID, &item.URL, &item.Type, &item.Order, &item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning media row: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

// Delete removes a media item by ID
func (r *MediaPostgres) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM publication_media WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting media: %w", err)
	}
	return nil
}

// DeleteByPublicationID removes all media items for a publication
func (r *MediaPostgres) DeleteByPublicationID(ctx context.Context, publicationID string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM publication_media WHERE publication_id = $1", publicationID)
	if err != nil {
		return fmt.Errorf("deleting media by publication: %w", err)
	}
	return nil
}

// UpdateOrder updates the order of media items
func (r *MediaPostgres) UpdateOrder(ctx context.Context, publicationID string, mediaIDs []string) error {
	for i, id := range mediaIDs {
		_, err := r.pool.Exec(ctx,
			"UPDATE publication_media SET sort_order = $1 WHERE id = $2 AND publication_id = $3",
			i, id, publicationID,
		)
		if err != nil {
			return fmt.Errorf("updating media order: %w", err)
		}
	}
	return nil
}
