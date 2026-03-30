package pipeline

import (
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// ProcessedMessage is a message with its standardized and evaluated data.
type ProcessedMessage struct {
	ID           string             `json:"id"`
	Source       string             `json:"source"`
	SourceTS     time.Time          `json:"source_ts"`
	Standardized *silo.Standardized `json:"standardized,omitempty"`
	Evaluation   *silo.Evaluation   `json:"evaluation,omitempty"`
}
