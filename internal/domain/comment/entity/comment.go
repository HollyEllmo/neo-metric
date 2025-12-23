package entity

import (
	"errors"
	"time"
)

// Comment represents an Instagram comment
type Comment struct {
	ID              string    `json:"id"`
	MediaID         string    `json:"media_id"`
	AuthorID        string    `json:"author_id,omitempty"`         // Instagram user ID of comment author
	Username        string    `json:"username"`
	Text            string    `json:"text"`
	Timestamp       time.Time `json:"timestamp"`
	LikeCount       int       `json:"like_count"`
	IsHidden        bool      `json:"hidden"`
	ParentID        string    `json:"parent_id,omitempty"`         // For replies
	RepliesCount    int       `json:"replies_count,omitempty"`
	ReplyToUsername string    `json:"reply_to_username,omitempty"` // Who this is replying to
}

// Author represents the author of a comment
type Author struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// Domain errors
var (
	ErrCommentNotFound    = errors.New("comment not found")
	ErrMediaNotFound      = errors.New("media not found")
	ErrEmptyReplyText     = errors.New("reply text cannot be empty")
	ErrReplyTextTooLong   = errors.New("reply text exceeds maximum length")
	ErrUnauthorized       = errors.New("unauthorized to perform this action")
	ErrCommentingDisabled = errors.New("commenting is disabled for this media")
)

// MaxReplyLength is the maximum length of a comment reply
const MaxReplyLength = 2200

// ValidateReplyText validates the text for a reply
func ValidateReplyText(text string) error {
	if text == "" {
		return ErrEmptyReplyText
	}
	if len(text) > MaxReplyLength {
		return ErrReplyTextTooLong
	}
	return nil
}
