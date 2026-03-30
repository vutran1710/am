package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran/agent-mesh/pkg/silo"
	sqlitestore "github.com/vutran/agent-mesh/pkg/store/sqlite"
)

func newMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Query stored messages",
	}
	cmd.AddCommand(newMessagesListCmd(), newMessagesGetCmd())
	return cmd
}

func newMessagesListCmd() *cobra.Command {
	var (
		source string
		search string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored messages",
		Example: `  agent-mesh messages list
  agent-mesh messages list --source gmail -n 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessagesList(cmd.Context(), source, search, limit)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Filter by source (gmail, slack, gcal)")
	cmd.Flags().StringVarP(&search, "search", "q", "", "Full-text search")
	cmd.Flags().IntVarP(&limit, "n", "n", 20, "Max results")

	return cmd
}

func newMessagesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <id>",
		Short:   "Get a message by ID (metadata + raw payload)",
		Args:    cobra.ExactArgs(1),
		Example: "  agent-mesh messages get gmail:personal:msg-123",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessagesGet(cmd.Context(), args[0])
		},
	}
}

func openStore() (*sqlitestore.Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	return sqlitestore.New(filepath.Join(dataDir, "messages.db"))
}

func runMessagesList(ctx context.Context, source, search string, limit int) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	q := silo.Query{Limit: limit}
	if source != "" {
		src := silo.Source(source)
		q.Source = &src
	}
	if search != "" {
		q.Search = search
	}

	msgs, err := store.List(ctx, q)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if len(msgs) == 0 {
		fmt.Println("No messages found.")
		return nil
	}

	fmt.Printf("Found %d message(s):\n\n", len(msgs))
	for i, msg := range msgs {
		fmt.Printf("─── %d ───\n", i+1)
		fmt.Printf("  ID:      %s\n", msg.ID)
		fmt.Printf("  Source:  %s\n", msg.Source)
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
	return nil
}

func runMessagesGet(ctx context.Context, id string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	msg, err := store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("message %q not found", id)
	}

	fmt.Printf("ID:         %s\n", msg.ID)
	fmt.Printf("Source:     %s\n", msg.Source)
	fmt.Printf("From:       %s\n", msg.Sender)
	fmt.Printf("Subject:    %s\n", msg.Subject)
	fmt.Printf("Date:       %s\n", msg.SourceTS.Format(time.RFC1123))
	fmt.Printf("Captured:   %s\n", msg.CapturedAt.Format(time.RFC1123))
	fmt.Printf("Preview:    %s\n", msg.Preview)
	fmt.Println()
	fmt.Println("─── Raw ───")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(json.RawMessage(msg.Raw))
	return nil
}
