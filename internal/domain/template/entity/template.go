package entity

import (
	"errors"
	"time"
)

// TemplateType represents the type of template
type TemplateType string

const (
	TemplateTypeDirect  TemplateType = "direct"
	TemplateTypeComment TemplateType = "comment"
	TemplateTypeBoth    TemplateType = "both"
)

// Template represents a reusable message template
type Template struct {
	ID         string       `json:"id"`
	AccountID  string       `json:"account_id"`
	Title      string       `json:"title"`
	Content    string       `json:"content"`
	Images     []string     `json:"images,omitempty"`
	Icon       string       `json:"icon,omitempty"`
	Type       TemplateType `json:"type"`
	UsageCount int          `json:"usage_count"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// Domain errors for Templates
var (
	ErrTemplateNotFound    = errors.New("template not found")
	ErrEmptyTitle          = errors.New("template title cannot be empty")
	ErrEmptyContent        = errors.New("template content cannot be empty")
	ErrInvalidTemplateType = errors.New("invalid template type")
	ErrTitleTooLong        = errors.New("template title exceeds maximum length")
	ErrContentTooLong      = errors.New("template content exceeds maximum length")
	ErrTooManyImages       = errors.New("too many images in template")
)

// MaxTitleLength is the maximum length of a template title
const MaxTitleLength = 255

// MaxContentLength is the maximum length of a template content
const MaxContentLength = 2200

// Validate validates template fields
func (t *Template) Validate() error {
	if t.Title == "" {
		return ErrEmptyTitle
	}
	if len(t.Title) > MaxTitleLength {
		return ErrTitleTooLong
	}
	if t.Content == "" {
		return ErrEmptyContent
	}
	if len(t.Content) > MaxContentLength {
		return ErrContentTooLong
	}
	if !IsValidTemplateType(t.Type) {
		return ErrInvalidTemplateType
	}
	return nil
}

// IsValidTemplateType checks if a template type is valid
func IsValidTemplateType(t TemplateType) bool {
	switch t {
	case TemplateTypeDirect, TemplateTypeComment, TemplateTypeBoth:
		return true
	}
	return false
}

// ParseTemplateType parses a string into a TemplateType
func ParseTemplateType(s string) (TemplateType, error) {
	switch s {
	case "direct":
		return TemplateTypeDirect, nil
	case "comment":
		return TemplateTypeComment, nil
	case "both":
		return TemplateTypeBoth, nil
	default:
		return "", ErrInvalidTemplateType
	}
}
