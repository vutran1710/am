package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/vutran/agent-mesh/pkg/config"
	"github.com/vutran/agent-mesh/pkg/llm"
	"github.com/vutran/agent-mesh/pkg/pipeline"
	"github.com/vutran/agent-mesh/pkg/provider"
	"github.com/vutran/agent-mesh/pkg/silo"
	sqlitestore "github.com/vutran/agent-mesh/pkg/store/sqlite"
)

type Daemon struct {
	cfg      *config.Config
	logger   *slog.Logger
	server   *http.Server
	silo     *silo.Silo
	pipeline *pipeline.Pipeline
}

func NewDaemon(cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}

	store, err := sqlitestore.New(filepath.Join(dataDir, "messages.db"))
	if err != nil {
		return nil, err
	}

	s := silo.NewSilo(store, silo.RealClock{}, logger)

	if len(cfg.Connections) == 0 {
		logger.Warn("no connections configured (run 'agent-mesh init' and 'agent-mesh add')")
	} else {
		registerAdapters(s, cfg.Connections, logger)
	}

	// Set up pipeline if LLM is configured
	var pipe *pipeline.Pipeline
	if cfg.Pipeline.Stage1.Mode != "" {
		pipe, err = setupPipeline(cfg, store, logger)
		if err != nil {
			logger.Warn("pipeline not started", "err", err)
		} else {
			logger.Info("pipeline enabled",
				"stage1", cfg.Pipeline.Stage1.Mode,
				"stage2", cfg.Pipeline.Stage2.Mode,
				"notify_min", cfg.Pipeline.NotifyMin,
			)
		}
	}

	mux := http.NewServeMux()
	d := &Daemon{
		cfg:      cfg,
		logger:   logger,
		silo:     s,
		pipeline: pipe,
		server: &http.Server{
			Addr:         cfg.Daemon.Addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}

	mux.HandleFunc("GET /healthz", d.handleHealth)
	mux.HandleFunc("GET /api/messages", d.handleListMessages)
	mux.HandleFunc("GET /api/messages/{id}", d.handleGetMessage)

	return d, nil
}

func setupPipeline(cfg *config.Config, store *sqlitestore.Store, logger *slog.Logger) (*pipeline.Pipeline, error) {
	stage1, err := createLLMBackend(cfg.Pipeline.Stage1)
	if err != nil {
		return nil, err
	}

	stage2 := stage1 // default: same backend for both stages
	if cfg.Pipeline.Stage2.Mode != "" {
		stage2, err = createLLMBackend(cfg.Pipeline.Stage2)
		if err != nil {
			return nil, err
		}
	}

	profile, instructions := config.LoadContext(dataDir)

	return pipeline.New(pipeline.Config{
		Store:        store,
		Stage1:       stage1,
		Stage2:       stage2,
		Profile:      profile,
		Instructions: instructions,
		NotifyMin:    cfg.Pipeline.NotifyMin,
		Logger:       logger,
	}), nil
}

func createLLMBackend(cfg config.LLMBackendConfig) (llm.LLM, error) {
	switch cfg.Mode {
	case "api":
		if cfg.APIURL == "" {
			return nil, errors.New("api_url required for api mode")
		}
		return llm.NewAPIBackend(cfg.APIURL, cfg.APIKey, cfg.Model), nil
	case "stdin":
		if cfg.Command == "" {
			return nil, errors.New("command required for stdin mode")
		}
		return llm.NewStdinBackend(cfg.Command)
	default:
		return nil, errors.New("unknown llm mode: " + cfg.Mode)
	}
}

func registerAdapters(s *silo.Silo, conns []config.Connection, logger *slog.Logger) {
	for _, conn := range conns {
		p, err := provider.Get(conn.Provider, dataDir)
		if err != nil {
			logger.Warn("skipping connection", "label", conn.Label, "err", err)
			continue
		}

		connID := conn.ConnectionID
		if conn.Token != "" {
			connID = conn.Token
		}
		adapter, err := p.NewPoller(conn.Service, conn.Label, connID, logger)
		if err != nil {
			logger.Warn("skipping connection", "label", conn.Label, "err", err)
			continue
		}

		s.Register(adapter, silo.AdapterConfig{
			Interval:   conn.Interval.Duration(),
			BackoffMax: 10 * time.Minute,
		})
		logger.Info("registered adapter", "name", adapter.Name(), "interval", conn.Interval.Duration())
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	go func() {
		if err := d.silo.Run(ctx); err != nil && ctx.Err() == nil {
			d.logger.Error("silo scheduler error", "err", err)
		}
	}()

	if d.pipeline != nil {
		go func() {
			if err := d.pipeline.Run(ctx); err != nil && ctx.Err() == nil {
				d.logger.Error("pipeline error", "err", err)
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		if err := d.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		d.logger.Info("shutting down gracefully")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		d.silo.Close()
		return d.server.Shutdown(shutdownCtx)
	}
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (d *Daemon) handleListMessages(w http.ResponseWriter, r *http.Request) {
	q := silo.Query{}
	if src := r.URL.Query().Get("source"); src != "" {
		s := silo.Source(src)
		q.Source = &s
	}
	if search := r.URL.Query().Get("q"); search != "" {
		q.Search = search
	}
	msgs, err := d.silo.Store.List(r.Context(), q)
	if err != nil {
		d.logger.Error("list messages failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func (d *Daemon) handleGetMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msg, err := d.silo.Store.Get(r.Context(), id)
	if err != nil {
		d.logger.Error("get message failed", "err", err, "id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if msg == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}
