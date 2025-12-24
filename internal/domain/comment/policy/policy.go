package policy

import (
	"context"
	"fmt"

	"github.com/vadim/neo-metric/internal/domain/comment/entity"
	"github.com/vadim/neo-metric/internal/domain/comment/service"
)

// AccountProvider provides access token and user ID for an account
type AccountProvider interface {
	GetAccessToken(ctx context.Context, accountID string) (string, error)
	GetInstagramUserID(ctx context.Context, accountID string) (string, error)
}

// DirectSender sends direct messages
type DirectSender interface {
	SendMessage(ctx context.Context, accountID, recipientID, message string) error
}

// CommentService defines the interface for comment operations
type CommentService interface {
	GetComments(ctx context.Context, in service.GetCommentsInput) (*service.GetCommentsOutput, error)
	GetReplies(ctx context.Context, in service.GetRepliesInput) (*service.GetCommentsOutput, error)
	CreateComment(ctx context.Context, in service.CreateCommentInput) (string, error)
	Reply(ctx context.Context, in service.ReplyInput) (string, error)
	Delete(ctx context.Context, in service.DeleteInput) error
	Hide(ctx context.Context, in service.HideInput) error
	GetStatistics(ctx context.Context, accountID string, topPostsLimit int) (*entity.CommentStatistics, error)
	GetComment(ctx context.Context, commentID string) (*entity.Comment, error)
	SyncMediaComments(ctx context.Context, mediaID, accessToken string) error
}

// Policy handles business policies for comments
type Policy struct {
	svc      CommentService
	accounts AccountProvider
	direct   DirectSender // optional, for send_to_direct
}

// New creates a new comment policy
func New(svc CommentService, accounts AccountProvider) *Policy {
	return &Policy{
		svc:      svc,
		accounts: accounts,
	}
}

// WithDirectSender sets the DirectSender for send_to_direct functionality
func (p *Policy) WithDirectSender(ds DirectSender) *Policy {
	p.direct = ds
	return p
}

// GetCommentsInput represents input for getting comments
type GetCommentsInput struct {
	AccountID string
	MediaID   string
	Limit     int
	After     string
}

// GetCommentsOutput represents output from getting comments
type GetCommentsOutput struct {
	Comments   []entity.Comment `json:"comments"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// GetComments retrieves comments for a media
func (p *Policy) GetComments(ctx context.Context, in GetCommentsInput) (*GetCommentsOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}

	result, err := p.svc.GetComments(ctx, service.GetCommentsInput{
		MediaID:     in.MediaID,
		AccessToken: accessToken,
		Limit:       in.Limit,
		After:       in.After,
	})
	if err != nil {
		return nil, err
	}

	return &GetCommentsOutput{
		Comments:   result.Comments,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}

// GetRepliesInput represents input for getting replies
type GetRepliesInput struct {
	AccountID string
	CommentID string
	Limit     int
	After     string
}

// GetReplies retrieves replies to a comment
func (p *Policy) GetReplies(ctx context.Context, in GetRepliesInput) (*GetCommentsOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}

	result, err := p.svc.GetReplies(ctx, service.GetRepliesInput{
		CommentID:   in.CommentID,
		AccessToken: accessToken,
		Limit:       in.Limit,
		After:       in.After,
	})
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
	AccountID string
	MediaID   string
	Message   string
}

// CreateCommentOutput represents output from creating a comment
type CreateCommentOutput struct {
	ID string `json:"id"`
}

// CreateComment creates a new comment on a media
func (p *Policy) CreateComment(ctx context.Context, in CreateCommentInput) (*CreateCommentOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}

	id, err := p.svc.CreateComment(ctx, service.CreateCommentInput{
		MediaID:     in.MediaID,
		AccessToken: accessToken,
		Message:     in.Message,
	})
	if err != nil {
		return nil, err
	}

	return &CreateCommentOutput{ID: id}, nil
}

// ReplyInput represents input for replying to a comment
type ReplyInput struct {
	AccountID    string
	CommentID    string
	Message      string
	SendToDirect bool // If true, also send the reply as a DM to comment author
}

// ReplyOutput represents output from replying to a comment
type ReplyOutput struct {
	ID           string `json:"id"`
	DirectSent   bool   `json:"direct_sent,omitempty"`   // Whether the DM was sent
	DirectError  string `json:"direct_error,omitempty"`  // Error if DM failed (non-fatal)
}

// Reply posts a reply to a comment
func (p *Policy) Reply(ctx context.Context, in ReplyInput) (*ReplyOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}

	id, err := p.svc.Reply(ctx, service.ReplyInput{
		CommentID:   in.CommentID,
		AccessToken: accessToken,
		Message:     in.Message,
	})
	if err != nil {
		return nil, err
	}

	output := &ReplyOutput{ID: id}

	// Send to direct if requested
	if in.SendToDirect && p.direct != nil {
		// Get the original comment to find the author
		comment, err := p.svc.GetComment(ctx, in.CommentID)
		if err != nil {
			output.DirectError = fmt.Sprintf("failed to get comment: %v", err)
		} else if comment == nil {
			output.DirectError = "comment not found"
		} else if comment.AuthorID == "" {
			output.DirectError = "comment author ID not available"
		} else {
			// Send DM to comment author
			if err := p.direct.SendMessage(ctx, in.AccountID, comment.AuthorID, in.Message); err != nil {
				output.DirectError = fmt.Sprintf("failed to send DM: %v", err)
			} else {
				output.DirectSent = true
			}
		}
	}

	return output, nil
}

// DeleteInput represents input for deleting a comment
type DeleteInput struct {
	AccountID string
	CommentID string
}

// Delete removes a comment
func (p *Policy) Delete(ctx context.Context, in DeleteInput) error {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return err
	}

	return p.svc.Delete(ctx, service.DeleteInput{
		CommentID:   in.CommentID,
		AccessToken: accessToken,
	})
}

// HideInput represents input for hiding a comment
type HideInput struct {
	AccountID string
	CommentID string
	Hide      bool
}

// Hide hides or unhides a comment
func (p *Policy) Hide(ctx context.Context, in HideInput) error {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return err
	}

	return p.svc.Hide(ctx, service.HideInput{
		CommentID:   in.CommentID,
		AccessToken: accessToken,
		Hide:        in.Hide,
	})
}

// GetStatisticsInput represents input for getting comment statistics
type GetStatisticsInput struct {
	AccountID     string
	TopPostsLimit int
}

// GetStatistics retrieves aggregated comment statistics for an account
func (p *Policy) GetStatistics(ctx context.Context, in GetStatisticsInput) (*entity.CommentStatistics, error) {
	return p.svc.GetStatistics(ctx, in.AccountID, in.TopPostsLimit)
}

// SyncCommentsInput represents input for syncing comments
type SyncCommentsInput struct {
	AccountID string
	MediaID   string
}

// SyncComments manually syncs comments for a specific media
func (p *Policy) SyncComments(ctx context.Context, in SyncCommentsInput) error {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return err
	}

	return p.svc.SyncMediaComments(ctx, in.MediaID, accessToken)
}
