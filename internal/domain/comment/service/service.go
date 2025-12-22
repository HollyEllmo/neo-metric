package service

import (
	"context"
	"time"

	"github.com/vadim/neo-metric/internal/domain/comment/entity"
)

// InstagramClient defines the interface for Instagram API operations
type InstagramClient interface {
	GetComments(ctx context.Context, mediaID, accessToken string, limit int, after string) (*CommentsResult, error)
	GetCommentReplies(ctx context.Context, commentID, accessToken string, limit int, after string) (*CommentsResult, error)
	CreateComment(ctx context.Context, mediaID, accessToken, message string) (string, error)
	ReplyToComment(ctx context.Context, commentID, accessToken, message string) (string, error)
	DeleteComment(ctx context.Context, commentID, accessToken string) error
	HideComment(ctx context.Context, commentID, accessToken string, hide bool) error
}

// CommentRepository defines the interface for comment storage
type CommentRepository interface {
	Upsert(ctx context.Context, comment *entity.Comment) error
	UpsertBatch(ctx context.Context, comments []entity.Comment) error
	GetByID(ctx context.Context, id string) (*entity.Comment, error)
	GetByMediaID(ctx context.Context, mediaID string, limit int, offset int) ([]entity.Comment, error)
	GetReplies(ctx context.Context, parentID string, limit int, offset int) ([]entity.Comment, error)
	Delete(ctx context.Context, id string) error
	UpdateHidden(ctx context.Context, id string, hidden bool) error
	Count(ctx context.Context, mediaID string) (int64, error)
	CountReplies(ctx context.Context, parentID string) (int64, error)
}

// SyncStatus represents the synchronization status for a media's comments
type SyncStatus struct {
	InstagramMediaID string
	LastSyncedAt     time.Time
	NextCursor       string
	SyncComplete     bool
}

// SyncStatusRepository defines the interface for sync status tracking
type SyncStatusRepository interface {
	GetSyncStatus(ctx context.Context, mediaID string) (*SyncStatus, error)
	UpdateSyncStatus(ctx context.Context, status *SyncStatus) error
	GetMediaIDsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error)
}

// CommentsResult represents the result of fetching comments
type CommentsResult struct {
	Comments   []entity.Comment
	NextCursor string
	HasMore    bool
}

// Service handles business logic for comments
type Service struct {
	ig         InstagramClient
	repo       CommentRepository
	syncRepo   SyncStatusRepository
	syncMaxAge time.Duration // How old sync status can be before refreshing
}

// New creates a new comment service
func New(ig InstagramClient) *Service {
	return &Service{
		ig:         ig,
		syncMaxAge: 5 * time.Minute, // Default: refresh comments older than 5 minutes
	}
}

// NewWithRepo creates a new comment service with repository support
func NewWithRepo(ig InstagramClient, repo CommentRepository, syncRepo SyncStatusRepository) *Service {
	return &Service{
		ig:         ig,
		repo:       repo,
		syncRepo:   syncRepo,
		syncMaxAge: 5 * time.Minute,
	}
}

// GetCommentsInput represents input for getting comments
type GetCommentsInput struct {
	MediaID     string
	AccessToken string
	Limit       int
	After       string
}

// GetCommentsOutput represents output from getting comments
type GetCommentsOutput struct {
	Comments   []entity.Comment `json:"comments"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// GetComments retrieves comments for a media
func (s *Service) GetComments(ctx context.Context, in GetCommentsInput) (*GetCommentsOutput, error) {
	if in.Limit <= 0 {
		in.Limit = 50
	}

	// If we have a repository, try to use cached data
	if s.repo != nil && s.syncRepo != nil {
		return s.getCommentsWithCache(ctx, in)
	}

	// Fallback to direct Instagram API call
	return s.getCommentsFromInstagram(ctx, in)
}

// getCommentsFromInstagram fetches comments directly from Instagram
func (s *Service) getCommentsFromInstagram(ctx context.Context, in GetCommentsInput) (*GetCommentsOutput, error) {
	result, err := s.ig.GetComments(ctx, in.MediaID, in.AccessToken, in.Limit, in.After)
	if err != nil {
		return nil, err
	}

	return &GetCommentsOutput{
		Comments:   result.Comments,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

// getCommentsWithCache fetches comments using cache with background sync
func (s *Service) getCommentsWithCache(ctx context.Context, in GetCommentsInput) (*GetCommentsOutput, error) {
	// Check sync status
	syncStatus, err := s.syncRepo.GetSyncStatus(ctx, in.MediaID)
	if err != nil {
		return nil, err
	}

	// If never synced or sync is stale, fetch from Instagram first
	needsSync := syncStatus == nil || time.Since(syncStatus.LastSyncedAt) > s.syncMaxAge

	if needsSync {
		// Fetch from Instagram and save to DB
		if err := s.syncCommentsFromInstagram(ctx, in.MediaID, in.AccessToken); err != nil {
			// If sync fails but we have cached data, return that
			if syncStatus != nil {
				// Log error but continue with cached data
			} else {
				return nil, err
			}
		}
	}

	// Fetch from database
	offset := 0
	if in.After != "" {
		// For simplicity, treat After as offset (could be improved with cursor-based pagination)
		// In production, you'd parse the cursor properly
	}

	comments, err := s.repo.GetByMediaID(ctx, in.MediaID, in.Limit+1, offset)
	if err != nil {
		return nil, err
	}

	hasMore := len(comments) > in.Limit
	if hasMore {
		comments = comments[:in.Limit]
	}

	var nextCursor string
	if hasMore {
		// Simple offset-based cursor
		nextCursor = "" // Could implement proper cursor here
	}

	return &GetCommentsOutput{
		Comments:   comments,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// syncCommentsFromInstagram fetches all comments from Instagram and saves to DB
func (s *Service) syncCommentsFromInstagram(ctx context.Context, mediaID, accessToken string) error {
	var cursor string
	var allComments []entity.Comment

	for {
		result, err := s.ig.GetComments(ctx, mediaID, accessToken, 100, cursor)
		if err != nil {
			return err
		}

		allComments = append(allComments, result.Comments...)

		if !result.HasMore || result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	// Save all comments to DB
	if err := s.repo.UpsertBatch(ctx, allComments); err != nil {
		return err
	}

	// Update sync status
	return s.syncRepo.UpdateSyncStatus(ctx, &SyncStatus{
		InstagramMediaID: mediaID,
		LastSyncedAt:     time.Now(),
		SyncComplete:     true,
	})
}

// GetRepliesInput represents input for getting comment replies
type GetRepliesInput struct {
	CommentID   string
	AccessToken string
	Limit       int
	After       string
}

// GetReplies retrieves replies to a comment
func (s *Service) GetReplies(ctx context.Context, in GetRepliesInput) (*GetCommentsOutput, error) {
	if in.Limit <= 0 {
		in.Limit = 50
	}

	// If we have a repository, try to use cached data
	if s.repo != nil {
		return s.getRepliesWithCache(ctx, in)
	}

	// Fallback to direct Instagram API call
	result, err := s.ig.GetCommentReplies(ctx, in.CommentID, in.AccessToken, in.Limit, in.After)
	if err != nil {
		return nil, err
	}

	return &GetCommentsOutput{
		Comments:   result.Comments,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

// getRepliesWithCache fetches replies using cache
func (s *Service) getRepliesWithCache(ctx context.Context, in GetRepliesInput) (*GetCommentsOutput, error) {
	// Sync replies from Instagram
	result, err := s.ig.GetCommentReplies(ctx, in.CommentID, in.AccessToken, 100, "")
	if err != nil {
		// Try to return cached data on error
		replies, dbErr := s.repo.GetReplies(ctx, in.CommentID, in.Limit, 0)
		if dbErr != nil || len(replies) == 0 {
			return nil, err
		}
		return &GetCommentsOutput{
			Comments: replies,
			HasMore:  false,
		}, nil
	}

	// Save replies to DB
	if err := s.repo.UpsertBatch(ctx, result.Comments); err != nil {
		// Log error but continue
	}

	// Return from Instagram API result
	return &GetCommentsOutput{
		Comments:   result.Comments,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

// CreateCommentInput represents input for creating a comment
type CreateCommentInput struct {
	MediaID     string
	AccessToken string
	Message     string
}

// CreateComment creates a new comment on a media
func (s *Service) CreateComment(ctx context.Context, in CreateCommentInput) (string, error) {
	if err := entity.ValidateReplyText(in.Message); err != nil {
		return "", err
	}

	id, err := s.ig.CreateComment(ctx, in.MediaID, in.AccessToken, in.Message)
	if err != nil {
		return "", err
	}

	// Save to DB if repository is available
	if s.repo != nil {
		comment := &entity.Comment{
			ID:        id,
			MediaID:   in.MediaID,
			Text:      in.Message,
			Timestamp: time.Now(),
		}
		// Best effort - don't fail if DB save fails
		_ = s.repo.Upsert(ctx, comment)
	}

	return id, nil
}

// ReplyInput represents input for replying to a comment
type ReplyInput struct {
	CommentID   string
	AccessToken string
	Message     string
}

// Reply posts a reply to a comment
func (s *Service) Reply(ctx context.Context, in ReplyInput) (string, error) {
	if err := entity.ValidateReplyText(in.Message); err != nil {
		return "", err
	}

	id, err := s.ig.ReplyToComment(ctx, in.CommentID, in.AccessToken, in.Message)
	if err != nil {
		return "", err
	}

	// Save to DB if repository is available
	if s.repo != nil {
		comment := &entity.Comment{
			ID:        id,
			ParentID:  in.CommentID,
			Text:      in.Message,
			Timestamp: time.Now(),
		}
		// Best effort - don't fail if DB save fails
		_ = s.repo.Upsert(ctx, comment)
	}

	return id, nil
}

// DeleteInput represents input for deleting a comment
type DeleteInput struct {
	CommentID   string
	AccessToken string
}

// Delete removes a comment
func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	err := s.ig.DeleteComment(ctx, in.CommentID, in.AccessToken)
	if err != nil {
		return err
	}

	// Delete from DB if repository is available
	if s.repo != nil {
		// Best effort - don't fail if DB delete fails
		_ = s.repo.Delete(ctx, in.CommentID)
	}

	return nil
}

// HideInput represents input for hiding a comment
type HideInput struct {
	CommentID   string
	AccessToken string
	Hide        bool
}

// Hide hides or unhides a comment
func (s *Service) Hide(ctx context.Context, in HideInput) error {
	err := s.ig.HideComment(ctx, in.CommentID, in.AccessToken, in.Hide)
	if err != nil {
		return err
	}

	// Update in DB if repository is available
	if s.repo != nil {
		// Best effort - don't fail if DB update fails
		_ = s.repo.UpdateHidden(ctx, in.CommentID, in.Hide)
	}

	return nil
}

// SyncMediaComments syncs comments for a specific media (for scheduler use)
func (s *Service) SyncMediaComments(ctx context.Context, mediaID, accessToken string) error {
	if s.repo == nil || s.syncRepo == nil {
		return nil
	}
	return s.syncCommentsFromInstagram(ctx, mediaID, accessToken)
}

// GetMediaIDsNeedingSync returns media IDs that need comment synchronization
func (s *Service) GetMediaIDsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	if s.syncRepo == nil {
		return nil, nil
	}
	return s.syncRepo.GetMediaIDsNeedingSync(ctx, olderThan, limit)
}
