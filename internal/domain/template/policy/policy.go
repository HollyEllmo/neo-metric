package policy

import (
	"context"

	"github.com/vadim/neo-metric/internal/domain/template/entity"
	"github.com/vadim/neo-metric/internal/domain/template/service"
)

// TemplateService defines the interface for the template service
type TemplateService interface {
	Create(ctx context.Context, in service.CreateInput) (*entity.Template, error)
	GetByID(ctx context.Context, id string) (*entity.Template, error)
	Update(ctx context.Context, in service.UpdateInput) (*entity.Template, error)
	Delete(ctx context.Context, id, accountID string) error
	List(ctx context.Context, in service.ListInput) (*service.ListOutput, error)
	IncrementUsage(ctx context.Context, id, accountID string) error
}

// Policy handles template operations
type Policy struct {
	svc TemplateService
}

// New creates a new template policy
func New(svc TemplateService) *Policy {
	return &Policy{svc: svc}
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
func (p *Policy) Create(ctx context.Context, in CreateInput) (*entity.Template, error) {
	return p.svc.Create(ctx, service.CreateInput{
		AccountID: in.AccountID,
		Title:     in.Title,
		Content:   in.Content,
		Images:    in.Images,
		Icon:      in.Icon,
		Type:      in.Type,
	})
}

// GetByID retrieves a template by ID
func (p *Policy) GetByID(ctx context.Context, id, accountID string) (*entity.Template, error) {
	tmpl, err := p.svc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check ownership
	if tmpl.AccountID != accountID {
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
func (p *Policy) Update(ctx context.Context, in UpdateInput) (*entity.Template, error) {
	return p.svc.Update(ctx, service.UpdateInput{
		ID:        in.ID,
		AccountID: in.AccountID,
		Title:     in.Title,
		Content:   in.Content,
		Images:    in.Images,
		Icon:      in.Icon,
		Type:      in.Type,
	})
}

// Delete removes a template
func (p *Policy) Delete(ctx context.Context, id, accountID string) error {
	return p.svc.Delete(ctx, id, accountID)
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
func (p *Policy) List(ctx context.Context, in ListInput) (*ListOutput, error) {
	result, err := p.svc.List(ctx, service.ListInput{
		AccountID: in.AccountID,
		Type:      in.Type,
		Limit:     in.Limit,
		Offset:    in.Offset,
		SortBy:    in.SortBy,
		Desc:      in.Desc,
	})
	if err != nil {
		return nil, err
	}

	return &ListOutput{
		Templates: result.Templates,
		Total:     result.Total,
	}, nil
}

// IncrementUsage increments the usage count of a template
func (p *Policy) IncrementUsage(ctx context.Context, id, accountID string) error {
	return p.svc.IncrementUsage(ctx, id, accountID)
}
