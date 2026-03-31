package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	serverURL string
	apiKey    string
)

func main() {
	root := &cobra.Command{
		Use:   "am",
		Short: "AMesh client — query your message silo",
	}

	root.PersistentFlags().StringVar(&serverURL, "server", envOr("AM_SERVER", "http://localhost:8090"), "AM server URL")
	root.PersistentFlags().StringVar(&apiKey, "key", os.Getenv("AM_API_KEY"), "API key")

	root.AddCommand(
		newListCmd(),
		newGetCmd(),
		newStatsCmd(),
		newSearchCmd(),
		newDBCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func apiGet(path string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("key", apiKey)

	reqURL := serverURL + path + "?" + params.Encode()
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

type message struct {
	ID         string          `json:"id"`
	Source     string          `json:"source"`
	Sender     string          `json:"sender"`
	Subject    string          `json:"subject"`
	Preview    string          `json:"preview"`
	Raw        json.RawMessage `json:"raw,omitempty"`
	CapturedAt time.Time       `json:"captured_at"`
	SourceTS   time.Time       `json:"source_ts"`
}

func newListCmd() *cobra.Command {
	var (
		source string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent messages",
		Example: `  am list
  am list --source gmail --limit 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if source != "" {
				params.Set("source", source)
			}
			if limit > 0 {
				params.Set("limit", fmt.Sprintf("%d", limit))
			}

			body, err := apiGet("/api/messages", params)
			if err != nil {
				return err
			}

			var msgs []message
			json.Unmarshal(body, &msgs)

			if len(msgs) == 0 {
				fmt.Println("No messages.")
				return nil
			}

			for i, m := range msgs {
				fmt.Printf("─── %d ───\n", i+1)
				fmt.Printf("  ID:      %s\n", m.ID)
				fmt.Printf("  Source:  %s\n", m.Source)
				fmt.Printf("  From:    %s\n", m.Sender)
				fmt.Printf("  Subject: %s\n", m.Subject)
				fmt.Printf("  Date:    %s\n", m.SourceTS.Format(time.RFC1123))
				if m.Preview != "" {
					preview := m.Preview
					if len(preview) > 120 {
						preview = preview[:120] + "..."
					}
					fmt.Printf("  Preview: %s\n", preview)
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Filter by source")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Max results")
	return cmd
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a message by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/api/messages/"+args[0], nil)
			if err != nil {
				return err
			}

			var m message
			json.Unmarshal(body, &m)

			fmt.Printf("ID:       %s\n", m.ID)
			fmt.Printf("Source:   %s\n", m.Source)
			fmt.Printf("From:     %s\n", m.Sender)
			fmt.Printf("Subject:  %s\n", m.Subject)
			fmt.Printf("Date:     %s\n", m.SourceTS.Format(time.RFC1123))
			fmt.Printf("Preview:  %s\n", m.Preview)
			fmt.Println("\n─── Raw ───")
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(m.Raw)
			return nil
		},
	}
}

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "search <query>",
		Short:   "Full-text search messages",
		Args:    cobra.ExactArgs(1),
		Example: "  am search meeting",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{"q": {args[0]}}
			body, err := apiGet("/api/messages", params)
			if err != nil {
				return err
			}

			var msgs []message
			json.Unmarshal(body, &msgs)

			fmt.Printf("Found %d result(s) for %q:\n\n", len(msgs), args[0])
			for i, m := range msgs {
				fmt.Printf("  %d. [%s] %s — %s\n", i+1, m.Source, m.Subject, m.Sender)
			}
			return nil
		},
	}
}

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show message statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/api/stats", nil)
			if err != nil {
				return err
			}

			var stats struct {
				Total   int `json:"total"`
				Sources []struct {
					Source string `json:"source"`
					Count  int    `json:"count"`
				} `json:"sources"`
			}
			json.Unmarshal(body, &stats)

			fmt.Printf("Total messages: %d\n\n", stats.Total)
			for _, s := range stats.Sources {
				fmt.Printf("  %-10s %d\n", s.Source, s.Count)
			}
			return nil
		},
	}
}
