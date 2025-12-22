package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/vadim/neo-metric/internal/domain/publication/dao"
	"github.com/vadim/neo-metric/internal/domain/publication/entity"
)

// Service handles business logic for publications
type Service struct {
	publications dao.PublicationRepository
	media        dao.MediaRepository
}

// New creates a new publication service
func New(publications dao.PublicationRepository, media dao.MediaRepository) *Service {
	return &Service{
		publications: publications,
		media:        media,
	}
}

// CreateInput represents input for creating a publication
type CreateInput struct {
	AccountID   string
	Type        entity.PublicationType
	Caption     string
	Media       []MediaInput
	ScheduledAt *time.Time
}

// MediaInput represents input for a media item
type MediaInput struct {
	URL   string
	Type  entity.MediaType
	Order int
}

// CreatePublication creates a new publication
func (s *Service) CreatePublication(ctx context.Context, in CreateInput) (*entity.Publication, error) {
	now := time.Now()

	// Determine initial status
	status := entity.PublicationStatusDraft
	if in.ScheduledAt != nil {
		status = entity.PublicationStatusScheduled
	}

	// Build media items
	mediaItems := make([]entity.MediaItem, len(in.Media))
	for i, m := range in.Media {
		mediaItems[i] = entity.MediaItem{
			ID:        uuid.New().String(),
			URL:       m.URL,
			Type:      m.Type,
			Order:     m.Order,
			CreatedAt: now,
		}
	}

	pub := &entity.Publication{
		ID:          uuid.New().String(),
		AccountID:   in.AccountID,
		Type:        in.Type,
		Status:      status,
		Caption:     in.Caption,
		Media:       mediaItems,
		ScheduledAt: in.ScheduledAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Validate publication
	if err := pub.Validate(); err != nil {
		return nil, err
	}

	// Persist publication
	if err := s.publications.Create(ctx, pub); err != nil {
		return nil, err
	}

	// Persist media items
	for i := range pub.Media {
		if err := s.media.Create(ctx, pub.ID, &pub.Media[i]); err != nil {
			return nil, err
		}
	}

	return pub, nil
}

// UpdateInput represents input for updating a publication
type UpdateInput struct {
	ID          string
	Caption     *string
	Media       []MediaInput
	ScheduledAt *time.Time
	ClearSchedule bool // If true, clears scheduled_at and sets status to draft
}

// UpdatePublication updates an existing publication
func (s *Service) UpdatePublication(ctx context.Context, in UpdateInput) (*entity.Publication, error) {
	pub, err := s.publications.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, entity.ErrPublicationNotFound
	}

	if !pub.IsEditable() {
		return nil, entity.ErrPublicationNotEditable
	}

	// Update fields
	if in.Caption != nil {
		pub.Caption = *in.Caption
	}

	if in.ClearSchedule {
		pub.ScheduledAt = nil
		pub.Status = entity.PublicationStatusDraft
	} else if in.ScheduledAt != nil {
		pub.ScheduledAt = in.ScheduledAt
		pub.Status = entity.PublicationStatusScheduled
	}

	// Update media if provided
	if len(in.Media) > 0 {
		// Delete existing media
		if err := s.media.DeleteByPublicationID(ctx, pub.ID); err != nil {
			return nil, err
		}

		// Create new media
		now := time.Now()
		pub.Media = make([]entity.MediaItem, len(in.Media))
		for i, m := range in.Media {
			pub.Media[i] = entity.MediaItem{
				ID:        uuid.New().String(),
				URL:       m.URL,
				Type:      m.Type,
				Order:     m.Order,
				CreatedAt: now,
			}
			if err := s.media.Create(ctx, pub.ID, &pub.Media[i]); err != nil {
				return nil, err
			}
		}
	}

	pub.UpdatedAt = time.Now()

	// Validate before saving
	if err := pub.Validate(); err != nil {
		return nil, err
	}

	if err := s.publications.Update(ctx, pub); err != nil {
		return nil, err
	}

	return pub, nil
}

// GetPublication retrieves a publication by ID
func (s *Service) GetPublication(ctx context.Context, id string) (*entity.Publication, error) {
	pub, err := s.publications.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, entity.ErrPublicationNotFound
	}

	// Load media items
	media, err := s.media.GetByPublicationID(ctx, id)
	if err != nil {
		return nil, err
	}
	pub.Media = media

	return pub, nil
}

// DeletePublication deletes a publication
func (s *Service) DeletePublication(ctx context.Context, id string) error {
	pub, err := s.publications.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if pub == nil {
		return entity.ErrPublicationNotFound
	}

	if !pub.IsDeletable() {
		return entity.ErrPublicationNotDeletable
	}

	// Delete media first
	if err := s.media.DeleteByPublicationID(ctx, id); err != nil {
		return err
	}

	return s.publications.Delete(ctx, id)
}

// ListInput represents input for listing publications
type ListInput struct {
	AccountID string
	Type      *entity.PublicationType
	Status    *entity.PublicationStatus
	Year      *int
	Month     *int
	Limit     int
	Offset    int
}

// ListOutput represents output from listing publications
type ListOutput struct {
	Publications []entity.Publication
	Total        int64
}

// ListPublications retrieves publications with filtering
func (s *Service) ListPublications(ctx context.Context, in ListInput) (*ListOutput, error) {
	filter := dao.PublicationFilter{
		AccountID: in.AccountID,
		Type:      in.Type,
		Status:    in.Status,
		Year:      in.Year,
		Month:     in.Month,
	}

	opts := dao.ListOptions{
		Limit:  in.Limit,
		Offset: in.Offset,
		SortBy: "scheduled_at",
		Desc:   true,
	}

	if opts.Limit == 0 {
		opts.Limit = 50
	}

	publications, err := s.publications.List(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	total, err := s.publications.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Load media for each publication
	for i := range publications {
		media, err := s.media.GetByPublicationID(ctx, publications[i].ID)
		if err != nil {
			return nil, err
		}
		publications[i].Media = media
	}

	return &ListOutput{
		Publications: publications,
		Total:        total,
	}, nil
}

// GetScheduledForPublishing retrieves all publications ready to be published
func (s *Service) GetScheduledForPublishing(ctx context.Context) ([]entity.Publication, error) {
	pubs, err := s.publications.GetScheduledForPublishing(ctx, time.Now())
	if err != nil {
		return nil, err
	}

	// Load media for each publication
	for i := range pubs {
		media, err := s.media.GetByPublicationID(ctx, pubs[i].ID)
		if err != nil {
			return nil, err
		}
		pubs[i].Media = media
	}

	return pubs, nil
}

// MarkAsPublished marks a publication as successfully published
func (s *Service) MarkAsPublished(ctx context.Context, id string, instagramMediaID string) error {
	return s.publications.SetPublished(ctx, id, instagramMediaID, time.Now())
}

// MarkAsFailed marks a publication as failed with error message
func (s *Service) MarkAsFailed(ctx context.Context, id string, errorMsg string) error {
	return s.publications.UpdateStatus(ctx, id, entity.PublicationStatusError, errorMsg)
}

// SaveAsDraft saves a publication as draft (removes scheduled time)
func (s *Service) SaveAsDraft(ctx context.Context, id string) (*entity.Publication, error) {
	return s.UpdatePublication(ctx, UpdateInput{
		ID:            id,
		ClearSchedule: true,
	})
}

// Schedule schedules a publication for a specific time
func (s *Service) Schedule(ctx context.Context, id string, scheduledAt time.Time) (*entity.Publication, error) {
	return s.UpdatePublication(ctx, UpdateInput{
		ID:          id,
		ScheduledAt: &scheduledAt,
	})
}
