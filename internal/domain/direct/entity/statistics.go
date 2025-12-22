package entity

import "time"

// Statistics represents DM activity statistics for a period
type Statistics struct {
	TotalDialogs          int   `json:"total_dialogs"`
	NewDialogs            int   `json:"new_dialogs"`
	UniqueUsers           int   `json:"unique_users"`
	FirstResponseTimeMs   int64 `json:"first_response_time_ms"`   // Average in milliseconds
	AvgResponseTimeMs     int64 `json:"avg_response_time_ms"`     // Average in milliseconds
	BusiestDay            int   `json:"busiest_day"`              // 0=Sunday, 6=Saturday
	BusiestHour           int   `json:"busiest_hour"`             // 0-23
	TotalMessagesSent     int   `json:"total_messages_sent"`
	TotalMessagesReceived int   `json:"total_messages_received"`
}

// HeatmapCell represents a single cell in the activity heatmap
type HeatmapCell struct {
	Day   int `json:"day"`   // 0=Sunday, 6=Saturday
	Hour  int `json:"hour"`  // 0-23
	Count int `json:"count"` // Number of messages
}

// Heatmap represents activity distribution by day and hour
type Heatmap struct {
	Cells []HeatmapCell `json:"cells"`
}

// StatisticsFilter for querying statistics
type StatisticsFilter struct {
	AccountID string
	StartDate time.Time
	EndDate   time.Time
}

// TimeSlot represents a time slot for statistics
type TimeSlot struct {
	Start string `json:"start"` // e.g., "00:00"
	End   string `json:"end"`   // e.g., "03:00"
}

// TimeSlots defines the 3-hour time slots for heatmap grouping
var TimeSlots = []TimeSlot{
	{Start: "00:00", End: "03:00"},
	{Start: "03:00", End: "06:00"},
	{Start: "06:00", End: "09:00"},
	{Start: "09:00", End: "12:00"},
	{Start: "12:00", End: "15:00"},
	{Start: "15:00", End: "18:00"},
	{Start: "18:00", End: "21:00"},
	{Start: "21:00", End: "00:00"},
}

// DayNames for statistics display
var DayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
