package dao

import (
	"context"
	"time"

	"github.com/vadim/neo-metric/internal/domain/publication/entity"
)

// PublicationFilter contains filters for listing publications
type PublicationFilter struct {
	AccountID string
	Type      *entity.PublicationType
	Status    *entity.PublicationStatus
	Year      *int
	Month     *int
}

// ListOptions contains pagination and sorting options
type ListOptions struct {
	Limit  int
	Offset int
	SortBy string // "scheduled_at", "created_at", "updated_at"
	Desc   bool
}

// PublicationRepository defines the interface for publication data access
// This interface will be implemented by the concrete DAO when the database is chosen
type PublicationRepository interface {
	// Create inserts a new publication into the database
	Create(ctx context.Context, pub *entity.Publication) error

	// GetByID retrieves a publication by its ID
	GetByID(ctx context.Context, id string) (*entity.Publication, error)

	// Update updates an existing publication
	Update(ctx context.Context, pub *entity.Publication) error

	// Delete removes a publication by ID
	Delete(ctx context.Context, id string) error

	// List retrieves publications with optional filtering and pagination
	List(ctx context.Context, filter PublicationFilter, opts ListOptions) ([]entity.Publication, error)

	// Count returns the total number of publications matching the filter
	Count(ctx context.Context, filter PublicationFilter) (int64, error)

	// GetScheduledForPublishing retrieves all scheduled publications that are due
	// (scheduled_at <= now and status = 'scheduled')
	GetScheduledForPublishing(ctx context.Context, now time.Time) ([]entity.Publication, error)

	// UpdateStatus updates only the status and related fields
	UpdateStatus(ctx context.Context, id string, status entity.PublicationStatus, errorMsg string) error

	// SetPublished marks a publication as published with Instagram media ID
	SetPublished(ctx context.Context, id string, instagramMediaID string, publishedAt time.Time) error

	// GetAccountIDByMediaID retrieves the account ID for a publication by its Instagram media ID
	GetAccountIDByMediaID(ctx context.Context, instagramMediaID string) (string, error)

	// GetStatistics retrieves aggregated publication statistics for an account
	GetStatistics(ctx context.Context, accountID string) (*entity.PublicationStatistics, error)
}

// MediaRepository defines the interface for media items data access
type MediaRepository interface {
	// Create inserts a new media item
	Create(ctx context.Context, publicationID string, media *entity.MediaItem) error

	// GetByPublicationID retrieves all media items for a publication
	GetByPublicationID(ctx context.Context, publicationID string) ([]entity.MediaItem, error)

	// Delete removes a media item by ID
	Delete(ctx context.Context, id string) error

	// DeleteByPublicationID removes all media items for a publication
	DeleteByPublicationID(ctx context.Context, publicationID string) error

	// UpdateOrder updates the order of media items
	UpdateOrder(ctx context.Context, publicationID string, mediaIDs []string) error
}

// AccountRepository defines the interface for Instagram account data access
// Note: Account management might be in Laravel backend, this is for local caching/reference
type AccountRepository interface {
	// GetAccessToken retrieves the access token for an account
	GetAccessToken(ctx context.Context, accountID string) (string, error)

	// GetInstagramUserID retrieves the Instagram user ID for an account
	GetInstagramUserID(ctx context.Context, accountID string) (string, error)
}
