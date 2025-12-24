package instagram

import (
	"context"
	"fmt"
	"time"

	"github.com/vadim/neo-metric/internal/domain/publication/entity"
)

// Publisher handles the complete publishing workflow for Instagram content
type Publisher struct {
	client *Client
}

// NewPublisher creates a new Instagram publisher
func NewPublisher(client *Client) *Publisher {
	return &Publisher{client: client}
}

// PublishInput represents input for publishing content
type PublishInput struct {
	UserID      string
	AccessToken string
	Publication *entity.Publication
}

// PublishOutput represents output from publishing content
type PublishOutput struct {
	InstagramMediaID string
	Permalink        string
}

// Publish publishes a publication to Instagram
// Handles the complete 3-step workflow: create container -> wait for processing -> publish
func (p *Publisher) Publish(ctx context.Context, in PublishInput) (*PublishOutput, error) {
	pub := in.Publication

	switch pub.Type {
	case entity.PublicationTypePost:
		return p.publishPost(ctx, in)
	case entity.PublicationTypeStory:
		return p.publishStory(ctx, in)
	case entity.PublicationTypeReel:
		return p.publishReel(ctx, in)
	default:
		return nil, entity.ErrInvalidPublicationType
	}
}

// publishPost publishes a feed post (single image, video, or carousel)
func (p *Publisher) publishPost(ctx context.Context, in PublishInput) (*PublishOutput, error) {
	pub := in.Publication

	var containerID string
	var err error

	if len(pub.Media) == 1 {
		// Single media post
		containerID, err = p.createSingleMediaContainer(ctx, in.UserID, in.AccessToken, pub.Media[0], pub.Caption, false)
	} else {
		// Carousel post
		containerID, err = p.createCarouselContainer(ctx, in.UserID, in.AccessToken, pub.Media, pub.Caption)
	}

	if err != nil {
		return nil, fmt.Errorf("creating media container: %w", err)
	}

	// Wait for container to be ready (for video content)
	if err := p.waitForContainer(ctx, containerID, in.AccessToken); err != nil {
		return nil, fmt.Errorf("waiting for container: %w", err)
	}

	// Publish
	return p.publishContainer(ctx, in.UserID, in.AccessToken, containerID)
}

// publishStory publishes a story
func (p *Publisher) publishStory(ctx context.Context, in PublishInput) (*PublishOutput, error) {
	pub := in.Publication

	if len(pub.Media) != 1 {
		return nil, entity.ErrSingleMediaRequired
	}

	media := pub.Media[0]
	mediaType := MediaTypeStories

	containerIn := CreateMediaContainerInput{
		UserID:      in.UserID,
		AccessToken: in.AccessToken,
		MediaType:   mediaType,
	}

	if media.Type == entity.MediaTypeImage {
		containerIn.ImageURL = media.URL
	} else {
		containerIn.VideoURL = media.URL
	}

	containerOut, err := p.client.CreateMediaContainer(ctx, containerIn)
	if err != nil {
		return nil, fmt.Errorf("creating story container: %w", err)
	}

	// Wait for processing
	if err := p.waitForContainer(ctx, containerOut.ID, in.AccessToken); err != nil {
		return nil, fmt.Errorf("waiting for story container: %w", err)
	}

	return p.publishContainer(ctx, in.UserID, in.AccessToken, containerOut.ID)
}

// publishReel publishes a reel
func (p *Publisher) publishReel(ctx context.Context, in PublishInput) (*PublishOutput, error) {
	pub := in.Publication

	if len(pub.Media) != 1 {
		return nil, entity.ErrSingleMediaRequired
	}

	media := pub.Media[0]
	if media.Type != entity.MediaTypeVideo {
		return nil, fmt.Errorf("reels require video content")
	}

	containerIn := CreateMediaContainerInput{
		UserID:      in.UserID,
		AccessToken: in.AccessToken,
		VideoURL:    media.URL,
		MediaType:   MediaTypeReels,
		Caption:     pub.Caption,
	}

	// Apply reel-specific options if provided
	if pub.ReelOptions != nil {
		containerIn.ShareToFeed = pub.ReelOptions.ShareToFeed
		containerIn.CoverURL = pub.ReelOptions.CoverURL
		containerIn.ThumbOffset = pub.ReelOptions.ThumbOffset
		containerIn.AudioName = pub.ReelOptions.AudioName
		containerIn.LocationID = pub.ReelOptions.LocationID
		containerIn.CollaboratorUsernames = pub.ReelOptions.CollaboratorUsernames
	}

	containerOut, err := p.client.CreateMediaContainer(ctx, containerIn)
	if err != nil {
		return nil, fmt.Errorf("creating reel container: %w", err)
	}

	// Reels require waiting for video processing
	if err := p.waitForContainer(ctx, containerOut.ID, in.AccessToken); err != nil {
		return nil, fmt.Errorf("waiting for reel container: %w", err)
	}

	return p.publishContainer(ctx, in.UserID, in.AccessToken, containerOut.ID)
}

// createSingleMediaContainer creates a container for a single media item
func (p *Publisher) createSingleMediaContainer(ctx context.Context, userID, accessToken string, media entity.MediaItem, caption string, isCarouselItem bool) (string, error) {
	containerIn := CreateMediaContainerInput{
		UserID:      userID,
		AccessToken: accessToken,
		IsCarousel:  isCarouselItem,
	}

	if media.Type == entity.MediaTypeImage {
		containerIn.ImageURL = media.URL
	} else {
		containerIn.VideoURL = media.URL
	}

	if !isCarouselItem {
		containerIn.Caption = caption
	}

	containerOut, err := p.client.CreateMediaContainer(ctx, containerIn)
	if err != nil {
		return "", err
	}

	return containerOut.ID, nil
}

// createCarouselContainer creates a carousel container with multiple media items
func (p *Publisher) createCarouselContainer(ctx context.Context, userID, accessToken string, media []entity.MediaItem, caption string) (string, error) {
	// First, create containers for each carousel item
	childIDs := make([]string, len(media))

	for i, m := range media {
		childID, err := p.createSingleMediaContainer(ctx, userID, accessToken, m, "", true)
		if err != nil {
			return "", fmt.Errorf("creating carousel item %d: %w", i, err)
		}

		// Wait for video items to be processed
		if m.Type == entity.MediaTypeVideo {
			if err := p.waitForContainer(ctx, childID, accessToken); err != nil {
				return "", fmt.Errorf("waiting for carousel item %d: %w", i, err)
			}
		}

		childIDs[i] = childID
	}

	// Create the carousel container
	containerIn := CreateMediaContainerInput{
		UserID:      userID,
		AccessToken: accessToken,
		MediaType:   MediaTypeCarousel,
		Caption:     caption,
		Children:    childIDs,
	}

	containerOut, err := p.client.CreateMediaContainer(ctx, containerIn)
	if err != nil {
		return "", fmt.Errorf("creating carousel container: %w", err)
	}

	return containerOut.ID, nil
}

// waitForContainer waits for a media container to be ready for publishing
func (p *Publisher) waitForContainer(ctx context.Context, containerID, accessToken string) error {
	maxAttempts := 30
	pollInterval := 5 * time.Second

	for i := 0; i < maxAttempts; i++ {
		status, err := p.client.GetContainerStatus(ctx, GetContainerStatusInput{
			ContainerID: containerID,
			AccessToken: accessToken,
		})
		if err != nil {
			return fmt.Errorf("checking container status: %w", err)
		}

		switch status.Status {
		case ContainerStatusFinished:
			return nil
		case ContainerStatusError:
			return fmt.Errorf("container error: %s", status.ErrorMessage)
		case ContainerStatusExpired:
			return fmt.Errorf("container expired")
		case ContainerStatusInProgress:
			// Continue waiting
		case ContainerStatusPublished:
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			// Continue polling
		}
	}

	return entity.ErrContainerNotReady
}

// publishContainer publishes a container and returns the Instagram media ID
func (p *Publisher) publishContainer(ctx context.Context, userID, accessToken, containerID string) (*PublishOutput, error) {
	publishOut, err := p.client.PublishMedia(ctx, PublishMediaInput{
		UserID:      userID,
		AccessToken: accessToken,
		ContainerID: containerID,
	})
	if err != nil {
		return nil, fmt.Errorf("publishing media: %w", err)
	}

	// Get permalink
	mediaDetails, err := p.client.GetMedia(ctx, GetMediaInput{
		MediaID:     publishOut.ID,
		AccessToken: accessToken,
		Fields:      []string{"id", "permalink"},
	})
	if err != nil {
		// Non-fatal error, we still have the media ID
		return &PublishOutput{
			InstagramMediaID: publishOut.ID,
		}, nil
	}

	return &PublishOutput{
		InstagramMediaID: publishOut.ID,
		Permalink:        mediaDetails.Permalink,
	}, nil
}

// Delete deletes a published media from Instagram
func (p *Publisher) Delete(ctx context.Context, mediaID, accessToken string) error {
	return p.client.DeleteMedia(ctx, DeleteMediaInput{
		MediaID:     mediaID,
		AccessToken: accessToken,
	})
}
