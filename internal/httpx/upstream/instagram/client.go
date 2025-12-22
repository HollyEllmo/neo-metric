package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// Check for error response
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
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
