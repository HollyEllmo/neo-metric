package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vadim/neo-metric/internal/domain/publication/entity"
	"github.com/vadim/neo-metric/internal/domain/publication/policy"
	"github.com/vadim/neo-metric/internal/httpx/response"
)

// PublicationPolicy defines the interface for publication operations
// Interface is defined by consumer (handler), not provider (policy)
type PublicationPolicy interface {
	CreatePublication(ctx context.Context, in policy.CreatePublicationInput) (*policy.CreatePublicationOutput, error)
	UpdatePublication(ctx context.Context, in policy.UpdatePublicationInput) (*policy.UpdatePublicationOutput, error)
	GetPublication(ctx context.Context, id string) (*entity.Publication, error)
	DeletePublication(ctx context.Context, in policy.DeletePublicationInput) error
	ListPublications(ctx context.Context, in policy.ListPublicationsInput) (*policy.ListPublicationsOutput, error)
	PublishNow(ctx context.Context, id string) (*entity.Publication, error)
	SchedulePublication(ctx context.Context, id string, scheduledAt time.Time) (*entity.Publication, error)
	SaveAsDraft(ctx context.Context, id string) (*entity.Publication, error)
}

// PublicationHandler handles HTTP requests for publications
type PublicationHandler struct {
	policy PublicationPolicy
}

// NewPublicationHandler creates a new publication handler
func NewPublicationHandler(p PublicationPolicy) *PublicationHandler {
	return &PublicationHandler{policy: p}
}

// RegisterRoutes registers publication routes
func (h *PublicationHandler) RegisterRoutes(r chi.Router) {
	r.Route("/publications", func(r chi.Router) {
		r.Post("/", h.Create())
		r.Get("/", h.List())
		r.Get("/{id}", h.Get())
		r.Put("/{id}", h.Update())
		r.Delete("/{id}", h.Delete())
		r.Post("/{id}/publish", h.PublishNow())
		r.Post("/{id}/schedule", h.Schedule())
		r.Post("/{id}/draft", h.SaveAsDraft())
	})
}

// CreateRequest represents the request body for creating a publication
type CreateRequest struct {
	AccountID   string              `json:"account_id"`
	Type        string              `json:"type"` // post, story, reel
	Caption     string              `json:"caption"`
	Media       []MediaRequest      `json:"media"`
	ScheduledAt *string             `json:"scheduled_at,omitempty"` // RFC3339 format
}

// MediaRequest represents a media item in requests
type MediaRequest struct {
	URL   string `json:"url"`
	Type  string `json:"type"` // image, video
	Order int    `json:"order"`
}

// Create handles POST /publications
func (h *PublicationHandler) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		// Validate required fields
		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}
		if len(req.Media) == 0 {
			response.BadRequest(w, "at least one media item is required")
			return
		}

		// Parse publication type
		pubType, err := parsePublicationType(req.Type)
		if err != nil {
			response.BadRequest(w, err.Error())
			return
		}

		// Parse scheduled time
		var scheduledAt *time.Time
		if req.ScheduledAt != nil && *req.ScheduledAt != "" {
			t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
			if err != nil {
				response.BadRequest(w, "invalid scheduled_at format, use RFC3339")
				return
			}
			scheduledAt = &t
		}

		// Build media input
		mediaInput := make([]policy.MediaInput, len(req.Media))
		for i, m := range req.Media {
			mediaType, err := parseMediaType(m.Type)
			if err != nil {
				response.BadRequest(w, err.Error())
				return
			}
			mediaInput[i] = policy.MediaInput{
				URL:   m.URL,
				Type:  mediaType,
				Order: m.Order,
			}
		}

		out, err := h.policy.CreatePublication(r.Context(), policy.CreatePublicationInput{
			AccountID:   req.AccountID,
			Type:        pubType,
			Caption:     req.Caption,
			Media:       mediaInput,
			ScheduledAt: scheduledAt,
		})
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.Created(w, out.Publication)
	}
}

// UpdateRequest represents the request body for updating a publication
type UpdateRequest struct {
	Caption       *string        `json:"caption,omitempty"`
	Media         []MediaRequest `json:"media,omitempty"`
	ScheduledAt   *string        `json:"scheduled_at,omitempty"`
	ClearSchedule bool           `json:"clear_schedule,omitempty"`
}

// Update handles PUT /publications/{id}
func (h *PublicationHandler) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var req UpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		// Parse scheduled time
		var scheduledAt *time.Time
		if req.ScheduledAt != nil && *req.ScheduledAt != "" {
			t, err := time.Parse(time.RFC3339, *req.ScheduledAt)
			if err != nil {
				response.BadRequest(w, "invalid scheduled_at format, use RFC3339")
				return
			}
			scheduledAt = &t
		}

		// Build media input
		var mediaInput []policy.MediaInput
		if len(req.Media) > 0 {
			mediaInput = make([]policy.MediaInput, len(req.Media))
			for i, m := range req.Media {
				mediaType, err := parseMediaType(m.Type)
				if err != nil {
					response.BadRequest(w, err.Error())
					return
				}
				mediaInput[i] = policy.MediaInput{
					URL:   m.URL,
					Type:  mediaType,
					Order: m.Order,
				}
			}
		}

		out, err := h.policy.UpdatePublication(r.Context(), policy.UpdatePublicationInput{
			ID:            id,
			Caption:       req.Caption,
			Media:         mediaInput,
			ScheduledAt:   scheduledAt,
			ClearSchedule: req.ClearSchedule,
		})
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.OK(w, out.Publication)
	}
}

// Get handles GET /publications/{id}
func (h *PublicationHandler) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		pub, err := h.policy.GetPublication(r.Context(), id)
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.OK(w, pub)
	}
}

// Delete handles DELETE /publications/{id}
func (h *PublicationHandler) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		err := h.policy.DeletePublication(r.Context(), policy.DeletePublicationInput{
			ID: id,
		})
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.NoContent(w)
	}
}

// ListResponse represents the response for listing publications
type ListResponse struct {
	Publications []entity.Publication `json:"publications"`
	Total        int64                `json:"total"`
	Limit        int                  `json:"limit"`
	Offset       int                  `json:"offset"`
}

// List handles GET /publications
func (h *PublicationHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		// Parse filters
		accountID := q.Get("account_id")

		var pubType *entity.PublicationType
		if t := q.Get("type"); t != "" {
			pt, err := parsePublicationType(t)
			if err != nil {
				response.BadRequest(w, err.Error())
				return
			}
			pubType = &pt
		}

		var status *entity.PublicationStatus
		if s := q.Get("status"); s != "" {
			ps, err := parsePublicationStatus(s)
			if err != nil {
				response.BadRequest(w, err.Error())
				return
			}
			status = &ps
		}

		var year, month *int
		if y := q.Get("year"); y != "" {
			yi, err := strconv.Atoi(y)
			if err != nil {
				response.BadRequest(w, "invalid year")
				return
			}
			year = &yi
		}
		if m := q.Get("month"); m != "" {
			mi, err := strconv.Atoi(m)
			if err != nil || mi < 1 || mi > 12 {
				response.BadRequest(w, "invalid month")
				return
			}
			month = &mi
		}

		// Parse pagination
		limit := 50
		offset := 0
		if l := q.Get("limit"); l != "" {
			li, err := strconv.Atoi(l)
			if err != nil || li < 1 {
				response.BadRequest(w, "invalid limit")
				return
			}
			if li > 100 {
				li = 100
			}
			limit = li
		}
		if o := q.Get("offset"); o != "" {
			oi, err := strconv.Atoi(o)
			if err != nil || oi < 0 {
				response.BadRequest(w, "invalid offset")
				return
			}
			offset = oi
		}

		out, err := h.policy.ListPublications(r.Context(), policy.ListPublicationsInput{
			AccountID: accountID,
			Type:      pubType,
			Status:    status,
			Year:      year,
			Month:     month,
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.OK(w, ListResponse{
			Publications: out.Publications,
			Total:        out.Total,
			Limit:        limit,
			Offset:       offset,
		})
	}
}

// PublishNow handles POST /publications/{id}/publish
func (h *PublicationHandler) PublishNow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		pub, err := h.policy.PublishNow(r.Context(), id)
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.OK(w, pub)
	}
}

// ScheduleRequest represents the request body for scheduling a publication
type ScheduleRequest struct {
	ScheduledAt string `json:"scheduled_at"` // RFC3339 format
}

// Schedule handles POST /publications/{id}/schedule
func (h *PublicationHandler) Schedule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var req ScheduleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			response.BadRequest(w, "invalid scheduled_at format, use RFC3339")
			return
		}

		pub, err := h.policy.SchedulePublication(r.Context(), id, scheduledAt)
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.OK(w, pub)
	}
}

// SaveAsDraft handles POST /publications/{id}/draft
func (h *PublicationHandler) SaveAsDraft() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		pub, err := h.policy.SaveAsDraft(r.Context(), id)
		if err != nil {
			handleDomainError(w, err)
			return
		}

		response.OK(w, pub)
	}
}

// Helper functions

func parsePublicationType(s string) (entity.PublicationType, error) {
	switch s {
	case "post":
		return entity.PublicationTypePost, nil
	case "story":
		return entity.PublicationTypeStory, nil
	case "reel":
		return entity.PublicationTypeReel, nil
	default:
		return "", entity.ErrInvalidPublicationType
	}
}

func parsePublicationStatus(s string) (entity.PublicationStatus, error) {
	switch s {
	case "draft":
		return entity.PublicationStatusDraft, nil
	case "scheduled":
		return entity.PublicationStatusScheduled, nil
	case "published":
		return entity.PublicationStatusPublished, nil
	case "error":
		return entity.PublicationStatusError, nil
	default:
		return "", entity.ErrInvalidStatus
	}
}

func parseMediaType(s string) (entity.MediaType, error) {
	switch s {
	case "image":
		return entity.MediaTypeImage, nil
	case "video":
		return entity.MediaTypeVideo, nil
	default:
		return "", entity.ErrInvalidPublicationType
	}
}

func handleDomainError(w http.ResponseWriter, err error) {
	switch err {
	case entity.ErrPublicationNotFound:
		response.NotFound(w, err.Error())
	case entity.ErrPublicationNotEditable, entity.ErrPublicationNotDeletable:
		response.Error(w, http.StatusConflict, err.Error())
	case entity.ErrEmptyAccountID, entity.ErrNoMedia, entity.ErrTooManyMediaItems,
		entity.ErrSingleMediaRequired, entity.ErrCaptionTooLong, entity.ErrScheduledTimeInPast,
		entity.ErrInvalidPublicationType, entity.ErrInvalidStatus:
		response.BadRequest(w, err.Error())
	case entity.ErrInstagramUnauthorized:
		response.Unauthorized(w, err.Error())
	case entity.ErrInstagramRateLimited, entity.ErrDailyPublishingLimit:
		response.Error(w, http.StatusTooManyRequests, err.Error())
	default:
		response.InternalError(w, "internal server error")
	}
}
