package policy

import (
	"context"
	"time"

	"github.com/vadim/neo-metric/internal/domain/publication/entity"
	"github.com/vadim/neo-metric/internal/domain/publication/service"
)

// InstagramPublisher defines the interface for Instagram publishing operations
// This interface is defined here (consumer) not in the upstream package (provider)
type InstagramPublisher interface {
	Publish(ctx context.Context, in PublishInput) (*PublishOutput, error)
	Delete(ctx context.Context, mediaID, accessToken string) error
}

// PublishInput represents input for publishing
type PublishInput struct {
	UserID      string
	AccessToken string
	Publication *entity.Publication
}

// PublishOutput represents output from publishing
type PublishOutput struct {
	InstagramMediaID string
	Permalink        string
}

// AccountProvider defines the interface for getting account credentials
type AccountProvider interface {
	GetAccessToken(ctx context.Context, accountID string) (string, error)
	GetInstagramUserID(ctx context.Context, accountID string) (string, error)
	GetUsername(ctx context.Context, accountID string) (string, error)
}

// Policy orchestrates publication use-cases
type Policy struct {
	svc      *service.Service
	ig       InstagramPublisher
	accounts AccountProvider
}

// New creates a new publication policy
func New(svc *service.Service, ig InstagramPublisher, accounts AccountProvider) *Policy {
	return &Policy{
		svc:      svc,
		ig:       ig,
		accounts: accounts,
	}
}

// CreatePublicationInput represents input for creating a publication
type CreatePublicationInput struct {
	AccountID   string
	Type        entity.PublicationType
	Caption     string
	Media       []MediaInput
	ReelOptions *entity.ReelOptions // Optional settings for Reels
	ScheduledAt *time.Time
	PublishNow  bool // If true, publish immediately after creation
}

// MediaInput represents input for a media item
type MediaInput struct {
	URL   string
	Type  entity.MediaType
	Order int
}

// CreatePublicationOutput represents output from creating a publication
type CreatePublicationOutput struct {
	Publication *entity.Publication
}

// CreatePublication creates a new publication (draft or scheduled)
func (p *Policy) CreatePublication(ctx context.Context, in CreatePublicationInput) (*CreatePublicationOutput, error) {
	// Validate publication type
	if !isValidPublicationType(in.Type) {
		return nil, entity.ErrInvalidPublicationType
	}

	// Convert media input
	mediaInput := make([]service.MediaInput, len(in.Media))
	for i, m := range in.Media {
		mediaInput[i] = service.MediaInput{
			URL:   m.URL,
			Type:  m.Type,
			Order: m.Order,
		}
	}

	pub, err := p.svc.CreatePublication(ctx, service.CreateInput{
		AccountID:   in.AccountID,
		Type:        in.Type,
		Caption:     in.Caption,
		Media:       mediaInput,
		ReelOptions: in.ReelOptions,
		ScheduledAt: in.ScheduledAt,
	})
	if err != nil {
		return nil, err
	}

	// If publish_now is set, publish immediately
	if in.PublishNow {
		pub, err = p.PublishNow(ctx, pub.ID)
		if err != nil {
			return nil, err
		}
	}

	return &CreatePublicationOutput{Publication: pub}, nil
}

// UpdatePublicationInput represents input for updating a publication
type UpdatePublicationInput struct {
	ID            string
	Caption       *string
	Media         []MediaInput
	ScheduledAt   *time.Time
	ClearSchedule bool
}

// UpdatePublicationOutput represents output from updating a publication
type UpdatePublicationOutput struct {
	Publication *entity.Publication
}

// UpdatePublication updates an existing publication
func (p *Policy) UpdatePublication(ctx context.Context, in UpdatePublicationInput) (*UpdatePublicationOutput, error) {
	var mediaInput []service.MediaInput
	if len(in.Media) > 0 {
		mediaInput = make([]service.MediaInput, len(in.Media))
		for i, m := range in.Media {
			mediaInput[i] = service.MediaInput{
				URL:   m.URL,
				Type:  m.Type,
				Order: m.Order,
			}
		}
	}

	pub, err := p.svc.UpdatePublication(ctx, service.UpdateInput{
		ID:            in.ID,
		Caption:       in.Caption,
		Media:         mediaInput,
		ScheduledAt:   in.ScheduledAt,
		ClearSchedule: in.ClearSchedule,
	})
	if err != nil {
		return nil, err
	}

	return &UpdatePublicationOutput{Publication: pub}, nil
}

// GetPublication retrieves a publication by ID
func (p *Policy) GetPublication(ctx context.Context, id string) (*entity.Publication, error) {
	return p.svc.GetPublication(ctx, id)
}

// DeletePublicationInput represents input for deleting a publication
type DeletePublicationInput struct {
	ID string
}

// DeletePublication deletes a publication
// Note: Instagram Graph API does not support deleting published media.
// Published posts must be deleted manually through the Instagram app.
func (p *Policy) DeletePublication(ctx context.Context, in DeletePublicationInput) error {
	// Verify publication exists
	if _, err := p.svc.GetPublication(ctx, in.ID); err != nil {
		return err
	}

	// Delete from local database
	// Note: If the publication was published to Instagram, it will remain there
	// as Instagram API does not support deletion of published content
	return p.svc.DeletePublication(ctx, in.ID)
}

// ListPublicationsInput represents input for listing publications
type ListPublicationsInput struct {
	AccountID string
	Type      *entity.PublicationType
	Status    *entity.PublicationStatus
	Year      *int
	Month     *int
	Limit     int
	Offset    int
}

// ListPublicationsOutput represents output from listing publications
type ListPublicationsOutput struct {
	Publications []entity.Publication
	Total        int64
}

// ListPublications retrieves publications with filtering
func (p *Policy) ListPublications(ctx context.Context, in ListPublicationsInput) (*ListPublicationsOutput, error) {
	out, err := p.svc.ListPublications(ctx, service.ListInput{
		AccountID: in.AccountID,
		Type:      in.Type,
		Status:    in.Status,
		Year:      in.Year,
		Month:     in.Month,
		Limit:     in.Limit,
		Offset:    in.Offset,
	})
	if err != nil {
		return nil, err
	}

	return &ListPublicationsOutput{
		Publications: out.Publications,
		Total:        out.Total,
	}, nil
}

// PublishNow immediately publishes a publication to Instagram
func (p *Policy) PublishNow(ctx context.Context, id string) (*entity.Publication, error) {
	pub, err := p.svc.GetPublication(ctx, id)
	if err != nil {
		return nil, err
	}

	if pub.Status == entity.PublicationStatusPublished {
		return pub, nil // Already published
	}

	if !pub.CanPublish() && pub.Status != entity.PublicationStatusDraft {
		return nil, entity.ErrPublicationNotEditable
	}

	// Get account credentials
	accessToken, err := p.accounts.GetAccessToken(ctx, pub.AccountID)
	if err != nil {
		return nil, err
	}

	userID, err := p.accounts.GetInstagramUserID(ctx, pub.AccountID)
	if err != nil {
		return nil, err
	}

	// Publish to Instagram
	result, err := p.ig.Publish(ctx, PublishInput{
		UserID:      userID,
		AccessToken: accessToken,
		Publication: pub,
	})
	if err != nil {
		// Mark as failed
		_ = p.svc.MarkAsFailed(ctx, id, err.Error())
		return nil, err
	}

	// Mark as published
	if err := p.svc.MarkAsPublished(ctx, id, result.InstagramMediaID); err != nil {
		return nil, err
	}

	// Refresh and return
	return p.svc.GetPublication(ctx, id)
}

// SchedulePublication schedules a publication for a specific time
func (p *Policy) SchedulePublication(ctx context.Context, id string, scheduledAt time.Time) (*entity.Publication, error) {
	if scheduledAt.Before(time.Now()) {
		return nil, entity.ErrScheduledTimeInPast
	}

	return p.svc.Schedule(ctx, id, scheduledAt)
}

// SaveAsDraft saves a publication as draft (removes scheduling)
func (p *Policy) SaveAsDraft(ctx context.Context, id string) (*entity.Publication, error) {
	return p.svc.SaveAsDraft(ctx, id)
}

// ProcessScheduledPublications processes all scheduled publications that are due
// This should be called by a cron job or scheduler
func (p *Policy) ProcessScheduledPublications(ctx context.Context) error {
	pubs, err := p.svc.GetScheduledForPublishing(ctx)
	if err != nil {
		return err
	}

	for _, pub := range pubs {
		// Process each publication
		_, err := p.PublishNow(ctx, pub.ID)
		if err != nil {
			// Error is already logged in PublishNow via MarkAsFailed
			continue
		}
	}

	return nil
}

// GetStatistics retrieves publication statistics for an account
func (p *Policy) GetStatistics(ctx context.Context, accountID string) (*entity.PublicationStatistics, error) {
	return p.svc.GetStatistics(ctx, accountID)
}

func isValidPublicationType(t entity.PublicationType) bool {
	switch t {
	case entity.PublicationTypePost, entity.PublicationTypeStory, entity.PublicationTypeReel:
		return true
	default:
		return false
	}
}
