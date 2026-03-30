package nango

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// cursor tracks the last modified_after timestamp for incremental fetches.
type cursor struct {
	ModifiedAfter time.Time `json:"modified_after"`
}

// Adapter polls a Nango integration for synced records.
type Adapter struct {
	name         string // unique name, e.g. "nango:gmail:personal"
	connectionID string // Nango connection ID
	service      ServiceConfig
	client       *Client
	logger       *slog.Logger
}

// NewAdapter creates a Nango adapter for a specific service and connection.
//
// connectionID identifies the Nango connection (each OAuth account is a connection).
// label is a human label (e.g. "personal", "work") used in the adapter name.
func NewAdapter(client *Client, service ServiceConfig, connectionID string, label string, logger *slog.Logger) *Adapter {
	name := fmt.Sprintf("nango:%s:%s", service.ProviderConfigKey, label)
	return &Adapter{
		name:         name,
		connectionID: connectionID,
		service:      service,
		client:       client,
		logger:       logger.With("adapter", name),
	}
}

func (a *Adapter) Name() string           { return a.name }
func (a *Adapter) Source() silo.Source    { return a.service.Source }
func (a *Adapter) Mode() silo.AdapterMode { return silo.ModePoll }

func (a *Adapter) Poll(ctx context.Context, since silo.Cursor) ([]silo.Message, silo.Cursor, error) {
	var cur cursor
	if since != nil {
		if err := json.Unmarshal(since, &cur); err != nil {
			a.logger.Warn("invalid cursor, starting fresh", "err", err)
		}
	}

	// Default to last 24 hours if no cursor
	if cur.ModifiedAfter.IsZero() {
		cur.ModifiedAfter = time.Now().Add(-24 * time.Hour)
	}

	var allMsgs []silo.Message
	latestModified := cur.ModifiedAfter
	pageCursor := ""

	for {
		result, err := a.client.FetchRecords(ctx, FetchRecordsInput{
			ProviderConfigKey: a.service.ProviderConfigKey,
			ConnectionID:      a.connectionID,
			Model:             a.service.Model,
			ModifiedAfter:     cur.ModifiedAfter,
			Cursor:            pageCursor,
			Limit:             100,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("fetch records: %w", err)
		}

		for _, rec := range result.Records {
			// Skip deleted records
			if rec.DeletedAt != nil {
				continue
			}

			msg := silo.Message{
				ID:         fmt.Sprintf("%s:%s:%s", a.service.Source, a.connectionID, rec.ID),
				Source:     a.service.Source,
				Raw:        rec.Data,
				CapturedAt: time.Now(),
				SourceTS:   rec.CreatedAt,
			}

			if a.service.Mapper.Sender != nil {
				msg.Sender = a.service.Mapper.Sender(rec.Data)
			}
			if a.service.Mapper.Subject != nil {
				msg.Subject = a.service.Mapper.Subject(rec.Data)
			}
			if a.service.Mapper.Preview != nil {
				msg.Preview = a.service.Mapper.Preview(rec.Data)
				if len(msg.Preview) > 500 {
					msg.Preview = msg.Preview[:500]
				}
			}

			allMsgs = append(allMsgs, msg)

			if rec.UpdatedAt.After(latestModified) {
				latestModified = rec.UpdatedAt
			}
		}

		if result.NextCursor == "" {
			break
		}
		pageCursor = result.NextCursor
	}

	newCursor := cursor{ModifiedAfter: latestModified}
	cursorBytes, _ := json.Marshal(newCursor)

	return allMsgs, silo.Cursor(cursorBytes), nil
}
