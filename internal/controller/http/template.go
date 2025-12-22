package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/vadim/neo-metric/internal/domain/template/entity"
	"github.com/vadim/neo-metric/internal/domain/template/policy"
	"github.com/vadim/neo-metric/internal/httpx/response"
)

// TemplatePolicy defines the interface for template operations
type TemplatePolicy interface {
	Create(ctx context.Context, in policy.CreateInput) (*entity.Template, error)
	GetByID(ctx context.Context, id, accountID string) (*entity.Template, error)
	Update(ctx context.Context, in policy.UpdateInput) (*entity.Template, error)
	Delete(ctx context.Context, id, accountID string) error
	List(ctx context.Context, in policy.ListInput) (*policy.ListOutput, error)
	IncrementUsage(ctx context.Context, id, accountID string) error
}

// TemplateHandler handles HTTP requests for templates
type TemplateHandler struct {
	policy TemplatePolicy
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler(p TemplatePolicy) *TemplateHandler {
	return &TemplateHandler{policy: p}
}

// RegisterRoutes registers template routes
func (h *TemplateHandler) RegisterRoutes(r chi.Router) {
	r.Route("/templates", func(r chi.Router) {
		// List templates
		r.Get("/", h.List())

		// Create template
		r.Post("/", h.Create())

		// Get template by ID
		r.Get("/{templateId}", h.GetByID())

		// Update template
		r.Put("/{templateId}", h.Update())

		// Delete template
		r.Delete("/{templateId}", h.Delete())

		// Increment usage count
		r.Post("/{templateId}/use", h.IncrementUsage())
	})
}

// CreateTemplateRequest represents the request body for creating a template
type CreateTemplateRequest struct {
	AccountID string              `json:"account_id"`
	Title     string              `json:"title"`
	Content   string              `json:"content"`
	Images    []string            `json:"images,omitempty"`
	Icon      string              `json:"icon,omitempty"`
	Type      entity.TemplateType `json:"type"`
}

// Create handles POST /templates
func (h *TemplateHandler) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}
		if req.Title == "" {
			response.BadRequest(w, "title is required")
			return
		}
		if req.Content == "" {
			response.BadRequest(w, "content is required")
			return
		}
		if req.Type == "" {
			req.Type = entity.TemplateTypeBoth
		}

		tmpl, err := h.policy.Create(r.Context(), policy.CreateInput{
			AccountID: req.AccountID,
			Title:     req.Title,
			Content:   req.Content,
			Images:    req.Images,
			Icon:      req.Icon,
			Type:      req.Type,
		})
		if err != nil {
			handleTemplateError(w, err)
			return
		}

		response.Created(w, tmpl)
	}
}

// GetByID handles GET /templates/{templateId}
func (h *TemplateHandler) GetByID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "templateId")
		accountID := r.URL.Query().Get("account_id")

		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		tmpl, err := h.policy.GetByID(r.Context(), templateID, accountID)
		if err != nil {
			handleTemplateError(w, err)
			return
		}

		response.OK(w, tmpl)
	}
}

// UpdateTemplateRequest represents the request body for updating a template
type UpdateTemplateRequest struct {
	AccountID string               `json:"account_id"`
	Title     *string              `json:"title,omitempty"`
	Content   *string              `json:"content,omitempty"`
	Images    []string             `json:"images,omitempty"`
	Icon      *string              `json:"icon,omitempty"`
	Type      *entity.TemplateType `json:"type,omitempty"`
}

// Update handles PUT /templates/{templateId}
func (h *TemplateHandler) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "templateId")

		var req UpdateTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		tmpl, err := h.policy.Update(r.Context(), policy.UpdateInput{
			ID:        templateID,
			AccountID: req.AccountID,
			Title:     req.Title,
			Content:   req.Content,
			Images:    req.Images,
			Icon:      req.Icon,
			Type:      req.Type,
		})
		if err != nil {
			handleTemplateError(w, err)
			return
		}

		response.OK(w, tmpl)
	}
}

// Delete handles DELETE /templates/{templateId}
func (h *TemplateHandler) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "templateId")
		accountID := r.URL.Query().Get("account_id")

		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		err := h.policy.Delete(r.Context(), templateID, accountID)
		if err != nil {
			handleTemplateError(w, err)
			return
		}

		response.NoContent(w)
	}
}

// ListTemplatesResponse represents the response for listing templates
type ListTemplatesResponse struct {
	Templates []entity.Template `json:"templates"`
	Total     int64             `json:"total"`
}

// List handles GET /templates
func (h *TemplateHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		// Parse optional type filter
		var templateType *entity.TemplateType
		if t := r.URL.Query().Get("type"); t != "" {
			tt := entity.TemplateType(t)
			templateType = &tt
		}

		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
				if limit > 100 {
					limit = 100
				}
			}
		}

		offset := 0
		if o := r.URL.Query().Get("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		sortBy := r.URL.Query().Get("sort_by")
		if sortBy == "" {
			sortBy = "created_at"
		}

		desc := r.URL.Query().Get("desc") == "true"

		result, err := h.policy.List(r.Context(), policy.ListInput{
			AccountID: accountID,
			Type:      templateType,
			Limit:     limit,
			Offset:    offset,
			SortBy:    sortBy,
			Desc:      desc,
		})
		if err != nil {
			handleTemplateError(w, err)
			return
		}

		response.OK(w, ListTemplatesResponse{
			Templates: result.Templates,
			Total:     result.Total,
		})
	}
}

// IncrementUsageRequest represents the request body for incrementing usage
type IncrementUsageRequest struct {
	AccountID string `json:"account_id"`
}

// IncrementUsage handles POST /templates/{templateId}/use
func (h *TemplateHandler) IncrementUsage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateID := chi.URLParam(r, "templateId")

		var req IncrementUsageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		err := h.policy.IncrementUsage(r.Context(), templateID, req.AccountID)
		if err != nil {
			handleTemplateError(w, err)
			return
		}

		response.OK(w, map[string]string{"status": "ok"})
	}
}

func handleTemplateError(w http.ResponseWriter, err error) {
	switch err {
	case entity.ErrTemplateNotFound:
		response.NotFound(w, err.Error())
	case entity.ErrEmptyTitle:
		response.BadRequest(w, err.Error())
	case entity.ErrEmptyContent:
		response.BadRequest(w, err.Error())
	case entity.ErrInvalidTemplateType:
		response.BadRequest(w, err.Error())
	case entity.ErrTitleTooLong:
		response.BadRequest(w, err.Error())
	case entity.ErrContentTooLong:
		response.BadRequest(w, err.Error())
	case entity.ErrTooManyImages:
		response.BadRequest(w, err.Error())
	default:
		response.InternalError(w, "internal server error")
	}
}
