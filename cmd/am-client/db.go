package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/spf13/cobra"
)

var localDB *sql.DB

func openLocalDB() (*sql.DB, error) {
	if localDB != nil {
		return localDB, nil
	}

	dir := envOr("AM_DATA_DIR", defaultDataDir())
	os.MkdirAll(dir, 0o700)

	db, err := sql.Open("sqlite", filepath.Join(dir, "client.db"))
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA journal_mode=WAL")
	localDB = db
	return db, nil
}

func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".agent-mesh")
	}
	return ".agent-mesh"
}

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Local database operations",
	}

	cmd.AddCommand(
		newDBTablesCmd(),
		newDBCreateCmd(),
		newDBWriteCmd(),
		newDBReadCmd(),
		newDBDropCmd(),
	)
	return cmd
}

// am db tables
func newDBTablesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tables",
		Short: "List all tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openLocalDB()
			if err != nil {
				return err
			}

			rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
			if err != nil {
				return err
			}
			defer rows.Close()

			var tables []string
			for rows.Next() {
				var name string
				rows.Scan(&name)
				tables = append(tables, name)
			}

			if len(tables) == 0 {
				fmt.Println("No tables. Create one with: am db create <table> <col:type> ...")
				return nil
			}

			for _, t := range tables {
				// Get column info
				cols, _ := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", t))
				var colDefs []string
				if cols != nil {
					for cols.Next() {
						var cid int
						var name, typ string
						var notnull int
						var dflt *string
						var pk int
						cols.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
						colDefs = append(colDefs, fmt.Sprintf("%s:%s", name, typ))
					}
					cols.Close()
				}

				var count int
				db.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s", t)).Scan(&count)
				fmt.Printf("  %s (%d rows) — %s\n", t, count, strings.Join(colDefs, ", "))
			}
			return nil
		},
	}
}

// am db create <table> <col:type> [col:type...]
// Types: text, int, real, json
func newDBCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "create <table> <col:type> [col:type...]",
		Short:   "Create a table",
		Args:    cobra.MinimumNArgs(2),
		Example: "  am db create tasks title:text status:text priority:int due:text",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openLocalDB()
			if err != nil {
				return err
			}

			table := args[0]
			var cols []string
			cols = append(cols, "id INTEGER PRIMARY KEY AUTOINCREMENT")
			cols = append(cols, "created_at TEXT DEFAULT (datetime('now'))")

			for _, arg := range args[1:] {
				parts := strings.SplitN(arg, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid column %q — use name:type (text, int, real, json)", arg)
				}
				name, typ := parts[0], strings.ToUpper(parts[1])
				switch typ {
				case "TEXT", "INT", "INTEGER", "REAL", "JSON":
					cols = append(cols, fmt.Sprintf("%s %s", name, typ))
				default:
					return fmt.Errorf("unknown type %q — use text, int, real, json", typ)
				}
			}

			ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", table, strings.Join(cols, ", "))
			if _, err := db.Exec(ddl); err != nil {
				return fmt.Errorf("create table: %w", err)
			}

			fmt.Printf("Created table %q\n", table)
			return nil
		},
	}
}

// am db write <table> <json>
func newDBWriteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "write <table> <json>",
		Short:   "Insert a row",
		Args:    cobra.ExactArgs(2),
		Example: `  am db write tasks '{"title":"Review PR","status":"pending","priority":1}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openLocalDB()
			if err != nil {
				return err
			}

			table := args[0]
			var data map[string]any
			if err := json.Unmarshal([]byte(args[1]), &data); err != nil {
				return fmt.Errorf("invalid json: %w", err)
			}

			var cols []string
			var placeholders []string
			var vals []any
			for k, v := range data {
				cols = append(cols, k)
				placeholders = append(placeholders, "?")
				vals = append(vals, v)
			}

			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

			result, err := db.Exec(query, vals...)
			if err != nil {
				return fmt.Errorf("insert: %w", err)
			}

			id, _ := result.LastInsertId()
			fmt.Printf("Inserted row %d into %s\n", id, table)
			return nil
		},
	}
}

// am db read <table> [--where col=val] [--limit N]
func newDBReadCmd() *cobra.Command {
	var (
		where string
		limit int
	)

	cmd := &cobra.Command{
		Use:   "read <table>",
		Short: "Query rows from a table",
		Args:  cobra.ExactArgs(1),
		Example: `  am db read tasks
  am db read tasks --where "status=pending" --limit 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openLocalDB()
			if err != nil {
				return err
			}

			table := args[0]
			query := fmt.Sprintf("SELECT * FROM %s", table)
			var queryArgs []any

			if where != "" {
				parts := strings.SplitN(where, "=", 2)
				if len(parts) == 2 {
					query += fmt.Sprintf(" WHERE %s = ?", parts[0])
					queryArgs = append(queryArgs, parts[1])
				}
			}

			query += " ORDER BY id DESC"
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.Query(query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			columns, _ := rows.Columns()
			count := 0

			for rows.Next() {
				vals := make([]any, len(columns))
				ptrs := make([]any, len(columns))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				rows.Scan(ptrs...)

				count++
				fmt.Printf("─── %d ───\n", count)
				for i, col := range columns {
					fmt.Printf("  %-15s %v\n", col+":", vals[i])
				}
				fmt.Println()
			}

			if count == 0 {
				fmt.Printf("No rows in %s\n", table)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&where, "where", "", "Filter (col=value)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Max rows")
	return cmd
}

// am db drop <table>
func newDBDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop <table>",
		Short: "Drop a table",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openLocalDB()
			if err != nil {
				return err
			}

			table := args[0]
			if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
				return err
			}

			fmt.Printf("Dropped table %q\n", table)
			return nil
		},
	}
}
