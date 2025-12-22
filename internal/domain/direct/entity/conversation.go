package entity

import "time"

// Conversation represents an Instagram DM conversation/thread
type Conversation struct {
	ID                        string     `json:"id"`
	AccountID                 string     `json:"account_id"`
	ParticipantID             string     `json:"participant_id"`
	ParticipantUsername       string     `json:"participant_username"`
	ParticipantName           string     `json:"participant_name,omitempty"`
	ParticipantAvatarURL      string     `json:"participant_avatar_url,omitempty"`
	ParticipantFollowersCount int        `json:"participant_followers_count,omitempty"`
	LastMessageText           string     `json:"last_message_text,omitempty"`
	LastMessageAt             *time.Time `json:"last_message_at,omitempty"`
	LastMessageIsFromMe       bool       `json:"last_message_is_from_me,omitempty"`
	UnreadCount               int        `json:"unread_count"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

// Participant represents the other user in a DM conversation
type Participant struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	FollowersCount int    `json:"followers_count,omitempty"`
}
