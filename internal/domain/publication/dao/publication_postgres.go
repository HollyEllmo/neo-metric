package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vadim/neo-metric/internal/domain/publication/entity"
)

// PublicationPostgres implements PublicationRepository for PostgreSQL
type PublicationPostgres struct {
	pool *pgxpool.Pool
}

// NewPublicationPostgres creates a new PostgreSQL publication repository
func NewPublicationPostgres(pool *pgxpool.Pool) *PublicationPostgres {
	return &PublicationPostgres{pool: pool}
}

// Create inserts a new publication
func (r *PublicationPostgres) Create(ctx context.Context, pub *entity.Publication) error {
	query := `
		INSERT INTO publications (id, account_id, type, status, caption, scheduled_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pool.Exec(ctx, query,
		pub.ID,
		pub.AccountID,
		pub.Type,
		pub.Status,
		pub.Caption,
		pub.ScheduledAt,
		pub.CreatedAt,
		pub.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting publication: %w", err)
	}

	return nil
}

// GetByID retrieves a publication by ID
func (r *PublicationPostgres) GetByID(ctx context.Context, id string) (*entity.Publication, error) {
	query := `
		SELECT id, account_id, instagram_media_id, type, status, caption,
		       scheduled_at, published_at, error_message, created_at, updated_at
		FROM publications
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)

	var pub entity.Publication
	var instagramMediaID, errorMessage *string
	var scheduledAt, publishedAt *time.Time

	err := row.Scan(
		&pub.ID,
		&pub.AccountID,
		&instagramMediaID,
		&pub.Type,
		&pub.Status,
		&pub.Caption,
		&scheduledAt,
		&publishedAt,
		&errorMessage,
		&pub.CreatedAt,
		&pub.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning publication: %w", err)
	}

	if instagramMediaID != nil {
		pub.InstagramMediaID = *instagramMediaID
	}
	if errorMessage != nil {
		pub.ErrorMessage = *errorMessage
	}
	pub.ScheduledAt = scheduledAt
	pub.PublishedAt = publishedAt

	return &pub, nil
}

// Update updates an existing publication
func (r *PublicationPostgres) Update(ctx context.Context, pub *entity.Publication) error {
	query := `
		UPDATE publications
		SET caption = $2, status = $3, scheduled_at = $4, updated_at = $5
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		pub.ID,
		pub.Caption,
		pub.Status,
		pub.ScheduledAt,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("updating publication: %w", err)
	}

	return nil
}

// Delete removes a publication
func (r *PublicationPostgres) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM publications WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting publication: %w", err)
	}
	return nil
}

// List retrieves publications with filtering
func (r *PublicationPostgres) List(ctx context.Context, filter PublicationFilter, opts ListOptions) ([]entity.Publication, error) {
	query := `
		SELECT id, account_id, instagram_media_id, type, status, caption,
		       scheduled_at, published_at, error_message, created_at, updated_at
		FROM publications
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if filter.AccountID != "" {
		query += fmt.Sprintf(" AND account_id = $%d", argNum)
		args = append(args, filter.AccountID)
		argNum++
	}

	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, *filter.Type)
		argNum++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, *filter.Status)
		argNum++
	}

	if filter.Year != nil && filter.Month != nil {
		query += fmt.Sprintf(" AND EXTRACT(YEAR FROM COALESCE(scheduled_at, created_at)) = $%d", argNum)
		args = append(args, *filter.Year)
		argNum++
		query += fmt.Sprintf(" AND EXTRACT(MONTH FROM COALESCE(scheduled_at, created_at)) = $%d", argNum)
		args = append(args, *filter.Month)
		argNum++
	}

	// Sorting
	sortCol := "created_at"
	if opts.SortBy != "" {
		sortCol = opts.SortBy
	}
	order := "DESC"
	if !opts.Desc {
		order = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortCol, order)

	// Pagination
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, opts.Limit)
		argNum++
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, opts.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying publications: %w", err)
	}
	defer rows.Close()

	var publications []entity.Publication
	for rows.Next() {
		var pub entity.Publication
		var instagramMediaID, errorMessage *string
		var scheduledAt, publishedAt *time.Time

		err := rows.Scan(
			&pub.ID,
			&pub.AccountID,
			&instagramMediaID,
			&pub.Type,
			&pub.Status,
			&pub.Caption,
			&scheduledAt,
			&publishedAt,
			&errorMessage,
			&pub.CreatedAt,
			&pub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if instagramMediaID != nil {
			pub.InstagramMediaID = *instagramMediaID
		}
		if errorMessage != nil {
			pub.ErrorMessage = *errorMessage
		}
		pub.ScheduledAt = scheduledAt
		pub.PublishedAt = publishedAt

		publications = append(publications, pub)
	}

	return publications, nil
}

// Count returns the total count of publications matching the filter
func (r *PublicationPostgres) Count(ctx context.Context, filter PublicationFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM publications WHERE 1=1"
	args := []interface{}{}
	argNum := 1

	if filter.AccountID != "" {
		query += fmt.Sprintf(" AND account_id = $%d", argNum)
		args = append(args, filter.AccountID)
		argNum++
	}

	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, *filter.Type)
		argNum++
	}

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, *filter.Status)
		argNum++
	}

	var count int64
	err := r.pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting publications: %w", err)
	}

	return count, nil
}

// GetScheduledForPublishing retrieves publications due for publishing
func (r *PublicationPostgres) GetScheduledForPublishing(ctx context.Context, now time.Time) ([]entity.Publication, error) {
	query := `
		SELECT id, account_id, instagram_media_id, type, status, caption,
		       scheduled_at, published_at, error_message, created_at, updated_at
		FROM publications
		WHERE status = 'scheduled' AND scheduled_at <= $1
		ORDER BY scheduled_at ASC
	`

	rows, err := r.pool.Query(ctx, query, now)
	if err != nil {
		return nil, fmt.Errorf("querying scheduled publications: %w", err)
	}
	defer rows.Close()

	var publications []entity.Publication
	for rows.Next() {
		var pub entity.Publication
		var instagramMediaID, errorMessage *string
		var scheduledAt, publishedAt *time.Time

		err := rows.Scan(
			&pub.ID,
			&pub.AccountID,
			&instagramMediaID,
			&pub.Type,
			&pub.Status,
			&pub.Caption,
			&scheduledAt,
			&publishedAt,
			&errorMessage,
			&pub.CreatedAt,
			&pub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if instagramMediaID != nil {
			pub.InstagramMediaID = *instagramMediaID
		}
		if errorMessage != nil {
			pub.ErrorMessage = *errorMessage
		}
		pub.ScheduledAt = scheduledAt
		pub.PublishedAt = publishedAt

		publications = append(publications, pub)
	}

	return publications, nil
}

// UpdateStatus updates only the status and error message
func (r *PublicationPostgres) UpdateStatus(ctx context.Context, id string, status entity.PublicationStatus, errorMsg string) error {
	query := `
		UPDATE publications
		SET status = $2, error_message = $3, updated_at = $4
		WHERE id = $1
	`

	var errPtr *string
	if errorMsg != "" {
		errPtr = &errorMsg
	}

	_, err := r.pool.Exec(ctx, query, id, status, errPtr, time.Now())
	if err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	return nil
}

// SetPublished marks a publication as published
func (r *PublicationPostgres) SetPublished(ctx context.Context, id string, instagramMediaID string, publishedAt time.Time) error {
	query := `
		UPDATE publications
		SET status = 'published', instagram_media_id = $2, published_at = $3, updated_at = $4
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id, instagramMediaID, publishedAt, time.Now())
	if err != nil {
		return fmt.Errorf("setting published: %w", err)
	}

	return nil
}

// GetAccountIDByMediaID retrieves the account ID for a publication by its Instagram media ID
func (r *PublicationPostgres) GetAccountIDByMediaID(ctx context.Context, instagramMediaID string) (string, error) {
	query := `SELECT account_id FROM publications WHERE instagram_media_id = $1`

	var accountID int64
	err := r.pool.QueryRow(ctx, query, instagramMediaID).Scan(&accountID)
	if err == pgx.ErrNoRows {
		return "", fmt.Errorf("publication not found for media id: %s", instagramMediaID)
	}
	if err != nil {
		return "", fmt.Errorf("getting account id: %w", err)
	}

	return fmt.Sprintf("%d", accountID), nil
}
