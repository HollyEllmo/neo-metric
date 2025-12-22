package entity

import "time"

// MessageType represents the type of DM message
type MessageType string

const (
	MessageTypeText         MessageType = "text"
	MessageTypeImage        MessageType = "image"
	MessageTypeVideo        MessageType = "video"
	MessageTypeAudio        MessageType = "audio"
	MessageTypeLink         MessageType = "link"
	MessageTypeStoryMention MessageType = "story_mention"
	MessageTypeStoryReply   MessageType = "story_reply"
)

// Message represents a direct message
type Message struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversation_id"`
	SenderID       string      `json:"sender_id"`
	Type           MessageType `json:"type"`
	Text           string      `json:"text,omitempty"`
	MediaURL       string      `json:"media_url,omitempty"`
	MediaType      string      `json:"media_type,omitempty"` // image/video/audio for media messages
	IsUnsent       bool        `json:"is_unsent"`
	IsFromMe       bool        `json:"is_from_me"`
	Timestamp      time.Time   `json:"timestamp"`
	CreatedAt      time.Time   `json:"created_at"`
}

// MaxMessageLength is the maximum length of a DM text message
const MaxMessageLength = 1000

// ValidateMessageText validates the text for a message
func ValidateMessageText(text string) error {
	if text == "" {
		return ErrEmptyMessage
	}
	if len(text) > MaxMessageLength {
		return ErrMessageTooLong
	}
	return nil
}
