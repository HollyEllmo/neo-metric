package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vadim/neo-metric/internal/domain/template/entity"
)

// TemplatePostgres implements template repository for PostgreSQL
type TemplatePostgres struct {
	pool *pgxpool.Pool
}

// NewTemplatePostgres creates a new PostgreSQL template repository
func NewTemplatePostgres(pool *pgxpool.Pool) *TemplatePostgres {
	return &TemplatePostgres{pool: pool}
}

// Create inserts a new template
func (r *TemplatePostgres) Create(ctx context.Context, tmpl *entity.Template) error {
	query := `
		INSERT INTO templates (id, account_id, title, content, images, icon, type, usage_count, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 0, $7, $7)
		RETURNING id, created_at, updated_at
	`

	now := time.Now()
	err := r.pool.QueryRow(ctx, query,
		tmpl.AccountID,
		tmpl.Title,
		tmpl.Content,
		tmpl.Images,
		tmpl.Icon,
		tmpl.Type,
		now,
	).Scan(&tmpl.ID, &tmpl.CreatedAt, &tmpl.UpdatedAt)

	if err != nil {
		return fmt.Errorf("creating template: %w", err)
	}

	return nil
}

// GetByID retrieves a template by ID
func (r *TemplatePostgres) GetByID(ctx context.Context, id string) (*entity.Template, error) {
	query := `
		SELECT id, account_id, title, content, images, icon, type, usage_count, created_at, updated_at
		FROM templates
		WHERE id = $1
	`

	var tmpl entity.Template
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&tmpl.ID,
		&tmpl.AccountID,
		&tmpl.Title,
		&tmpl.Content,
		&tmpl.Images,
		&tmpl.Icon,
		&tmpl.Type,
		&tmpl.UsageCount,
		&tmpl.CreatedAt,
		&tmpl.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting template: %w", err)
	}

	return &tmpl, nil
}

// Update updates an existing template
func (r *TemplatePostgres) Update(ctx context.Context, tmpl *entity.Template) error {
	query := `
		UPDATE templates
		SET title = $2, content = $3, images = $4, icon = $5, type = $6, updated_at = $7
		WHERE id = $1
	`

	now := time.Now()
	result, err := r.pool.Exec(ctx, query,
		tmpl.ID,
		tmpl.Title,
		tmpl.Content,
		tmpl.Images,
		tmpl.Icon,
		tmpl.Type,
		now,
	)
	if err != nil {
		return fmt.Errorf("updating template: %w", err)
	}

	if result.RowsAffected() == 0 {
		return entity.ErrTemplateNotFound
	}

	tmpl.UpdatedAt = now
	return nil
}

// Delete removes a template
func (r *TemplatePostgres) Delete(ctx context.Context, id string) error {
	result, err := r.pool.Exec(ctx, "DELETE FROM templates WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting template: %w", err)
	}

	if result.RowsAffected() == 0 {
		return entity.ErrTemplateNotFound
	}

	return nil
}

// ListFilter contains filters for listing templates
type ListFilter struct {
	AccountID string
	Type      *entity.TemplateType
}

// ListOptions contains pagination and sorting options
type ListOptions struct {
	Limit   int
	Offset  int
	SortBy  string // "usage_count", "created_at", "updated_at", "title"
	Desc    bool
}

// List retrieves templates with filtering and pagination
func (r *TemplatePostgres) List(ctx context.Context, filter ListFilter, opts ListOptions) ([]entity.Template, error) {
	query := `
		SELECT id, account_id, title, content, images, icon, type, usage_count, created_at, updated_at
		FROM templates
		WHERE account_id = $1
	`
	args := []interface{}{filter.AccountID}
	argNum := 2

	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, *filter.Type)
		argNum++
	}

	// Sorting
	sortCol := "usage_count"
	if opts.SortBy != "" {
		switch opts.SortBy {
		case "usage_count", "created_at", "updated_at", "title":
			sortCol = opts.SortBy
		}
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
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	defer rows.Close()

	var templates []entity.Template
	for rows.Next() {
		var tmpl entity.Template
		err := rows.Scan(
			&tmpl.ID,
			&tmpl.AccountID,
			&tmpl.Title,
			&tmpl.Content,
			&tmpl.Images,
			&tmpl.Icon,
			&tmpl.Type,
			&tmpl.UsageCount,
			&tmpl.CreatedAt,
			&tmpl.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning template: %w", err)
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}

// Count returns the total count of templates for an account
func (r *TemplatePostgres) Count(ctx context.Context, filter ListFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM templates WHERE account_id = $1"
	args := []interface{}{filter.AccountID}

	if filter.Type != nil {
		query += " AND type = $2"
		args = append(args, *filter.Type)
	}

	var count int64
	err := r.pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting templates: %w", err)
	}

	return count, nil
}

// IncrementUsageCount increments the usage count of a template
func (r *TemplatePostgres) IncrementUsageCount(ctx context.Context, id string) error {
	result, err := r.pool.Exec(ctx,
		"UPDATE templates SET usage_count = usage_count + 1, updated_at = $2 WHERE id = $1",
		id, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("incrementing usage count: %w", err)
	}

	if result.RowsAffected() == 0 {
		return entity.ErrTemplateNotFound
	}

	return nil
}
