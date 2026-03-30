package composio

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
)

func fetchGCalEvents(ctx context.Context, client *Client, connID, entityID string, since time.Time) ([]silo.Message, error) {
	args := map[string]any{
		"calendarId":    "primary",
		"time_min":      since.Format(time.RFC3339),
		"time_max":      time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
		"single_events": true,
	}

	result, err := client.ExecuteTool(ctx, "GOOGLECALENDAR_EVENTS_LIST", connID, entityID, args)
	if err != nil {
		return nil, fmt.Errorf("find events: %w", err)
	}

	return mapGCalEvents(result.Data), nil
}

type gcalEvent struct {
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	HTMLLink  string `json:"htmlLink"`
	Created   string `json:"created"`
	Organizer struct {
		Email string `json:"email"`
	} `json:"organizer"`
	Start struct {
		DateTime string `json:"dateTime"`
		Date     string `json:"date"`
	} `json:"start"`
	End struct {
		DateTime string `json:"dateTime"`
		Date     string `json:"date"`
	} `json:"end"`
}

func mapGCalEvents(data json.RawMessage) []silo.Message {
	// EVENTS_LIST response: {"items": [...]}
	var wrapper struct {
		Items []gcalEvent `json:"items"`
	}
	if json.Unmarshal(data, &wrapper) != nil {
		return nil
	}

	var msgs []silo.Message
	for _, e := range wrapper.Items {
		raw, _ := json.Marshal(e)

		startTime := e.Start.DateTime
		if startTime == "" {
			startTime = e.Start.Date
		}
		endTime := e.End.DateTime
		if endTime == "" {
			endTime = e.End.Date
		}

		preview := fmt.Sprintf("%s — %s", startTime, endTime)

		msg := silo.Message{
			ID:      e.ID,
			Source:  silo.SourceGCal,
			Sender:  e.Organizer.Email,
			Subject: e.Summary,
			Preview: preview,
			Raw:     raw,
		}

		if t, err := time.Parse(time.RFC3339, e.Start.DateTime); err == nil {
			msg.SourceTS = t
		} else if t, err := time.Parse("2006-01-02", e.Start.Date); err == nil {
			msg.SourceTS = t
		}

		msgs = append(msgs, msg)
	}

	return msgs
}
