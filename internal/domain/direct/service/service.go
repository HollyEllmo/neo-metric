package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vadim/neo-metric/internal/domain/direct/entity"
)

// InstagramClient defines the interface for Instagram DM API operations
type InstagramClient interface {
	GetConversations(ctx context.Context, userID, accessToken string, limit int, after string) (*ConversationsResult, error)
	GetMessages(ctx context.Context, conversationID, userID, accessToken string, limit int, after string) (*MessagesResult, error)
	SendMessage(ctx context.Context, userID, recipientID, accessToken, message string) (*SendMessageResult, error)
	SendMediaMessage(ctx context.Context, userID, recipientID, accessToken, mediaURL, mediaType string) (*SendMessageResult, error)
	GetParticipant(ctx context.Context, userID, accessToken string) (*ParticipantResult, error)
}

// ConversationRepository defines the interface for conversation storage
type ConversationRepository interface {
	Upsert(ctx context.Context, conv *entity.Conversation) error
	UpsertBatch(ctx context.Context, convs []entity.Conversation) error
	GetByID(ctx context.Context, id string) (*entity.Conversation, error)
	GetByAccountID(ctx context.Context, accountID string, limit, offset int) ([]entity.Conversation, error)
	Search(ctx context.Context, accountID, query string, limit, offset int) ([]entity.Conversation, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, accountID string) (int64, error)
}

// MessageRepository defines the interface for message storage
type MessageRepository interface {
	Upsert(ctx context.Context, msg *entity.Message) error
	UpsertBatch(ctx context.Context, msgs []entity.Message) error
	GetByID(ctx context.Context, id string) (*entity.Message, error)
	GetByConversationID(ctx context.Context, conversationID string, limit, offset int) ([]entity.Message, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, conversationID string) (int64, error)
	GetStatistics(ctx context.Context, filter entity.StatisticsFilter) (*entity.Statistics, error)
	GetHeatmap(ctx context.Context, filter entity.StatisticsFilter) (*entity.Heatmap, error)
}

// ConversationSyncRepository defines sync status tracking for conversations
type ConversationSyncRepository interface {
	GetSyncStatus(ctx context.Context, conversationID string) (*ConversationSyncStatus, error)
	UpdateSyncStatus(ctx context.Context, status *ConversationSyncStatus) error
	GetConversationsNeedingSync(ctx context.Context, accountID string, olderThan time.Duration, limit int) ([]string, error)
	IncrementRetryCount(ctx context.Context, conversationID string, lastError string, maxRetries int) error
	ResetRetryCount(ctx context.Context, conversationID string) error
}

// AccountSyncRepository defines sync status tracking for accounts
type AccountSyncRepository interface {
	GetSyncStatus(ctx context.Context, accountID string) (*AccountSyncStatus, error)
	UpdateSyncStatus(ctx context.Context, status *AccountSyncStatus) error
	GetAccountsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error)
	IncrementRetryCount(ctx context.Context, accountID string, lastError string, maxRetries int) error
	ResetRetryCount(ctx context.Context, accountID string) error
}

// ConversationsResult from Instagram API
type ConversationsResult struct {
	Conversations []entity.Conversation
	NextCursor    string
	HasMore       bool
}

// MessagesResult from Instagram API
type MessagesResult struct {
	Messages   []entity.Message
	NextCursor string
	HasMore    bool
}

// SendMessageResult from Instagram API
type SendMessageResult struct {
	MessageID string
}

// ParticipantResult from Instagram API
type ParticipantResult struct {
	ID             string
	Username       string
	Name           string
	AvatarURL      string
	FollowersCount int
}

// ConversationSyncStatus tracks sync state per conversation
type ConversationSyncStatus struct {
	ConversationID         string
	LastSyncedAt           time.Time
	NextCursor             string
	SyncComplete           bool
	OldestMessageTimestamp *time.Time
	RetryCount             int
	Failed                 bool
	LastError              string
}

// AccountSyncStatus tracks sync state per account
type AccountSyncStatus struct {
	AccountID    string
	LastSyncedAt time.Time
	NextCursor   string
	SyncComplete bool
	RetryCount   int
	Failed       bool
	LastError    string
}

// Service handles DM business logic
type Service struct {
	ig              InstagramClient
	convRepo        ConversationRepository
	msgRepo         MessageRepository
	convSyncRepo    ConversationSyncRepository
	accountSyncRepo AccountSyncRepository
	syncMaxAge      time.Duration
}

// New creates a new direct message service (API only, no repository)
func New(ig InstagramClient) *Service {
	return &Service{
		ig:         ig,
		syncMaxAge: 5 * time.Minute,
	}
}

// NewWithRepo creates service with repository support
func NewWithRepo(
	ig InstagramClient,
	convRepo ConversationRepository,
	msgRepo MessageRepository,
	convSyncRepo ConversationSyncRepository,
	accountSyncRepo AccountSyncRepository,
) *Service {
	return &Service{
		ig:              ig,
		convRepo:        convRepo,
		msgRepo:         msgRepo,
		convSyncRepo:    convSyncRepo,
		accountSyncRepo: accountSyncRepo,
		syncMaxAge:      5 * time.Minute,
	}
}

// GetConversationsInput represents input for getting conversations
type GetConversationsInput struct {
	AccountID   string
	UserID      string
	AccessToken string
	Limit       int
	Offset      int
}

// GetConversationsOutput represents output from getting conversations
type GetConversationsOutput struct {
	Conversations []entity.Conversation
	Total         int64
	HasMore       bool
}

// GetConversations retrieves conversations for an account
func (s *Service) GetConversations(ctx context.Context, in GetConversationsInput) (*GetConversationsOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	// If we have a repository, get from local cache
	if s.convRepo != nil {
		conversations, err := s.convRepo.GetByAccountID(ctx, in.AccountID, limit, in.Offset)
		if err != nil {
			return nil, fmt.Errorf("getting conversations from cache: %w", err)
		}

		total, _ := s.convRepo.Count(ctx, in.AccountID)

		return &GetConversationsOutput{
			Conversations: conversations,
			Total:         total,
			HasMore:       int64(in.Offset+len(conversations)) < total,
		}, nil
	}

	// Fallback to direct API call
	result, err := s.ig.GetConversations(ctx, in.UserID, in.AccessToken, limit, "")
	if err != nil {
		return nil, fmt.Errorf("getting conversations from API: %w", err)
	}

	return &GetConversationsOutput{
		Conversations: result.Conversations,
		Total:         int64(len(result.Conversations)),
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
func (s *Service) SearchConversations(ctx context.Context, in SearchConversationsInput) (*GetConversationsOutput, error) {
	if s.convRepo == nil {
		return nil, fmt.Errorf("search requires repository")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	conversations, err := s.convRepo.Search(ctx, in.AccountID, in.Query, limit, in.Offset)
	if err != nil {
		return nil, fmt.Errorf("searching conversations: %w", err)
	}

	return &GetConversationsOutput{
		Conversations: conversations,
		Total:         int64(len(conversations)),
		HasMore:       len(conversations) == limit,
	}, nil
}

// GetMessagesInput represents input for getting messages
type GetMessagesInput struct {
	AccountID      string
	ConversationID string
	UserID         string
	AccessToken    string
	Limit          int
	Offset         int
}

// GetMessagesOutput represents output from getting messages
type GetMessagesOutput struct {
	Messages []entity.Message
	Total    int64
	HasMore  bool
}

// GetMessages retrieves messages for a conversation (triggers on-demand sync)
func (s *Service) GetMessages(ctx context.Context, in GetMessagesInput) (*GetMessagesOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}

	// If we have repositories, check if we need to sync
	if s.msgRepo != nil && s.convSyncRepo != nil {
		// Check sync status
		syncStatus, err := s.convSyncRepo.GetSyncStatus(ctx, in.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("getting sync status: %w", err)
		}

		// Sync if never synced or stale
		needsSync := syncStatus == nil || time.Since(syncStatus.LastSyncedAt) > s.syncMaxAge
		if needsSync {
			if err := s.syncMessagesFromInstagram(ctx, in.ConversationID, in.UserID, in.AccessToken); err != nil {
				// Log error but continue with cached data if available
				fmt.Printf("sync error (continuing with cache): %v\n", err)
			}
		}

		// Get messages from cache
		messages, err := s.msgRepo.GetByConversationID(ctx, in.ConversationID, limit, in.Offset)
		if err != nil {
			return nil, fmt.Errorf("getting messages from cache: %w", err)
		}

		total, _ := s.msgRepo.Count(ctx, in.ConversationID)

		return &GetMessagesOutput{
			Messages: messages,
			Total:    total,
			HasMore:  int64(in.Offset+len(messages)) < total,
		}, nil
	}

	// Fallback to direct API call
	result, err := s.ig.GetMessages(ctx, in.ConversationID, in.UserID, in.AccessToken, limit, "")
	if err != nil {
		return nil, fmt.Errorf("getting messages from API: %w", err)
	}

	return &GetMessagesOutput{
		Messages: result.Messages,
		Total:    int64(len(result.Messages)),
		HasMore:  result.HasMore,
	}, nil
}

// syncMessagesFromInstagram syncs messages from Instagram API to local database
// Saves each page incrementally and asynchronously
func (s *Service) syncMessagesFromInstagram(ctx context.Context, conversationID, userID, accessToken string) error {
	cursor := ""
	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	var oldestTimestamp *time.Time
	var mu sync.Mutex

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}

		// Check for async errors
		select {
		case err := <-errCh:
			wg.Wait()
			return fmt.Errorf("async save failed: %w", err)
		default:
		}

		result, err := s.ig.GetMessages(ctx, conversationID, userID, accessToken, 100, cursor)
		if err != nil {
			wg.Wait()
			return fmt.Errorf("fetching messages: %w", err)
		}

		// Save page asynchronously
		if len(result.Messages) > 0 {
			messages := make([]entity.Message, len(result.Messages))
			copy(messages, result.Messages)

			// Track oldest message timestamp
			mu.Lock()
			lastMsg := messages[len(messages)-1]
			if oldestTimestamp == nil || lastMsg.Timestamp.Before(*oldestTimestamp) {
				oldestTimestamp = &lastMsg.Timestamp
			}
			mu.Unlock()

			wg.Add(1)
			go func(msgs []entity.Message) {
				defer wg.Done()
				if err := s.msgRepo.UpsertBatch(ctx, msgs); err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}(messages)
		}

		if !result.HasMore || result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	// Wait for all saves
	wg.Wait()

	// Check for errors
	select {
	case err := <-errCh:
		return fmt.Errorf("async save failed: %w", err)
	default:
	}

	// Update sync status
	if err := s.convSyncRepo.UpdateSyncStatus(ctx, &ConversationSyncStatus{
		ConversationID:         conversationID,
		LastSyncedAt:           time.Now(),
		NextCursor:             "",
		SyncComplete:           true,
		OldestMessageTimestamp: oldestTimestamp,
	}); err != nil {
		return fmt.Errorf("updating sync status: %w", err)
	}

	return nil
}

// SendMessageInput represents input for sending a message
type SendMessageInput struct {
	AccountID      string
	ConversationID string
	UserID         string
	RecipientID    string
	AccessToken    string
	Message        string
}

// SendMessageOutput represents output from sending a message
type SendMessageOutput struct {
	MessageID string
}

// SendMessage sends a text message
func (s *Service) SendMessage(ctx context.Context, in SendMessageInput) (*SendMessageOutput, error) {
	if err := entity.ValidateMessageText(in.Message); err != nil {
		return nil, err
	}

	result, err := s.ig.SendMessage(ctx, in.UserID, in.RecipientID, in.AccessToken, in.Message)
	if err != nil {
		return nil, fmt.Errorf("sending message: %w", err)
	}

	// Best-effort: save to local database
	if s.msgRepo != nil {
		msg := &entity.Message{
			ID:             result.MessageID,
			ConversationID: in.ConversationID,
			SenderID:       in.UserID,
			Type:           entity.MessageTypeText,
			Text:           in.Message,
			IsFromMe:       true,
			Timestamp:      time.Now(),
		}
		_ = s.msgRepo.Upsert(ctx, msg)
	}

	return &SendMessageOutput{MessageID: result.MessageID}, nil
}

// SendMediaMessageInput represents input for sending a media message
type SendMediaMessageInput struct {
	AccountID      string
	ConversationID string
	UserID         string
	RecipientID    string
	AccessToken    string
	MediaURL       string
	MediaType      string // "image" or "video"
}

// SendMediaMessage sends a media message
func (s *Service) SendMediaMessage(ctx context.Context, in SendMediaMessageInput) (*SendMessageOutput, error) {
	if in.MediaURL == "" {
		return nil, entity.ErrMediaRequired
	}

	result, err := s.ig.SendMediaMessage(ctx, in.UserID, in.RecipientID, in.AccessToken, in.MediaURL, in.MediaType)
	if err != nil {
		return nil, fmt.Errorf("sending media message: %w", err)
	}

	// Best-effort: save to local database
	if s.msgRepo != nil {
		msgType := entity.MessageTypeImage
		if in.MediaType == "video" {
			msgType = entity.MessageTypeVideo
		}
		msg := &entity.Message{
			ID:             result.MessageID,
			ConversationID: in.ConversationID,
			SenderID:       in.UserID,
			Type:           msgType,
			MediaURL:       in.MediaURL,
			MediaType:      in.MediaType,
			IsFromMe:       true,
			Timestamp:      time.Now(),
		}
		_ = s.msgRepo.Upsert(ctx, msg)
	}

	return &SendMessageOutput{MessageID: result.MessageID}, nil
}

// SyncConversations syncs conversations list from Instagram (for scheduler)
// Saves each page incrementally and asynchronously to avoid memory buildup
func (s *Service) SyncConversations(ctx context.Context, accountID, userID, accessToken string) error {
	if s.convRepo == nil {
		return fmt.Errorf("repository required for sync")
	}

	cursor := ""
	var wg sync.WaitGroup
	errCh := make(chan error, 1) // Buffer for first error
	emptyPages := 0              // Counter for consecutive empty pages
	const maxEmptyPages = 3      // Stop after this many consecutive empty pages

	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		default:
		}

		// Check if async save failed
		select {
		case err := <-errCh:
			wg.Wait()
			return fmt.Errorf("async save failed: %w", err)
		default:
		}

		result, err := s.ig.GetConversations(ctx, userID, accessToken, 100, cursor)
		if err != nil {
			wg.Wait()
			return fmt.Errorf("fetching conversations: %w", err)
		}

		// log.Printf("[DEBUG] SyncConversations: got %d conversations, hasMore=%v, cursor=%s", len(result.Conversations), result.HasMore, cursor)

		// Track consecutive empty pages to prevent infinite loops
		if len(result.Conversations) == 0 {
			emptyPages++
			if emptyPages >= maxEmptyPages {
				log.Printf("[WARN] SyncConversations: stopping after %d consecutive empty pages (possible API permission issue)", emptyPages)
				break
			}
		} else {
			emptyPages = 0 // Reset counter on non-empty page
		}

		// Save page asynchronously
		if len(result.Conversations) > 0 {
			// Set account ID for all conversations
			conversations := make([]entity.Conversation, len(result.Conversations))
			copy(conversations, result.Conversations)
			for i := range conversations {
				conversations[i].AccountID = accountID
			}

			wg.Add(1)
			go func(convs []entity.Conversation) {
				defer wg.Done()
				if err := s.convRepo.UpsertBatch(ctx, convs); err != nil {
					log.Printf("[ERROR] UpsertBatch failed: %v", err)
					// Send error only if channel is empty
					select {
					case errCh <- err:
					default:
					}
				} // else {
				// 	log.Printf("[DEBUG] UpsertBatch: saved %d conversations", len(convs))
				// }
			}(conversations)
		}

		if !result.HasMore || result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	// Wait for all async saves to complete
	wg.Wait()

	// Check for any errors from async saves
	select {
	case err := <-errCh:
		return fmt.Errorf("async save failed: %w", err)
	default:
	}

	// Update account sync status
	if s.accountSyncRepo != nil {
		if err := s.accountSyncRepo.UpdateSyncStatus(ctx, &AccountSyncStatus{
			AccountID:    accountID,
			LastSyncedAt: time.Now(),
			SyncComplete: true,
		}); err != nil {
			return fmt.Errorf("updating account sync status: %w", err)
		}
	}

	return nil
}

// GetAccountsNeedingSync returns accounts that need conversation sync (for scheduler)
func (s *Service) GetAccountsNeedingSync(ctx context.Context, olderThan time.Duration, limit int) ([]string, error) {
	if s.accountSyncRepo == nil {
		return nil, fmt.Errorf("repository required")
	}
	return s.accountSyncRepo.GetAccountsNeedingSync(ctx, olderThan, limit)
}

// SyncMessages manually syncs messages for a specific conversation
func (s *Service) SyncMessages(ctx context.Context, conversationID, userID, accessToken string) error {
	if s.msgRepo == nil {
		return fmt.Errorf("repository required for sync")
	}

	return s.syncMessagesFromInstagram(ctx, conversationID, userID, accessToken)
}

// GetStatisticsInput represents input for getting statistics
type GetStatisticsInput struct {
	AccountID string
	StartDate time.Time
	EndDate   time.Time
}

// GetStatistics returns DM statistics for an account
func (s *Service) GetStatistics(ctx context.Context, in GetStatisticsInput) (*entity.Statistics, error) {
	if s.msgRepo == nil {
		return nil, fmt.Errorf("repository required for statistics")
	}

	return s.msgRepo.GetStatistics(ctx, entity.StatisticsFilter{
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
func (s *Service) GetHeatmap(ctx context.Context, in GetHeatmapInput) (*entity.Heatmap, error) {
	if s.msgRepo == nil {
		return nil, fmt.Errorf("repository required for heatmap")
	}

	return s.msgRepo.GetHeatmap(ctx, entity.StatisticsFilter{
		AccountID: in.AccountID,
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
	})
}

// IncrementAccountSyncRetryCount increments the retry count for account sync
func (s *Service) IncrementAccountSyncRetryCount(ctx context.Context, accountID string, lastError string, maxRetries int) error {
	if s.accountSyncRepo == nil {
		return nil
	}
	return s.accountSyncRepo.IncrementRetryCount(ctx, accountID, lastError, maxRetries)
}

// ResetAccountSyncRetryCount resets the retry count after a successful account sync
func (s *Service) ResetAccountSyncRetryCount(ctx context.Context, accountID string) error {
	if s.accountSyncRepo == nil {
		return nil
	}
	return s.accountSyncRepo.ResetRetryCount(ctx, accountID)
}

// IncrementConversationSyncRetryCount increments the retry count for conversation sync
func (s *Service) IncrementConversationSyncRetryCount(ctx context.Context, conversationID string, lastError string, maxRetries int) error {
	if s.convSyncRepo == nil {
		return nil
	}
	return s.convSyncRepo.IncrementRetryCount(ctx, conversationID, lastError, maxRetries)
}

// ResetConversationSyncRetryCount resets the retry count after a successful conversation sync
func (s *Service) ResetConversationSyncRetryCount(ctx context.Context, conversationID string) error {
	if s.convSyncRepo == nil {
		return nil
	}
	return s.convSyncRepo.ResetRetryCount(ctx, conversationID)
}
