package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vadim/neo-metric/internal/domain/direct/entity"
	"github.com/vadim/neo-metric/internal/domain/direct/policy"
	"github.com/vadim/neo-metric/internal/httpx/response"
)

// DirectPolicy defines the interface for direct message operations
type DirectPolicy interface {
	GetConversations(ctx context.Context, in policy.GetConversationsInput) (*policy.GetConversationsOutput, error)
	SearchConversations(ctx context.Context, in policy.SearchConversationsInput) (*policy.GetConversationsOutput, error)
	GetMessages(ctx context.Context, in policy.GetMessagesInput) (*policy.GetMessagesOutput, error)
	SendMessage(ctx context.Context, in policy.SendMessageInput) (*policy.SendMessageOutput, error)
	SendMediaMessage(ctx context.Context, in policy.SendMediaMessageInput) (*policy.SendMessageOutput, error)
	GetStatistics(ctx context.Context, in policy.GetStatisticsInput) (*entity.Statistics, error)
	GetHeatmap(ctx context.Context, in policy.GetHeatmapInput) (*entity.Heatmap, error)
}

// DirectHandler handles HTTP requests for direct messages
type DirectHandler struct {
	policy DirectPolicy
}

// NewDirectHandler creates a new direct message handler
func NewDirectHandler(p DirectPolicy) *DirectHandler {
	return &DirectHandler{policy: p}
}

// RegisterRoutes registers direct message routes
func (h *DirectHandler) RegisterRoutes(r chi.Router) {
	r.Route("/direct", func(r chi.Router) {
		// Get conversations list
		r.Get("/conversations", h.GetConversations())

		// Search conversations
		r.Get("/conversations/search", h.SearchConversations())

		// Get messages in a conversation
		r.Get("/conversations/{conversationId}/messages", h.GetMessages())

		// Send text message
		r.Post("/conversations/{conversationId}/messages", h.SendMessage())

		// Send media message
		r.Post("/conversations/{conversationId}/media", h.SendMediaMessage())

		// Get statistics
		r.Get("/statistics", h.GetStatistics())

		// Get heatmap
		r.Get("/heatmap", h.GetHeatmap())
	})
}

// GetConversationsResponse represents the response for getting conversations
type GetConversationsResponse struct {
	Conversations []entity.Conversation `json:"conversations"`
	Total         int64                 `json:"total"`
	HasMore       bool                  `json:"has_more"`
}

// GetConversations handles GET /direct/conversations
func (h *DirectHandler) GetConversations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
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

		result, err := h.policy.GetConversations(r.Context(), policy.GetConversationsInput{
			AccountID: accountID,
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			handleDirectError(w, err)
			return
		}

		response.OK(w, GetConversationsResponse{
			Conversations: result.Conversations,
			Total:         result.Total,
			HasMore:       result.HasMore,
		})
	}
}

// SearchConversations handles GET /direct/conversations/search
func (h *DirectHandler) SearchConversations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		query := r.URL.Query().Get("q")
		if query == "" {
			response.BadRequest(w, "q (query) is required")
			return
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

		result, err := h.policy.SearchConversations(r.Context(), policy.SearchConversationsInput{
			AccountID: accountID,
			Query:     query,
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			handleDirectError(w, err)
			return
		}

		response.OK(w, GetConversationsResponse{
			Conversations: result.Conversations,
			Total:         result.Total,
			HasMore:       result.HasMore,
		})
	}
}

// GetMessagesResponse represents the response for getting messages
type GetMessagesResponse struct {
	Messages []entity.Message `json:"messages"`
	Total    int64            `json:"total"`
	HasMore  bool             `json:"has_more"`
}

// GetMessages handles GET /direct/conversations/{conversationId}/messages
func (h *DirectHandler) GetMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conversationID := chi.URLParam(r, "conversationId")
		accountID := r.URL.Query().Get("account_id")

		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
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

		result, err := h.policy.GetMessages(r.Context(), policy.GetMessagesInput{
			AccountID:      accountID,
			ConversationID: conversationID,
			Limit:          limit,
			Offset:         offset,
		})
		if err != nil {
			handleDirectError(w, err)
			return
		}

		response.OK(w, GetMessagesResponse{
			Messages: result.Messages,
			Total:    result.Total,
			HasMore:  result.HasMore,
		})
	}
}

// SendMessageRequest represents the request body for sending a message
type SendMessageRequest struct {
	AccountID   string `json:"account_id"`
	RecipientID string `json:"recipient_id"`
	Message     string `json:"message"`
}

// SendMessageResponse represents the response for sending a message
type SendMessageResponse struct {
	MessageID string `json:"message_id"`
}

// SendMessage handles POST /direct/conversations/{conversationId}/messages
func (h *DirectHandler) SendMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conversationID := chi.URLParam(r, "conversationId")

		var req SendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}
		if req.RecipientID == "" {
			response.BadRequest(w, "recipient_id is required")
			return
		}
		if req.Message == "" {
			response.BadRequest(w, "message is required")
			return
		}

		result, err := h.policy.SendMessage(r.Context(), policy.SendMessageInput{
			AccountID:      req.AccountID,
			ConversationID: conversationID,
			RecipientID:    req.RecipientID,
			Message:        req.Message,
		})
		if err != nil {
			handleDirectError(w, err)
			return
		}

		response.Created(w, SendMessageResponse{MessageID: result.MessageID})
	}
}

// SendMediaMessageRequest represents the request body for sending a media message
type SendMediaMessageRequest struct {
	AccountID   string `json:"account_id"`
	RecipientID string `json:"recipient_id"`
	MediaURL    string `json:"media_url"`
	MediaType   string `json:"media_type"` // image, video, audio
}

// SendMediaMessage handles POST /direct/conversations/{conversationId}/media
func (h *DirectHandler) SendMediaMessage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conversationID := chi.URLParam(r, "conversationId")

		var req SendMediaMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}
		if req.RecipientID == "" {
			response.BadRequest(w, "recipient_id is required")
			return
		}
		if req.MediaURL == "" {
			response.BadRequest(w, "media_url is required")
			return
		}
		if req.MediaType == "" {
			response.BadRequest(w, "media_type is required")
			return
		}

		result, err := h.policy.SendMediaMessage(r.Context(), policy.SendMediaMessageInput{
			AccountID:      req.AccountID,
			ConversationID: conversationID,
			RecipientID:    req.RecipientID,
			MediaURL:       req.MediaURL,
			MediaType:      req.MediaType,
		})
		if err != nil {
			handleDirectError(w, err)
			return
		}

		response.Created(w, SendMessageResponse{MessageID: result.MessageID})
	}
}

// GetStatistics handles GET /direct/statistics
func (h *DirectHandler) GetStatistics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		// Parse date range (default to last 30 days)
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -30)

		if s := r.URL.Query().Get("start_date"); s != "" {
			if parsed, err := time.Parse("2006-01-02", s); err == nil {
				startDate = parsed
			}
		}

		if e := r.URL.Query().Get("end_date"); e != "" {
			if parsed, err := time.Parse("2006-01-02", e); err == nil {
				endDate = parsed.Add(24*time.Hour - time.Second) // End of day
			}
		}

		stats, err := h.policy.GetStatistics(r.Context(), policy.GetStatisticsInput{
			AccountID: accountID,
			StartDate: startDate,
			EndDate:   endDate,
		})
		if err != nil {
			handleDirectError(w, err)
			return
		}

		response.OK(w, stats)
	}
}

// GetHeatmap handles GET /direct/heatmap
func (h *DirectHandler) GetHeatmap() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		// Parse date range (default to last 30 days)
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -30)

		if s := r.URL.Query().Get("start_date"); s != "" {
			if parsed, err := time.Parse("2006-01-02", s); err == nil {
				startDate = parsed
			}
		}

		if e := r.URL.Query().Get("end_date"); e != "" {
			if parsed, err := time.Parse("2006-01-02", e); err == nil {
				endDate = parsed.Add(24*time.Hour - time.Second) // End of day
			}
		}

		heatmap, err := h.policy.GetHeatmap(r.Context(), policy.GetHeatmapInput{
			AccountID: accountID,
			StartDate: startDate,
			EndDate:   endDate,
		})
		if err != nil {
			handleDirectError(w, err)
			return
		}

		response.OK(w, heatmap)
	}
}

func handleDirectError(w http.ResponseWriter, err error) {
	switch err {
	case entity.ErrConversationNotFound:
		response.NotFound(w, err.Error())
	case entity.ErrMessageNotFound:
		response.NotFound(w, err.Error())
	case entity.ErrEmptyMessage:
		response.BadRequest(w, err.Error())
	case entity.ErrMessageTooLong:
		response.BadRequest(w, err.Error())
	case entity.ErrInvalidMediaType:
		response.BadRequest(w, err.Error())
	case entity.ErrUnauthorized:
		response.Unauthorized(w, err.Error())
	case entity.ErrRateLimited:
		response.Error(w, http.StatusTooManyRequests, err.Error())
	default:
		response.InternalError(w, "internal server error")
	}
}
