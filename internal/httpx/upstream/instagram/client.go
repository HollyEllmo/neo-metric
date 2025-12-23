package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL    = "https://graph.instagram.com"
	defaultAPIVersion = "v21.0"
	defaultTimeout    = 30 * time.Second
)

// Client is an Instagram Graph API client for content publishing
type Client struct {
	baseURL    string
	apiVersion string
	httpClient *http.Client
	logger     *slog.Logger
}

// ClientOption is a function that configures the Client
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithAPIVersion sets the API version
func WithAPIVersion(version string) ClientOption {
	return func(c *Client) {
		c.apiVersion = version
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithLogger sets a logger for debug output
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// New creates a new Instagram API client
func New(opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		apiVersion: defaultAPIVersion,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// APIError represents an error from the Instagram API
type APIError struct {
	Message      string `json:"message"`
	Type         string `json:"type"`
	Code         int    `json:"code"`
	ErrorSubcode int    `json:"error_subcode"`
	FBTraceID    string `json:"fbtrace_id"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("instagram API error: %s (code: %d, subcode: %d)", e.Message, e.Code, e.ErrorSubcode)
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// MediaType represents the type of media being published
type MediaType string

const (
	MediaTypeImage    MediaType = "IMAGE"
	MediaTypeVideo    MediaType = "VIDEO"
	MediaTypeCarousel MediaType = "CAROUSEL"
	MediaTypeReels    MediaType = "REELS"
	MediaTypeStories  MediaType = "STORIES"
)

// ContainerStatus represents the status of a media container
type ContainerStatus string

const (
	ContainerStatusExpired    ContainerStatus = "EXPIRED"
	ContainerStatusError      ContainerStatus = "ERROR"
	ContainerStatusFinished   ContainerStatus = "FINISHED"
	ContainerStatusInProgress ContainerStatus = "IN_PROGRESS"
	ContainerStatusPublished  ContainerStatus = "PUBLISHED"
)

// CreateMediaContainerInput represents input for creating a media container
type CreateMediaContainerInput struct {
	UserID      string
	AccessToken string
	ImageURL    string    // For single image
	VideoURL    string    // For video/reel
	MediaType   MediaType // IMAGE, VIDEO, REELS, STORIES
	Caption     string
	IsCarousel  bool     // True for carousel items
	Children    []string // Container IDs for carousel
}

// CreateMediaContainerOutput represents output from creating a media container
type CreateMediaContainerOutput struct {
	ID string `json:"id"`
}

// CreateMediaContainer creates a media container for publishing
// Step 1 of the publishing process
func (c *Client) CreateMediaContainer(ctx context.Context, in CreateMediaContainerInput) (*CreateMediaContainerOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/media", c.baseURL, c.apiVersion, in.UserID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)

	// Set media URL based on type
	if in.ImageURL != "" {
		params.Set("image_url", in.ImageURL)
	}
	if in.VideoURL != "" {
		params.Set("video_url", in.VideoURL)
	}

	// Set media type for special content
	switch in.MediaType {
	case MediaTypeReels:
		params.Set("media_type", "REELS")
	case MediaTypeStories:
		params.Set("media_type", "STORIES")
	case MediaTypeCarousel:
		params.Set("media_type", "CAROUSEL")
		// Add children for carousel
		for _, childID := range in.Children {
			params.Add("children", childID)
		}
	}

	// Set carousel item flag
	if in.IsCarousel {
		params.Set("is_carousel_item", "true")
	}

	// Caption (not for carousel items)
	if in.Caption != "" && !in.IsCarousel {
		params.Set("caption", in.Caption)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out CreateMediaContainerOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetContainerStatusInput represents input for checking container status
type GetContainerStatusInput struct {
	ContainerID string
	AccessToken string
}

// GetContainerStatusOutput represents output from checking container status
type GetContainerStatusOutput struct {
	ID           string          `json:"id"`
	Status       ContainerStatus `json:"status_code"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

// GetContainerStatus checks the status of a media container
// Step 2 of the publishing process (for video content)
func (c *Client) GetContainerStatus(ctx context.Context, in GetContainerStatusInput) (*GetContainerStatusOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, c.apiVersion, in.ContainerID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("fields", "status_code,error_message")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out GetContainerStatusOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// PublishMediaInput represents input for publishing media
type PublishMediaInput struct {
	UserID      string
	AccessToken string
	ContainerID string
}

// PublishMediaOutput represents output from publishing media
type PublishMediaOutput struct {
	ID string `json:"id"` // Instagram Media ID
}

// PublishMedia publishes a media container
// Step 3 of the publishing process
func (c *Client) PublishMedia(ctx context.Context, in PublishMediaInput) (*PublishMediaOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/media_publish", c.baseURL, c.apiVersion, in.UserID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("creation_id", in.ContainerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out PublishMediaOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// DeleteMediaInput represents input for deleting media
type DeleteMediaInput struct {
	MediaID     string
	AccessToken string
}

// DeleteMedia deletes published media from Instagram
// Note: This only works for media published via the API
func (c *Client) DeleteMedia(ctx context.Context, in DeleteMediaInput) error {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, c.apiVersion, in.MediaID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	var result map[string]interface{}
	return c.do(req, &result)
}

// GetMediaInput represents input for getting media details
type GetMediaInput struct {
	MediaID     string
	AccessToken string
	Fields      []string
}

// GetMediaOutput represents media details from Instagram
type GetMediaOutput struct {
	ID           string `json:"id"`
	Caption      string `json:"caption,omitempty"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url,omitempty"`
	Permalink    string `json:"permalink,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
	Username     string `json:"username,omitempty"`
}

// GetMedia retrieves details of a published media
func (c *Client) GetMedia(ctx context.Context, in GetMediaInput) (*GetMediaOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, c.apiVersion, in.MediaID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)

	fields := in.Fields
	if len(fields) == 0 {
		fields = []string{"id", "caption", "media_type", "media_url", "permalink", "thumbnail_url", "timestamp", "username"}
	}
	params.Set("fields", joinStrings(fields, ","))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out GetMediaOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// do executes an HTTP request and decodes the response
func (c *Client) do(req *http.Request, out interface{}) error {
	// Log request details at DEBUG level
	if c.logger != nil {
		c.logger.Debug("instagram API request",
			"method", req.Method,
			"url", sanitizeURL(req.URL.String()),
		)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		if c.logger != nil {
			c.logger.Debug("instagram API request failed",
				"method", req.Method,
				"url", sanitizeURL(req.URL.String()),
				"duration_ms", duration.Milliseconds(),
				"error", err.Error(),
			)
		}
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// Log response at DEBUG level
	if c.logger != nil {
		c.logger.Debug("instagram API response",
			"method", req.Method,
			"url", sanitizeURL(req.URL.String()),
			"status", resp.StatusCode,
			"duration_ms", duration.Milliseconds(),
			"body_size", len(body),
			"body", string(body),
		)
	}

	// Check for error response
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			if c.logger != nil {
				c.logger.Error("instagram API error response",
					"status", resp.StatusCode,
					"body", string(body),
				)
			}
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		if c.logger != nil {
			c.logger.Error("instagram API error",
				"code", errResp.Error.Code,
				"subcode", errResp.Error.ErrorSubcode,
				"message", errResp.Error.Message,
				"type", errResp.Error.Type,
				"trace_id", errResp.Error.FBTraceID,
			)
		}
		return &errResp.Error
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// sanitizeURL removes access_token from URL for logging
func sanitizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	if q.Has("access_token") {
		q.Set("access_token", "[REDACTED]")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}

// ============================================================================
// Comments API
// ============================================================================

// CommentData represents a comment from Instagram API
type CommentData struct {
	ID           string    `json:"id"`
	Text         string    `json:"text"`
	Username     string    `json:"username"`
	Timestamp    time.Time `json:"timestamp"`
	LikeCount    int       `json:"like_count"`
	Hidden       bool      `json:"hidden"`
	RepliesCount int       `json:"replies_count,omitempty"`
}

// GetCommentsInput represents input for getting comments
type GetCommentsInput struct {
	MediaID     string
	AccessToken string
	Limit       int
	After       string // Cursor for pagination
}

// GetCommentsOutput represents output from getting comments
type GetCommentsOutput struct {
	Data   []CommentData `json:"data"`
	Paging *Paging       `json:"paging,omitempty"`
}

// Paging represents pagination info from Instagram API
type Paging struct {
	Cursors struct {
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"cursors"`
	Next string `json:"next,omitempty"`
}

// GetComments retrieves comments for a media
// GET /{media-id}/comments
func (c *Client) GetComments(ctx context.Context, in GetCommentsInput) (*GetCommentsOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/comments", c.baseURL, c.apiVersion, in.MediaID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("fields", "id,text,username,timestamp,like_count,hidden")

	if in.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.After != "" {
		params.Set("after", in.After)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out GetCommentsOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetCommentRepliesInput represents input for getting comment replies
type GetCommentRepliesInput struct {
	CommentID   string
	AccessToken string
	Limit       int
	After       string
}

// GetCommentReplies retrieves replies to a comment
// GET /{comment-id}/replies
func (c *Client) GetCommentReplies(ctx context.Context, in GetCommentRepliesInput) (*GetCommentsOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/replies", c.baseURL, c.apiVersion, in.CommentID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("fields", "id,text,username,timestamp,like_count,hidden")

	if in.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.After != "" {
		params.Set("after", in.After)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out GetCommentsOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ReplyToCommentInput represents input for replying to a comment
type ReplyToCommentInput struct {
	CommentID   string
	AccessToken string
	Message     string
}

// ReplyToCommentOutput represents output from replying to a comment
type ReplyToCommentOutput struct {
	ID string `json:"id"`
}

// ReplyToComment posts a reply to a comment
// POST /{comment-id}/replies
func (c *Client) ReplyToComment(ctx context.Context, in ReplyToCommentInput) (*ReplyToCommentOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/replies", c.baseURL, c.apiVersion, in.CommentID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("message", in.Message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out ReplyToCommentOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// DeleteCommentInput represents input for deleting a comment
type DeleteCommentInput struct {
	CommentID   string
	AccessToken string
}

// DeleteComment deletes a comment
// DELETE /{comment-id}
func (c *Client) DeleteComment(ctx context.Context, in DeleteCommentInput) error {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, c.apiVersion, in.CommentID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	var result map[string]interface{}
	return c.do(req, &result)
}

// HideCommentInput represents input for hiding/unhiding a comment
type HideCommentInput struct {
	CommentID   string
	AccessToken string
	Hide        bool
}

// HideComment hides or unhides a comment
// POST /{comment-id}?hide=true/false
func (c *Client) HideComment(ctx context.Context, in HideCommentInput) error {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, c.apiVersion, in.CommentID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("hide", fmt.Sprintf("%t", in.Hide))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	var result map[string]interface{}
	return c.do(req, &result)
}

// CreateCommentInput represents input for creating a comment on media
type CreateCommentInput struct {
	MediaID     string
	AccessToken string
	Message     string
}

// CreateCommentOutput represents output from creating a comment
type CreateCommentOutput struct {
	ID string `json:"id"`
}

// CreateComment creates a new comment on a media
// POST /{media-id}/comments
func (c *Client) CreateComment(ctx context.Context, in CreateCommentInput) (*CreateCommentOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/comments", c.baseURL, c.apiVersion, in.MediaID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("message", in.Message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out CreateCommentOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ============================================================================
// Direct Messages API (Instagram Messenger Platform)
// ============================================================================

// DMConversationData represents a conversation from Instagram DM API
type DMConversationData struct {
	ID           string          `json:"id"`
	Participants *DMParticipants `json:"participants,omitempty"`
	Messages     *DMMessagesData `json:"messages,omitempty"`
	UpdatedTime  string          `json:"updated_time,omitempty"`
}

// DMParticipants holds conversation participants
type DMParticipants struct {
	Data []DMParticipantData `json:"data"`
}

// DMParticipantData represents a participant in a conversation
type DMParticipantData struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
}

// DMMessagesData holds messages in a conversation
type DMMessagesData struct {
	Data []DMMessageData `json:"data"`
}

// DMMessageData represents a message from Instagram DM API
type DMMessageData struct {
	ID          string             `json:"id"`
	Message     string             `json:"message,omitempty"`
	From        *DMParticipantData `json:"from,omitempty"`
	CreatedTime string             `json:"created_time,omitempty"`
	Attachments *DMAttachments     `json:"attachments,omitempty"`
}

// DMAttachments holds message attachments
type DMAttachments struct {
	Data []DMAttachment `json:"data"`
}

// DMAttachment represents a message attachment
type DMAttachment struct {
	ID        string             `json:"id"`
	Type      string             `json:"type,omitempty"`      // image, video, audio, share, story_mention, etc.
	MimeType  string             `json:"mime_type,omitempty"`
	Name      string             `json:"name,omitempty"`
	Size      int64              `json:"size,omitempty"`
	ImageData *DMAttachmentImage `json:"image_data,omitempty"`
	VideoData *DMAttachmentVideo `json:"video_data,omitempty"`
	// For shared content (reels, posts, etc.)
	ShareURL string `json:"share_url,omitempty"`
	// Generic payload for unknown attachment types
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// DMAttachmentImage represents image attachment data
type DMAttachmentImage struct {
	URL       string `json:"url"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	MaxWidth  int    `json:"max_width,omitempty"`
	MaxHeight int    `json:"max_height,omitempty"`
}

// DMAttachmentVideo represents video attachment data
type DMAttachmentVideo struct {
	URL        string `json:"url"`
	PreviewURL string `json:"preview_url,omitempty"`
	Length     int    `json:"length,omitempty"`
}

// GetDMConversationsInput represents input for getting DM conversations
type GetDMConversationsInput struct {
	UserID      string
	AccessToken string
	Limit       int
	After       string
}

// GetDMConversationsOutput represents output from getting conversations
type GetDMConversationsOutput struct {
	Data   []DMConversationData `json:"data"`
	Paging *Paging              `json:"paging,omitempty"`
}

// GetDMConversations retrieves DM conversations for a user
// GET /{user-id}/conversations
func (c *Client) GetDMConversations(ctx context.Context, in GetDMConversationsInput) (*GetDMConversationsOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/conversations", c.baseURL, c.apiVersion, in.UserID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("platform", "instagram")
	params.Set("fields", "id,participants,messages{id,message,from,created_time},updated_time")

	if in.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.After != "" {
		params.Set("after", in.After)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out GetDMConversationsOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetDMMessagesInput represents input for getting messages in a conversation
type GetDMMessagesInput struct {
	ConversationID string
	AccessToken    string
	Limit          int
	After          string
}

// GetDMMessagesOutput represents output from getting messages
type GetDMMessagesOutput struct {
	Data   []DMMessageData `json:"data"`
	Paging *Paging         `json:"paging,omitempty"`
}

// GetDMMessages retrieves messages in a conversation
// GET /{conversation-id}/messages
func (c *Client) GetDMMessages(ctx context.Context, in GetDMMessagesInput) (*GetDMMessagesOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/messages", c.baseURL, c.apiVersion, in.ConversationID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("fields", "id,message,from,created_time,attachments{id,mime_type,name,size,image_data,video_data}")

	if in.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	if in.After != "" {
		params.Set("after", in.After)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out GetDMMessagesOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// SendDMMessageInput represents input for sending a DM message
type SendDMMessageInput struct {
	UserID      string // Instagram user ID of the sender (page-scoped)
	RecipientID string // Instagram user ID of the recipient
	AccessToken string
	Message     string
}

// SendDMMessageOutput represents output from sending a message
type SendDMMessageOutput struct {
	RecipientID string `json:"recipient_id"`
	MessageID   string `json:"message_id"`
}

// SendDMMessage sends a text message via Instagram DM
// POST /{user-id}/messages
func (c *Client) SendDMMessage(ctx context.Context, in SendDMMessageInput) (*SendDMMessageOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/messages", c.baseURL, c.apiVersion, in.UserID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("recipient", fmt.Sprintf(`{"id":"%s"}`, in.RecipientID))
	params.Set("message", fmt.Sprintf(`{"text":"%s"}`, in.Message))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out SendDMMessageOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// SendDMMediaMessageInput represents input for sending a media message
type SendDMMediaMessageInput struct {
	UserID      string
	RecipientID string
	AccessToken string
	MediaURL    string
	MediaType   string // "image" or "video"
}

// SendDMMediaMessage sends a media message via Instagram DM
// POST /{user-id}/messages
func (c *Client) SendDMMediaMessage(ctx context.Context, in SendDMMediaMessageInput) (*SendDMMessageOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s/messages", c.baseURL, c.apiVersion, in.UserID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("recipient", fmt.Sprintf(`{"id":"%s"}`, in.RecipientID))

	// Build attachment based on media type
	attachmentType := "image"
	if in.MediaType == "video" {
		attachmentType = "video"
	}
	params.Set("message", fmt.Sprintf(`{"attachment":{"type":"%s","payload":{"url":"%s"}}}`, attachmentType, in.MediaURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out SendDMMessageOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetDMParticipantInput represents input for getting participant info
type GetDMParticipantInput struct {
	UserID      string
	AccessToken string
}

// GetDMParticipantOutput represents participant profile info
type GetDMParticipantOutput struct {
	ID             string `json:"id"`
	Username       string `json:"username,omitempty"`
	Name           string `json:"name,omitempty"`
	ProfilePicURL  string `json:"profile_pic,omitempty"`
	FollowersCount int    `json:"followers_count,omitempty"`
}

// GetDMParticipant retrieves profile info for a DM participant
// GET /{user-id}
func (c *Client) GetDMParticipant(ctx context.Context, in GetDMParticipantInput) (*GetDMParticipantOutput, error) {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, c.apiVersion, in.UserID)

	params := url.Values{}
	params.Set("access_token", in.AccessToken)
	params.Set("fields", "id,username,name,profile_pic,followers_count")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var out GetDMParticipantOutput
	if err := c.do(req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}
