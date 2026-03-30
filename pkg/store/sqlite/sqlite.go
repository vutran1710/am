package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vutran/agent-mesh/pkg/silo"
	_ "modernc.org/sqlite"
)

// Store implements silo.Store backed by SQLite with FTS5 full-text search.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at the given path.
// Use ":memory:" for an in-memory database (useful in tests).
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	const ddl = `
	CREATE TABLE IF NOT EXISTS messages (
		id          TEXT PRIMARY KEY,
		source      TEXT NOT NULL,
		sender      TEXT DEFAULT '',
		subject     TEXT DEFAULT '',
		preview     TEXT DEFAULT '',
		raw         TEXT NOT NULL,
		captured_at INTEGER NOT NULL,
		source_ts   INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_messages_source_ts ON messages(source, source_ts);

	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		sender, subject, preview, content=messages, content_rowid=rowid
	);

	CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
		INSERT INTO messages_fts(rowid, sender, subject, preview)
		VALUES (new.rowid, new.sender, new.subject, new.preview);
	END;

	CREATE TABLE IF NOT EXISTS cursors (
		adapter TEXT PRIMARY KEY,
		data    BLOB NOT NULL
	);
	`
	_, err := db.Exec(ddl)
	return err
}

func (s *Store) Put(ctx context.Context, msgs ...silo.Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO messages (id, source, sender, subject, preview, raw, captured_at, source_ts)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range msgs {
		raw, err := json.Marshal(m.Raw)
		if err != nil {
			return fmt.Errorf("marshal raw for %s: %w", m.ID, err)
		}
		_, err = stmt.ExecContext(ctx,
			m.ID, string(m.Source), m.Sender, m.Subject, m.Preview,
			string(raw), m.CapturedAt.Unix(), m.SourceTS.Unix(),
		)
		if err != nil {
			return fmt.Errorf("insert %s: %w", m.ID, err)
		}
	}

	return tx.Commit()
}

func (s *Store) Get(ctx context.Context, id string) (*silo.Message, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, source, sender, subject, preview, raw, captured_at, source_ts
		 FROM messages WHERE id = ?`, id)
	return scanMessage(row)
}

func (s *Store) List(ctx context.Context, q silo.Query) ([]silo.Message, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}

	if q.Search != "" {
		return s.searchFTS(ctx, q, limit)
	}

	var (
		where []string
		args  []any
	)

	if q.Source != nil {
		where = append(where, "source = ?")
		args = append(args, string(*q.Source))
	}
	if q.Since != nil {
		where = append(where, "source_ts >= ?")
		args = append(args, q.Since.Unix())
	}
	if q.Until != nil {
		where = append(where, "source_ts <= ?")
		args = append(args, q.Until.Unix())
	}

	query := "SELECT id, source, sender, subject, preview, raw, captured_at, source_ts FROM messages"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY source_ts DESC LIMIT ? OFFSET ?"
	args = append(args, limit, q.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (s *Store) searchFTS(ctx context.Context, q silo.Query, limit int) ([]silo.Message, error) {
	var (
		where = []string{"messages_fts MATCH ?"}
		args  = []any{q.Search}
	)

	if q.Source != nil {
		where = append(where, "m.source = ?")
		args = append(args, string(*q.Source))
	}

	query := `SELECT m.id, m.source, m.sender, m.subject, m.preview, m.raw, m.captured_at, m.source_ts
		FROM messages m
		JOIN messages_fts ON messages_fts.rowid = m.rowid
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY rank LIMIT ? OFFSET ?`
	args = append(args, limit, q.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

func (s *Store) LoadCursor(ctx context.Context, adapterName string) (silo.Cursor, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM cursors WHERE adapter = ?`, adapterName).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load cursor %s: %w", adapterName, err)
	}
	return data, nil
}

func (s *Store) SaveCursor(ctx context.Context, adapterName string, c silo.Cursor) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cursors (adapter, data) VALUES (?, ?)
		 ON CONFLICT(adapter) DO UPDATE SET data = excluded.data`,
		adapterName, []byte(c))
	if err != nil {
		return fmt.Errorf("save cursor %s: %w", adapterName, err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// scanner abstracts *sql.Row and *sql.Rows for shared scanning.
type scanner interface {
	Scan(dest ...any) error
}

func scanMessage(sc scanner) (*silo.Message, error) {
	var (
		m          silo.Message
		source     string
		raw        string
		capturedAt int64
		sourceTS   int64
	)
	err := sc.Scan(&m.ID, &source, &m.Sender, &m.Subject, &m.Preview, &raw, &capturedAt, &sourceTS)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Source = silo.Source(source)
	m.Raw = json.RawMessage(raw)
	m.CapturedAt = time.Unix(capturedAt, 0)
	m.SourceTS = time.Unix(sourceTS, 0)
	return &m, nil
}

func scanMessages(rows *sql.Rows) ([]silo.Message, error) {
	var msgs []silo.Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, *m)
	}
	return msgs, rows.Err()
}
