package entity

// PublicationStatistics represents aggregated publication statistics
type PublicationStatistics struct {
	ScheduledCount int                  `json:"scheduled_count"` // Count of scheduled publications
	PublishedCount int                  `json:"published_count"` // Count of successfully published
	ErrorCount     int                  `json:"error_count"`     // Count of publications with errors
	DraftCount     int                  `json:"draft_count"`     // Count of drafts
	ByType         TypeBreakdown        `json:"by_type"`         // Breakdown by publication type
}

// TypeBreakdown represents statistics breakdown by publication type
type TypeBreakdown struct {
	Post  TypeStats `json:"post"`
	Story TypeStats `json:"story"`
	Reel  TypeStats `json:"reel"`
}

// TypeStats represents statistics for a specific publication type
type TypeStats struct {
	ScheduledCount int `json:"scheduled_count"`
	PublishedCount int `json:"published_count"`
	ErrorCount     int `json:"error_count"`
	DraftCount     int `json:"draft_count"`
}
