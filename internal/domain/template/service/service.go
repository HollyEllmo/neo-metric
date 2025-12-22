package service

import (
	"context"
	"fmt"

	"github.com/vadim/neo-metric/internal/domain/template/entity"
)

// TemplateRepository defines the interface for template storage
type TemplateRepository interface {
	Create(ctx context.Context, tmpl *entity.Template) error
	GetByID(ctx context.Context, id string) (*entity.Template, error)
	Update(ctx context.Context, tmpl *entity.Template) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ListFilter, opts ListOptions) ([]entity.Template, error)
	Count(ctx context.Context, filter ListFilter) (int64, error)
	IncrementUsageCount(ctx context.Context, id string) error
}

// ListFilter contains filters for listing templates
type ListFilter struct {
	AccountID string
	Type      *entity.TemplateType
}

// ListOptions contains pagination and sorting options
type ListOptions struct {
	Limit  int
	Offset int
	SortBy string // "usage_count", "created_at", "updated_at", "title"
	Desc   bool
}

// Service handles template business logic
type Service struct {
	repo TemplateRepository
}

// New creates a new template service
func New(repo TemplateRepository) *Service {
	return &Service{repo: repo}
}

// CreateInput represents input for creating a template
type CreateInput struct {
	AccountID string
	Title     string
	Content   string
	Images    []string
	Icon      string
	Type      entity.TemplateType
}

// Create creates a new template
func (s *Service) Create(ctx context.Context, in CreateInput) (*entity.Template, error) {
	tmpl := &entity.Template{
		AccountID: in.AccountID,
		Title:     in.Title,
		Content:   in.Content,
		Images:    in.Images,
		Icon:      in.Icon,
		Type:      in.Type,
	}

	if err := tmpl.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, tmpl); err != nil {
		return nil, fmt.Errorf("creating template: %w", err)
	}

	return tmpl, nil
}

// GetByID retrieves a template by ID
func (s *Service) GetByID(ctx context.Context, id string) (*entity.Template, error) {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting template: %w", err)
	}
	if tmpl == nil {
		return nil, entity.ErrTemplateNotFound
	}
	return tmpl, nil
}

// UpdateInput represents input for updating a template
type UpdateInput struct {
	ID        string
	AccountID string
	Title     *string
	Content   *string
	Images    []string
	Icon      *string
	Type      *entity.TemplateType
}

// Update updates an existing template
func (s *Service) Update(ctx context.Context, in UpdateInput) (*entity.Template, error) {
	tmpl, err := s.repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, fmt.Errorf("getting template: %w", err)
	}
	if tmpl == nil {
		return nil, entity.ErrTemplateNotFound
	}

	// Check ownership
	if tmpl.AccountID != in.AccountID {
		return nil, entity.ErrTemplateNotFound
	}

	// Apply updates
	if in.Title != nil {
		tmpl.Title = *in.Title
	}
	if in.Content != nil {
		tmpl.Content = *in.Content
	}
	if in.Images != nil {
		tmpl.Images = in.Images
	}
	if in.Icon != nil {
		tmpl.Icon = *in.Icon
	}
	if in.Type != nil {
		tmpl.Type = *in.Type
	}

	if err := tmpl.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, tmpl); err != nil {
		return nil, fmt.Errorf("updating template: %w", err)
	}

	return tmpl, nil
}

// Delete removes a template
func (s *Service) Delete(ctx context.Context, id, accountID string) error {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting template: %w", err)
	}
	if tmpl == nil {
		return entity.ErrTemplateNotFound
	}

	// Check ownership
	if tmpl.AccountID != accountID {
		return entity.ErrTemplateNotFound
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting template: %w", err)
	}

	return nil
}

// ListInput represents input for listing templates
type ListInput struct {
	AccountID string
	Type      *entity.TemplateType
	Limit     int
	Offset    int
	SortBy    string
	Desc      bool
}

// ListOutput represents output from listing templates
type ListOutput struct {
	Templates []entity.Template
	Total     int64
}

// List retrieves templates with filtering and pagination
func (s *Service) List(ctx context.Context, in ListInput) (*ListOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	filter := ListFilter{
		AccountID: in.AccountID,
		Type:      in.Type,
	}

	opts := ListOptions{
		Limit:  limit,
		Offset: in.Offset,
		SortBy: in.SortBy,
		Desc:   in.Desc,
	}

	templates, err := s.repo.List(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}

	total, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("counting templates: %w", err)
	}

	return &ListOutput{
		Templates: templates,
		Total:     total,
	}, nil
}

// IncrementUsage increments the usage count of a template
func (s *Service) IncrementUsage(ctx context.Context, id, accountID string) error {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting template: %w", err)
	}
	if tmpl == nil {
		return entity.ErrTemplateNotFound
	}

	// Check ownership
	if tmpl.AccountID != accountID {
		return entity.ErrTemplateNotFound
	}

	if err := s.repo.IncrementUsageCount(ctx, id); err != nil {
		return fmt.Errorf("incrementing usage: %w", err)
	}

	return nil
}
