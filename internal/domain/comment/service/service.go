package service

import (
	"context"

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

// CommentsResult represents the result of fetching comments
type CommentsResult struct {
	Comments   []entity.Comment
	NextCursor string
	HasMore    bool
}

// Service handles business logic for comments
type Service struct {
	ig InstagramClient
}

// New creates a new comment service
func New(ig InstagramClient) *Service {
	return &Service{ig: ig}
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

	return s.ig.CreateComment(ctx, in.MediaID, in.AccessToken, in.Message)
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

	return s.ig.ReplyToComment(ctx, in.CommentID, in.AccessToken, in.Message)
}

// DeleteInput represents input for deleting a comment
type DeleteInput struct {
	CommentID   string
	AccessToken string
}

// Delete removes a comment
func (s *Service) Delete(ctx context.Context, in DeleteInput) error {
	return s.ig.DeleteComment(ctx, in.CommentID, in.AccessToken)
}

// HideInput represents input for hiding a comment
type HideInput struct {
	CommentID   string
	AccessToken string
	Hide        bool
}

// Hide hides or unhides a comment
func (s *Service) Hide(ctx context.Context, in HideInput) error {
	return s.ig.HideComment(ctx, in.CommentID, in.AccessToken, in.Hide)
}
