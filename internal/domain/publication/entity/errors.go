package entity

import "errors"

// Domain errors for publication
var (
	// Validation errors
	ErrEmptyAccountID      = errors.New("account ID is required")
	ErrNoMedia             = errors.New("at least one media item is required")
	ErrTooManyMediaItems   = errors.New("post cannot have more than 10 media items")
	ErrSingleMediaRequired = errors.New("story and reel require exactly one media item")
	ErrCaptionTooLong      = errors.New("caption exceeds maximum length of 2200 characters")
	ErrScheduledTimeInPast = errors.New("scheduled time must be in the future")

	// Business logic errors
	ErrPublicationNotFound    = errors.New("publication not found")
	ErrPublicationNotEditable = errors.New("publication cannot be edited in current status")
	ErrPublicationNotDeletable = errors.New("published content cannot be deleted from our system")
	ErrInvalidPublicationType = errors.New("invalid publication type")
	ErrInvalidStatus          = errors.New("invalid publication status")

	// Instagram API errors
	ErrInstagramAPIFailure    = errors.New("instagram API request failed")
	ErrInstagramRateLimited   = errors.New("instagram API rate limit exceeded")
	ErrInstagramUnauthorized  = errors.New("instagram access token is invalid or expired")
	ErrContainerNotReady      = errors.New("media container is not ready for publishing")
	ErrDailyPublishingLimit   = errors.New("daily publishing limit exceeded (max 25 per day)")
)
