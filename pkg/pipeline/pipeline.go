package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/vutran/agent-mesh/pkg/llm"
	"github.com/vutran/agent-mesh/pkg/silo"
)

// Notifier receives important messages.
type Notifier interface {
	Notify(ctx context.Context, msg silo.Message, std *silo.Standardized, eval *silo.Evaluation) error
}

// Pipeline processes messages through Stage 1 (standardize) and Stage 2 (evaluate).
type Pipeline struct {
	store        silo.PipelineStore
	stage1       llm.LLM
	stage2       llm.LLM
	profile      string
	instructions string
	notifyMin    int
	notifier     Notifier
	logger       *slog.Logger
}

// Config for creating a pipeline.
type Config struct {
	Store        silo.PipelineStore
	Stage1       llm.LLM
	Stage2       llm.LLM
	Profile      string
	Instructions string
	NotifyMin    int
	Notifier     Notifier
	Logger       *slog.Logger
}

// New creates a message processing pipeline.
func New(cfg Config) *Pipeline {
	if cfg.NotifyMin == 0 {
		cfg.NotifyMin = 7
	}
	return &Pipeline{
		store:        cfg.Store,
		stage1:       cfg.Stage1,
		stage2:       cfg.Stage2,
		profile:      cfg.Profile,
		instructions: cfg.Instructions,
		notifyMin:    cfg.NotifyMin,
		notifier:     cfg.Notifier,
		logger:       cfg.Logger,
	}
}

// Run processes messages in a loop until ctx is cancelled.
func (p *Pipeline) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		processed := 0

		// Stage 1: standardize raw messages
		raw, err := p.store.ListByStatus(ctx, "raw", 10)
		if err != nil {
			p.logger.Error("list raw messages", "err", err)
		} else {
			for _, msg := range raw {
				if err := p.standardize(ctx, msg); err != nil {
					p.logger.Error("standardize failed", "id", msg.ID, "err", err)
					continue
				}
				processed++
				time.Sleep(1 * time.Second) // rate limit protection
			}
		}

		// Stage 2: evaluate standardized messages
		pending, err := p.store.ListByStatus(ctx, "standardized", 10)
		if err != nil {
			p.logger.Error("list standardized messages", "err", err)
		} else {
			for _, msg := range pending {
				if err := p.evaluate(ctx, msg); err != nil {
					p.logger.Error("evaluate failed", "id", msg.ID, "err", err)
					continue
				}
				processed++
				time.Sleep(1 * time.Second) // rate limit protection
			}
		}

		if processed == 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (p *Pipeline) standardize(ctx context.Context, msg silo.Message) error {
	system := standardizeSystemPrompt(p.profile, p.instructions)
	prompt := standardizeUserPrompt(
		string(msg.Source), msg.Sender, msg.Subject, msg.Preview, string(msg.Raw),
	)

	resp, err := p.stage1.Complete(ctx, system, prompt)
	if err != nil {
		return fmt.Errorf("llm: %w", err)
	}

	var std silo.Standardized
	if err := json.Unmarshal([]byte(resp), &std); err != nil {
		return fmt.Errorf("parse response: %w (response: %s)", err, truncate(resp, 200))
	}
	std.MessageID = msg.ID

	if err := p.store.SaveStandardized(ctx, &std); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	return p.store.UpdateStatus(ctx, msg.ID, "standardized")
}

func (p *Pipeline) evaluate(ctx context.Context, msg silo.Message) error {
	std, err := p.store.GetStandardized(ctx, msg.ID)
	if err != nil || std == nil {
		return fmt.Errorf("load standardized: %w", err)
	}

	system := evaluateSystemPrompt(p.profile, p.instructions)
	prompt := evaluateUserPrompt(std, string(msg.Source))

	resp, err := p.stage2.Complete(ctx, system, prompt)
	if err != nil {
		return fmt.Errorf("llm: %w", err)
	}

	var eval silo.Evaluation
	if err := json.Unmarshal([]byte(resp), &eval); err != nil {
		return fmt.Errorf("parse response: %w (response: %s)", err, truncate(resp, 200))
	}
	eval.MessageID = msg.ID
	eval.Notify = eval.Importance >= p.notifyMin

	if err := p.store.SaveEvaluation(ctx, &eval); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	if err := p.store.UpdateStatus(ctx, msg.ID, "evaluated"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if eval.Notify && p.notifier != nil {
		if err := p.notifier.Notify(ctx, msg, std, &eval); err != nil {
			p.logger.Error("notify failed", "id", msg.ID, "err", err)
		}
	}

	p.logger.Info("evaluated message",
		"id", msg.ID,
		"importance", eval.Importance,
		"category", eval.Category,
		"notify", eval.Notify,
		"reason", eval.Reason,
	)

	return nil
}
