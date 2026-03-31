package silo

import (
	"encoding/json"
	"time"
)

// Source identifies the communication service a message came from.
type Source string

const (
	SourceGmail   Source = "gmail"
	SourceGCal    Source = "gcal"
	SourceSlack   Source = "slack"
	SourceDiscord Source = "discord"
)

// Message is the unified envelope for all captured messages.
type Message struct {
	ID         string          `json:"id"`
	Source     Source          `json:"source"`
	Sender     string          `json:"sender,omitempty"`
	Subject    string          `json:"subject,omitempty"`
	Preview    string          `json:"preview,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
	CapturedAt time.Time       `json:"captured_at"`
	SourceTS   time.Time       `json:"source_ts,omitempty"`
}
