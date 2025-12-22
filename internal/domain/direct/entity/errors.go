package entity

import "errors"

// Domain errors for Direct Messages
var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMessageNotFound      = errors.New("message not found")
	ErrEmptyMessage         = errors.New("message text cannot be empty")
	ErrMessageTooLong       = errors.New("message exceeds maximum length")
	ErrUnauthorized         = errors.New("unauthorized to perform this action")
	ErrMessagingDisabled    = errors.New("messaging is disabled for this user")
	ErrUserNotFound         = errors.New("user not found")
	ErrInvalidRecipient     = errors.New("invalid recipient")
	ErrMediaRequired        = errors.New("media is required for this message type")
	ErrInvalidMediaType     = errors.New("invalid media type")
	ErrRateLimited          = errors.New("rate limit exceeded")
)
