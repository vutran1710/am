package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/vutran/agent-mesh/pkg/config"
	"github.com/vutran/agent-mesh/pkg/silo"
	sqlitestore "github.com/vutran/agent-mesh/pkg/store/sqlite"
)

func main() {
	dataDir := config.DataDir()
	cfg, err := config.Load(dataDir)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		logger.Error("create data dir", "err", err)
		os.Exit(1)
	}

	store, err := sqlitestore.New(filepath.Join(dataDir, "messages.db"))
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	mux := http.NewServeMux()
	s := &server{store: store, apiKey: cfg.Server.APIKey, logger: logger}

	// Public
	mux.HandleFunc("GET /healthz", s.handleHealth)

	// Authenticated
	mux.HandleFunc("POST /ingest", s.auth(s.handleIngest))
	mux.HandleFunc("GET /api/messages", s.auth(s.handleList))
	mux.HandleFunc("GET /api/messages/{id}", s.auth(s.handleGet))
	mux.HandleFunc("GET /api/stats", s.auth(s.handleStats))

	httpServer := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      cors(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		logger.Info("am-server starting", "addr", cfg.Server.Addr, "data_dir", dataDir)
		logger.Info("api key", "key", cfg.Server.APIKey)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
}

// cors wraps a handler with CORS headers for browser extensions.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type server struct {
	store  *sqlitestore.Store
	apiKey string
	logger *slog.Logger
}

// auth wraps a handler with API key authentication.
func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("key")
		}
		if key != s.apiKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

// handleIngest accepts messages from browser extensions / scripts.
// POST /ingest
// Body: single message or array of messages.
func (s *server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var msgs []silo.Message

	// Try array first, then single message
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&msgs); err != nil {
		// Reset and try single
		r.Body.Close()
		// Re-read won't work, so just return error
		http.Error(w, fmt.Sprintf(`{"error":"invalid json: %s"}`, err), http.StatusBadRequest)
		return
	}

	// Set defaults
	now := time.Now()
	for i := range msgs {
		if msgs[i].CapturedAt.IsZero() {
			msgs[i].CapturedAt = now
		}
		if msgs[i].SourceTS.IsZero() {
			msgs[i].SourceTS = now
		}
		if msgs[i].ID == "" {
			msgs[i].ID = fmt.Sprintf("%s:%d:%d", msgs[i].Source, now.UnixNano(), i)
		}
	}

	if err := s.store.Put(r.Context(), msgs...); err != nil {
		s.logger.Error("ingest failed", "err", err, "count", len(msgs))
		http.Error(w, `{"error":"store failed"}`, http.StatusInternalServerError)
		return
	}

	s.logger.Info("ingested", "count", len(msgs))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"ingested": len(msgs)})
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	q := silo.Query{}
	if src := r.URL.Query().Get("source"); src != "" {
		source := silo.Source(src)
		q.Source = &source
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q.Since = &t
		}
	}
	if until := r.URL.Query().Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			q.Until = &t
		}
	}
	if search := r.URL.Query().Get("q"); search != "" {
		q.Search = search
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		fmt.Sscanf(limit, "%d", &q.Limit)
	}

	msgs, err := s.store.List(r.Context(), q)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func (s *server) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msg, err := s.store.Get(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	if msg == nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	var total int
	s.store.DB().QueryRow("SELECT count(*) FROM messages").Scan(&total)

	type sourceStat struct {
		Source string `json:"source"`
		Count  int    `json:"count"`
	}
	var sources []sourceStat
	rows, _ := s.store.DB().Query("SELECT source, count(*) FROM messages GROUP BY source ORDER BY count(*) DESC")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var ss sourceStat
			rows.Scan(&ss.Source, &ss.Count)
			sources = append(sources, ss)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total":   total,
		"sources": sources,
	})
}
