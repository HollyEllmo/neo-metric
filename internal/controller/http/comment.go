package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/vadim/neo-metric/internal/domain/comment/entity"
	"github.com/vadim/neo-metric/internal/domain/comment/policy"
	"github.com/vadim/neo-metric/internal/httpx/response"
)

// CommentPolicy defines the interface for comment operations
type CommentPolicy interface {
	GetComments(ctx context.Context, in policy.GetCommentsInput) (*policy.GetCommentsOutput, error)
	GetReplies(ctx context.Context, in policy.GetRepliesInput) (*policy.GetCommentsOutput, error)
	CreateComment(ctx context.Context, in policy.CreateCommentInput) (*policy.CreateCommentOutput, error)
	Reply(ctx context.Context, in policy.ReplyInput) (*policy.ReplyOutput, error)
	Delete(ctx context.Context, in policy.DeleteInput) error
	Hide(ctx context.Context, in policy.HideInput) error
	GetStatistics(ctx context.Context, in policy.GetStatisticsInput) (*entity.CommentStatistics, error)
}

// CommentHandler handles HTTP requests for comments
type CommentHandler struct {
	policy CommentPolicy
}

// NewCommentHandler creates a new comment handler
func NewCommentHandler(p CommentPolicy) *CommentHandler {
	return &CommentHandler{policy: p}
}

// RegisterRoutes registers comment routes
func (h *CommentHandler) RegisterRoutes(r chi.Router) {
	r.Route("/comments", func(r chi.Router) {
		// Get comments for a media
		r.Get("/media/{mediaId}", h.GetComments())

		// Get statistics
		r.Get("/statistics", h.GetStatistics())

		// Get replies to a comment
		r.Get("/{commentId}/replies", h.GetReplies())

		// Create a comment on media
		r.Post("/media/{mediaId}", h.CreateComment())

		// Reply to a comment
		r.Post("/{commentId}/replies", h.Reply())

		// Delete a comment
		r.Delete("/{commentId}", h.Delete())

		// Hide/unhide a comment
		r.Post("/{commentId}/hide", h.Hide())
	})
}

// GetCommentsResponse represents the response for getting comments
type GetCommentsResponse struct {
	Comments   []entity.Comment `json:"comments"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// GetComments handles GET /comments/media/{mediaId}
func (h *CommentHandler) GetComments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mediaID := chi.URLParam(r, "mediaId")
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

		after := r.URL.Query().Get("after")

		result, err := h.policy.GetComments(r.Context(), policy.GetCommentsInput{
			AccountID: accountID,
			MediaID:   mediaID,
			Limit:     limit,
			After:     after,
		})
		if err != nil {
			handleCommentError(w, err)
			return
		}

		response.OK(w, GetCommentsResponse{
			Comments:   result.Comments,
			NextCursor: result.NextCursor,
			HasMore:    result.HasMore,
		})
	}
}

// GetReplies handles GET /comments/{commentId}/replies
func (h *CommentHandler) GetReplies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		commentID := chi.URLParam(r, "commentId")
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

		after := r.URL.Query().Get("after")

		result, err := h.policy.GetReplies(r.Context(), policy.GetRepliesInput{
			AccountID: accountID,
			CommentID: commentID,
			Limit:     limit,
			After:     after,
		})
		if err != nil {
			handleCommentError(w, err)
			return
		}

		response.OK(w, GetCommentsResponse{
			Comments:   result.Comments,
			NextCursor: result.NextCursor,
			HasMore:    result.HasMore,
		})
	}
}

// CreateCommentRequest represents the request body for creating a comment
type CreateCommentRequest struct {
	AccountID string `json:"account_id"`
	Message   string `json:"message"`
}

// CreateCommentResponse represents the response for creating a comment
type CreateCommentResponse struct {
	ID string `json:"id"`
}

// CreateComment handles POST /comments/media/{mediaId}
func (h *CommentHandler) CreateComment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mediaID := chi.URLParam(r, "mediaId")

		var req CreateCommentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}
		if req.Message == "" {
			response.BadRequest(w, "message is required")
			return
		}

		result, err := h.policy.CreateComment(r.Context(), policy.CreateCommentInput{
			AccountID: req.AccountID,
			MediaID:   mediaID,
			Message:   req.Message,
		})
		if err != nil {
			handleCommentError(w, err)
			return
		}

		response.Created(w, CreateCommentResponse{ID: result.ID})
	}
}

// ReplyRequest represents the request body for replying to a comment
type ReplyRequest struct {
	AccountID    string `json:"account_id"`
	Message      string `json:"message"`
	SendToDirect bool   `json:"send_to_direct"` // If true, also send the reply as a DM
}

// ReplyResponse represents the response for replying to a comment
type ReplyResponse struct {
	ID          string `json:"id"`
	DirectSent  bool   `json:"direct_sent,omitempty"`  // Whether the DM was sent
	DirectError string `json:"direct_error,omitempty"` // Error if DM failed (non-fatal)
}

// Reply handles POST /comments/{commentId}/replies
func (h *CommentHandler) Reply() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		commentID := chi.URLParam(r, "commentId")

		var req ReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}
		if req.Message == "" {
			response.BadRequest(w, "message is required")
			return
		}

		result, err := h.policy.Reply(r.Context(), policy.ReplyInput{
			AccountID:    req.AccountID,
			CommentID:    commentID,
			Message:      req.Message,
			SendToDirect: req.SendToDirect,
		})
		if err != nil {
			handleCommentError(w, err)
			return
		}

		response.Created(w, ReplyResponse{
			ID:          result.ID,
			DirectSent:  result.DirectSent,
			DirectError: result.DirectError,
		})
	}
}

// DeleteRequest represents the request body for deleting a comment
type DeleteCommentRequest struct {
	AccountID string `json:"account_id"`
}

// Delete handles DELETE /comments/{commentId}
func (h *CommentHandler) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		commentID := chi.URLParam(r, "commentId")
		accountID := r.URL.Query().Get("account_id")

		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		err := h.policy.Delete(r.Context(), policy.DeleteInput{
			AccountID: accountID,
			CommentID: commentID,
		})
		if err != nil {
			handleCommentError(w, err)
			return
		}

		response.NoContent(w)
	}
}

// HideRequest represents the request body for hiding a comment
type HideRequest struct {
	AccountID string `json:"account_id"`
	Hide      bool   `json:"hide"`
}

// Hide handles POST /comments/{commentId}/hide
func (h *CommentHandler) Hide() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		commentID := chi.URLParam(r, "commentId")

		var req HideRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "invalid JSON")
			return
		}

		if req.AccountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		err := h.policy.Hide(r.Context(), policy.HideInput{
			AccountID: req.AccountID,
			CommentID: commentID,
			Hide:      req.Hide,
		})
		if err != nil {
			handleCommentError(w, err)
			return
		}

		response.OK(w, map[string]bool{"hidden": req.Hide})
	}
}

// GetStatistics handles GET /comments/statistics
func (h *CommentHandler) GetStatistics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		if accountID == "" {
			response.BadRequest(w, "account_id is required")
			return
		}

		topPostsLimit := 5
		if l := r.URL.Query().Get("top_posts_limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				topPostsLimit = parsed
				if topPostsLimit > 20 {
					topPostsLimit = 20
				}
			}
		}

		stats, err := h.policy.GetStatistics(r.Context(), policy.GetStatisticsInput{
			AccountID:     accountID,
			TopPostsLimit: topPostsLimit,
		})
		if err != nil {
			handleCommentError(w, err)
			return
		}

		response.OK(w, stats)
	}
}

func handleCommentError(w http.ResponseWriter, err error) {
	switch err {
	case entity.ErrCommentNotFound:
		response.NotFound(w, err.Error())
	case entity.ErrMediaNotFound:
		response.NotFound(w, err.Error())
	case entity.ErrEmptyReplyText, entity.ErrReplyTextTooLong:
		response.BadRequest(w, err.Error())
	case entity.ErrUnauthorized:
		response.Unauthorized(w, err.Error())
	case entity.ErrCommentingDisabled:
		response.Error(w, http.StatusForbidden, err.Error())
	default:
		response.InternalError(w, "internal server error")
	}
}
