package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vadim/neo-metric/internal/httpx/response"
)

// AccountInfo represents an Instagram account
type AccountInfo struct {
	ID              string `json:"id"`
	InstagramUserID string `json:"instagram_user_id"`
	Username        string `json:"username"`
	HasAccessToken  bool   `json:"has_access_token"`
}

// AccountLister defines the interface for listing accounts
type AccountLister interface {
	ListAccounts(ctx context.Context) ([]AccountInfo, error)
}

// AccountHandler handles HTTP requests for Instagram accounts
type AccountHandler struct {
	lister AccountLister
}

// NewAccountHandler creates a new account handler
func NewAccountHandler(lister AccountLister) *AccountHandler {
	return &AccountHandler{lister: lister}
}

// RegisterRoutes registers account routes
func (h *AccountHandler) RegisterRoutes(r chi.Router) {
	r.Get("/accounts", h.List())
	r.Get("/accounts/{id}", h.Get())
}

// List handles GET /accounts
func (h *AccountHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accounts, err := h.lister.ListAccounts(r.Context())
		if err != nil {
			response.InternalError(w, "failed to list accounts")
			return
		}

		response.OK(w, map[string]interface{}{
			"accounts": accounts,
			"total":    len(accounts),
		})
	}
}

// Get handles GET /accounts/{id}
func (h *AccountHandler) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		accounts, err := h.lister.ListAccounts(r.Context())
		if err != nil {
			response.InternalError(w, "failed to get account")
			return
		}

		for _, acc := range accounts {
			if acc.ID == id {
				response.OK(w, acc)
				return
			}
		}

		response.NotFound(w, "account not found")
	}
}
