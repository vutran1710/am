package silo

import "context"

// Standardized holds the output of Stage 1 (LLM standardization).
type Standardized struct {
	MessageID string `json:"message_id"`
	CleanText string `json:"clean_text"`
	FromName  string `json:"from_name"`
	FromEmail string `json:"from_email"`
	Action    string `json:"action"`
	Deadline  string `json:"deadline"`
	Summary   string `json:"summary"`
}

// Evaluation holds the output of Stage 2 (LLM evaluation).
type Evaluation struct {
	MessageID    string `json:"message_id"`
	Importance   int    `json:"importance"`
	Category     string `json:"category"`
	ActionNeeded bool   `json:"action_needed"`
	Urgency      string `json:"urgency"`
	Notify       bool   `json:"notify"`
	Reason       string `json:"reason"`
}

// PipelineStore extends Store with pipeline-specific operations.
type PipelineStore interface {
	Store
	ListByStatus(ctx context.Context, status string, limit int) ([]Message, error)
	UpdateStatus(ctx context.Context, id, status string) error
	SaveStandardized(ctx context.Context, s *Standardized) error
	GetStandardized(ctx context.Context, messageID string) (*Standardized, error)
	SaveEvaluation(ctx context.Context, e *Evaluation) error
	GetEvaluation(ctx context.Context, messageID string) (*Evaluation, error)
}
