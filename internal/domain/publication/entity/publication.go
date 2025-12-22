package entity

import (
	"time"
)

// PublicationType represents the type of Instagram publication
type PublicationType string

const (
	PublicationTypePost  PublicationType = "post"
	PublicationTypeStory PublicationType = "story"
	PublicationTypeReel  PublicationType = "reel"
)

// PublicationStatus represents the current status of a publication
type PublicationStatus string

const (
	PublicationStatusDraft     PublicationStatus = "draft"
	PublicationStatusScheduled PublicationStatus = "scheduled"
	PublicationStatusPublished PublicationStatus = "published"
	PublicationStatusError     PublicationStatus = "error"
)

// MediaType represents the type of media file
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

// MediaItem represents a single media file attached to a publication
type MediaItem struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Type      MediaType `json:"type"`
	Order     int       `json:"order"`
	CreatedAt time.Time `json:"created_at"`
}

// Publication represents an Instagram publication (post, story, or reel)
type Publication struct {
	ID               string            `json:"id"`
	AccountID        string            `json:"account_id"`
	InstagramMediaID string            `json:"instagram_media_id,omitempty"` // ID from Instagram after publishing
	Type             PublicationType   `json:"type"`
	Status           PublicationStatus `json:"status"`
	Caption          string            `json:"caption"`
	Media            []MediaItem       `json:"media"`
	ScheduledAt      *time.Time        `json:"scheduled_at,omitempty"`
	PublishedAt      *time.Time        `json:"published_at,omitempty"`
	ErrorMessage     string            `json:"error_message,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

// IsEditable returns true if the publication can be edited
func (p *Publication) IsEditable() bool {
	return p.Status == PublicationStatusDraft || p.Status == PublicationStatusScheduled
}

// IsDeletable returns true if the publication can be deleted
func (p *Publication) IsDeletable() bool {
	return p.Status != PublicationStatusPublished
}

// CanPublish returns true if the publication is ready for publishing
func (p *Publication) CanPublish() bool {
	return p.Status == PublicationStatusScheduled && len(p.Media) > 0
}

// Validate validates the publication according to Instagram rules
func (p *Publication) Validate() error {
	if p.AccountID == "" {
		return ErrEmptyAccountID
	}

	if len(p.Media) == 0 {
		return ErrNoMedia
	}

	// Validate media count based on publication type
	switch p.Type {
	case PublicationTypePost:
		if len(p.Media) > 10 {
			return ErrTooManyMediaItems
		}
	case PublicationTypeStory, PublicationTypeReel:
		if len(p.Media) > 1 {
			return ErrSingleMediaRequired
		}
	}

	// Validate caption length (Instagram limit is 2200, but spec says 1100)
	if len(p.Caption) > 2200 {
		return ErrCaptionTooLong
	}

	// Validate scheduled time is in the future
	if p.Status == PublicationStatusScheduled && p.ScheduledAt != nil {
		if p.ScheduledAt.Before(time.Now()) {
			return ErrScheduledTimeInPast
		}
	}

	return nil
}
