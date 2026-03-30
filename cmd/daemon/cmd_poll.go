package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran/agent-mesh/pkg/provider"
	"github.com/vutran/agent-mesh/pkg/silo"
	sqlitestore "github.com/vutran/agent-mesh/pkg/store/sqlite"
)

func newPollCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "poll <service> <label>",
		Short: "Poll a connection once",
		Example: `  agent-mesh poll gmail personal
  agent-mesh poll slack work`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPoll(cmd.Context(), args[0], args[1])
		},
	}
}

func runPoll(ctx context.Context, service, label string) error {
	conn := cfg.FindConnection(service, label)
	if conn == nil {
		return fmt.Errorf("no connection for %s/%s — run 'agent-mesh add %s %s'", service, label, service, label)
	}

	p, err := provider.Get(conn.Provider, dataDir)
	if err != nil {
		return err
	}

	connID := conn.ConnectionID
	if conn.Token != "" {
		connID = conn.Token
	}
	adapter, err := p.NewPoller(service, label, connID, logger)
	if err != nil {
		return err
	}

	return runPollAdapter(ctx, adapter)
}

func runPollAdapter(ctx context.Context, adapter silo.Poller) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	store, err := sqlitestore.New(filepath.Join(dataDir, "messages.db"))
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	cursor, err := store.LoadCursor(ctx, adapter.Name())
	if err != nil {
		logger.Warn("failed to load cursor, starting fresh", "err", err)
	}

	fmt.Printf("Polling %s...\n", adapter.Name())
	start := time.Now()

	msgs, newCursor, err := adapter.Poll(ctx, cursor)
	if err != nil {
		return fmt.Errorf("poll failed: %w", err)
	}

	fmt.Printf("Found %d new message(s) in %s\n\n", len(msgs), time.Since(start).Round(time.Millisecond))

	for i, msg := range msgs {
		printMessageSummary(i+1, msg)
	}

	if len(msgs) > 0 {
		if err := store.Put(ctx, msgs...); err != nil {
			return fmt.Errorf("save messages: %w", err)
		}
		fmt.Printf("Saved %d message(s) to local store\n", len(msgs))
	}

	if newCursor != nil {
		if err := store.SaveCursor(ctx, adapter.Name(), newCursor); err != nil {
			return fmt.Errorf("save cursor: %w", err)
		}
		printCursorUpdate(newCursor)
	}

	return nil
}

func printMessageSummary(n int, msg silo.Message) {
	fmt.Printf("─── %d ───\n", n)
	fmt.Printf("  ID:      %s\n", msg.ID)
	fmt.Printf("  From:    %s\n", msg.Sender)
	fmt.Printf("  Subject: %s\n", msg.Subject)
	fmt.Printf("  Date:    %s\n", msg.SourceTS.Format(time.RFC1123))
	if msg.Preview != "" {
		preview := msg.Preview
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		fmt.Printf("  Preview: %s\n", preview)
	}
	fmt.Println()
}

func printCursorUpdate(cursorBytes silo.Cursor) {
	var cur struct {
		ModifiedAfter time.Time `json:"modified_after"`
		After         time.Time `json:"after"`
		AfterEpoch    int64     `json:"after_epoch"`
	}
	json.Unmarshal(cursorBytes, &cur)
	switch {
	case !cur.ModifiedAfter.IsZero():
		fmt.Printf("Cursor updated (%s)\n", cur.ModifiedAfter.Format(time.RFC1123))
	case !cur.After.IsZero():
		fmt.Printf("Cursor updated (%s)\n", cur.After.Format(time.RFC1123))
	case cur.AfterEpoch > 0:
		fmt.Printf("Cursor updated (%s)\n", time.Unix(cur.AfterEpoch, 0).Format(time.RFC1123))
	}
}
