package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/vadim/neo-metric/internal/httpx/response"
)

// MaxUploadSize is the maximum allowed upload size (50MB)
const MaxUploadSize = 50 << 20

// MediaUploader defines the interface for uploading media
type MediaUploader interface {
	Upload(ctx context.Context, in MediaUploadInput) (*MediaUploadOutput, error)
}

// MediaUploadInput represents input for media upload
type MediaUploadInput struct {
	Reader      interface{ Read([]byte) (int, error) }
	ContentType string
	Size        int64
	Filename    string
}

// MediaUploadOutput represents output from media upload
type MediaUploadOutput struct {
	URL  string
	Key  string
	Size int64
}

// MediaHandler handles media upload HTTP requests
type MediaHandler struct {
	uploader MediaUploader
}

// NewMediaHandler creates a new media handler
func NewMediaHandler(uploader MediaUploader) *MediaHandler {
	return &MediaHandler{uploader: uploader}
}

// RegisterRoutes registers media routes
func (h *MediaHandler) RegisterRoutes(r chi.Router) {
	r.Post("/media/upload", h.Upload())
}

// UploadResponse represents the response from upload endpoint
type UploadResponse struct {
	URL  string `json:"url"`
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

// Upload handles POST /media/upload
func (h *MediaHandler) Upload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize)

		// Parse multipart form
		if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
			response.BadRequest(w, "file too large or invalid multipart form")
			return
		}

		// Get file from form
		file, header, err := r.FormFile("file")
		if err != nil {
			response.BadRequest(w, "missing file in request")
			return
		}
		defer file.Close()

		// Validate content type
		contentType := header.Header.Get("Content-Type")
		if !isAllowedMediaType(contentType) {
			response.BadRequest(w, fmt.Sprintf("unsupported media type: %s", contentType))
			return
		}

		// Upload to storage
		result, err := h.uploader.Upload(r.Context(), MediaUploadInput{
			Reader:      file,
			ContentType: contentType,
			Size:        header.Size,
			Filename:    header.Filename,
		})
		if err != nil {
			// Log error for debugging (in production, use proper logger)
			fmt.Printf("upload error: %v\n", err)
			response.InternalError(w, fmt.Sprintf("failed to upload file: %v", err))
			return
		}

		response.Created(w, UploadResponse{
			URL:  result.URL,
			Key:  result.Key,
			Size: result.Size,
		})
	}
}

// isAllowedMediaType checks if the content type is allowed for upload
func isAllowedMediaType(contentType string) bool {
	allowed := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"video/mp4",
		"video/quicktime",
	}

	for _, a := range allowed {
		if strings.EqualFold(contentType, a) {
			return true
		}
	}
	return false
}
