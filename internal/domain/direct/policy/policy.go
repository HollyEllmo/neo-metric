package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/vadim/neo-metric/internal/domain/direct/entity"
	"github.com/vadim/neo-metric/internal/domain/direct/service"
)

// AccountProvider provides account information for authentication
type AccountProvider interface {
	GetAccessToken(ctx context.Context, accountID string) (string, error)
	GetInstagramUserID(ctx context.Context, accountID string) (string, error)
}

// DirectService defines the interface for the direct service
type DirectService interface {
	GetConversations(ctx context.Context, in service.GetConversationsInput) (*service.GetConversationsOutput, error)
	SearchConversations(ctx context.Context, in service.SearchConversationsInput) (*service.GetConversationsOutput, error)
	GetMessages(ctx context.Context, in service.GetMessagesInput) (*service.GetMessagesOutput, error)
	SendMessage(ctx context.Context, in service.SendMessageInput) (*service.SendMessageOutput, error)
	SendMediaMessage(ctx context.Context, in service.SendMediaMessageInput) (*service.SendMessageOutput, error)
	SyncConversations(ctx context.Context, accountID, userID, accessToken string) error
	SyncMessages(ctx context.Context, conversationID, userID, accessToken string) error
	GetStatistics(ctx context.Context, in service.GetStatisticsInput) (*entity.Statistics, error)
	GetHeatmap(ctx context.Context, in service.GetHeatmapInput) (*entity.Heatmap, error)
}

// Policy handles direct message operations with account authorization
type Policy struct {
	svc      DirectService
	accounts AccountProvider
}

// New creates a new direct policy
func New(svc DirectService, accounts AccountProvider) *Policy {
	return &Policy{
		svc:      svc,
		accounts: accounts,
	}
}

// GetConversationsInput represents input for getting conversations
type GetConversationsInput struct {
	AccountID string
	Limit     int
	Offset    int
}

// GetConversationsOutput represents output from getting conversations
type GetConversationsOutput struct {
	Conversations []entity.Conversation
	Total         int64
	HasMore       bool
}

// GetConversations retrieves conversations for an account
func (p *Policy) GetConversations(ctx context.Context, in GetConversationsInput) (*GetConversationsOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	userID, err := p.accounts.GetInstagramUserID(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting user ID: %w", err)
	}

	result, err := p.svc.GetConversations(ctx, service.GetConversationsInput{
		AccountID:   in.AccountID,
		UserID:      userID,
		AccessToken: accessToken,
		Limit:       in.Limit,
		Offset:      in.Offset,
	})
	if err != nil {
		return nil, err
	}

	return &GetConversationsOutput{
		Conversations: result.Conversations,
		Total:         result.Total,
		HasMore:       result.HasMore,
	}, nil
}

// SearchConversationsInput represents input for searching conversations
type SearchConversationsInput struct {
	AccountID string
	Query     string
	Limit     int
	Offset    int
}

// SearchConversations searches conversations by participant username/name
func (p *Policy) SearchConversations(ctx context.Context, in SearchConversationsInput) (*GetConversationsOutput, error) {
	result, err := p.svc.SearchConversations(ctx, service.SearchConversationsInput{
		AccountID: in.AccountID,
		Query:     in.Query,
		Limit:     in.Limit,
		Offset:    in.Offset,
	})
	if err != nil {
		return nil, err
	}

	return &GetConversationsOutput{
		Conversations: result.Conversations,
		Total:         result.Total,
		HasMore:       result.HasMore,
	}, nil
}

// GetMessagesInput represents input for getting messages
type GetMessagesInput struct {
	AccountID      string
	ConversationID string
	Limit          int
	Offset         int
}

// GetMessagesOutput represents output from getting messages
type GetMessagesOutput struct {
	Messages []entity.Message
	Total    int64
	HasMore  bool
}

// GetMessages retrieves messages for a conversation
func (p *Policy) GetMessages(ctx context.Context, in GetMessagesInput) (*GetMessagesOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	userID, err := p.accounts.GetInstagramUserID(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting user ID: %w", err)
	}

	result, err := p.svc.GetMessages(ctx, service.GetMessagesInput{
		AccountID:      in.AccountID,
		ConversationID: in.ConversationID,
		UserID:         userID,
		AccessToken:    accessToken,
		Limit:          in.Limit,
		Offset:         in.Offset,
	})
	if err != nil {
		return nil, err
	}

	return &GetMessagesOutput{
		Messages: result.Messages,
		Total:    result.Total,
		HasMore:  result.HasMore,
	}, nil
}

// SendMessageInput represents input for sending a message
type SendMessageInput struct {
	AccountID      string
	ConversationID string
	RecipientID    string
	Message        string
}

// SendMessageOutput represents output from sending a message
type SendMessageOutput struct {
	MessageID string
}

// SendMessage sends a text message
func (p *Policy) SendMessage(ctx context.Context, in SendMessageInput) (*SendMessageOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	userID, err := p.accounts.GetInstagramUserID(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting user ID: %w", err)
	}

	result, err := p.svc.SendMessage(ctx, service.SendMessageInput{
		AccountID:      in.AccountID,
		ConversationID: in.ConversationID,
		UserID:         userID,
		RecipientID:    in.RecipientID,
		AccessToken:    accessToken,
		Message:        in.Message,
	})
	if err != nil {
		return nil, err
	}

	return &SendMessageOutput{MessageID: result.MessageID}, nil
}

// SendMediaMessageInput represents input for sending a media message
type SendMediaMessageInput struct {
	AccountID      string
	ConversationID string
	RecipientID    string
	MediaURL       string
	MediaType      string
}

// SendMediaMessage sends a media message
func (p *Policy) SendMediaMessage(ctx context.Context, in SendMediaMessageInput) (*SendMessageOutput, error) {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	userID, err := p.accounts.GetInstagramUserID(ctx, in.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting user ID: %w", err)
	}

	result, err := p.svc.SendMediaMessage(ctx, service.SendMediaMessageInput{
		AccountID:      in.AccountID,
		ConversationID: in.ConversationID,
		UserID:         userID,
		RecipientID:    in.RecipientID,
		AccessToken:    accessToken,
		MediaURL:       in.MediaURL,
		MediaType:      in.MediaType,
	})
	if err != nil {
		return nil, err
	}

	return &SendMessageOutput{MessageID: result.MessageID}, nil
}

// GetStatisticsInput represents input for getting statistics
type GetStatisticsInput struct {
	AccountID string
	StartDate time.Time
	EndDate   time.Time
}

// GetStatistics returns DM statistics for an account
func (p *Policy) GetStatistics(ctx context.Context, in GetStatisticsInput) (*entity.Statistics, error) {
	return p.svc.GetStatistics(ctx, service.GetStatisticsInput{
		AccountID: in.AccountID,
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
	})
}

// GetHeatmapInput represents input for getting heatmap
type GetHeatmapInput struct {
	AccountID string
	StartDate time.Time
	EndDate   time.Time
}

// GetHeatmap returns activity heatmap for an account
func (p *Policy) GetHeatmap(ctx context.Context, in GetHeatmapInput) (*entity.Heatmap, error) {
	return p.svc.GetHeatmap(ctx, service.GetHeatmapInput{
		AccountID: in.AccountID,
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
	})
}

// SyncConversationsInput represents input for syncing conversations
type SyncConversationsInput struct {
	AccountID string
}

// SyncConversations manually triggers conversation sync for an account
func (p *Policy) SyncConversations(ctx context.Context, in SyncConversationsInput) error {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return fmt.Errorf("getting access token: %w", err)
	}

	userID, err := p.accounts.GetInstagramUserID(ctx, in.AccountID)
	if err != nil {
		return fmt.Errorf("getting user ID: %w", err)
	}

	return p.svc.SyncConversations(ctx, in.AccountID, userID, accessToken)
}

// SyncMessagesInput represents input for syncing messages
type SyncMessagesInput struct {
	AccountID      string
	ConversationID string
}

// SyncMessages manually triggers message sync for a specific conversation
func (p *Policy) SyncMessages(ctx context.Context, in SyncMessagesInput) error {
	accessToken, err := p.accounts.GetAccessToken(ctx, in.AccountID)
	if err != nil {
		return fmt.Errorf("getting access token: %w", err)
	}

	userID, err := p.accounts.GetInstagramUserID(ctx, in.AccountID)
	if err != nil {
		return fmt.Errorf("getting user ID: %w", err)
	}

	return p.svc.SyncMessages(ctx, in.ConversationID, userID, accessToken)
}
