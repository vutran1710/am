# AMesh — Development Progress

Last updated: 2026-03-31

## Architecture

```
Sources → Pollers → SQLite Store → LLM Pipeline → Notifications
                                      ↓
                                   HTTP API → macOS App (planned)
```

**Monorepo structure** (Go workspace):

```
cmd/daemon/           CLI + HTTP daemon (cobra)
pkg/silo/             Core types: Message, Poller, Watcher, Store, Scheduler
pkg/store/sqlite/     SQLite storage with FTS5 full-text search + pipeline tables
pkg/provider/         Provider interface + registry (self-registration via init())
pkg/adapter/composio/ Composio provider (Gmail, Slack, GCal via API)
pkg/adapter/nango/    Nango provider (Gmail, Slack, GCal — alternative)
pkg/adapter/discord/  Discord provider (direct bot token, discordgo)
pkg/adapter/gmail/    Direct Gmail adapter (standalone Google OAuth)
pkg/llm/              LLM abstraction: API backend (OpenAI-compat) + Stdin backend
pkg/pipeline/         Two-stage message processor (standardize → evaluate)
pkg/config/           Unified TOML config (~/.agent-mesh/config.toml)
pkg/log/              Structured JSON logger (slog)
```

---

## What's Done

### Phase 1 — Observe (backend complete)

- [x] **Daemon** — polls all connections on schedule, stores to SQLite
- [x] **Gmail** — 2 accounts (personal + work) via Composio
- [x] **Slack** — channel history via Composio (mentions filter pending)
- [x] **Google Calendar** — upcoming events via Composio
- [x] **Discord** — direct bot adapter using discordgo (waiting on server invite)
- [x] **SQLite store** — FTS5 full-text search, cursor persistence
- [x] **HTTP API** — `/healthz`, `/api/messages`, `/api/messages/{id}`, source/search filtering
- [x] **CLI** — `init`, `add`, `poll`, `serve`, `messages list/get`, `connections`
- [x] **Provider abstraction** — Composio, Nango, Discord all behind `provider.Provider` interface
- [x] **Unified config** — single `~/.agent-mesh/config.toml` with secrets, connections, pipeline config
- [x] **Connection management** — `am add gmail personal` auto-creates OAuth, generates auth link

### LLM Pipeline (built, needs LLM provider)

- [x] **LLM interface** — `llm.LLM` with `Complete(ctx, system, prompt)` — pure text in/out, no tool calling
- [x] **API backend** — any OpenAI-compatible endpoint (Ollama, OpenRouter, Anthropic, z.ai, etc.)
- [x] **Stdin backend** — pipe to any CLI process (claude, llama.cpp, etc.)
- [x] **Stage 1 (standardize)** — extract clean text, sender, action type, deadline, summary from raw messages
- [x] **Stage 2 (evaluate)** — score importance 1-10, categorize, determine urgency, decide notification
- [x] **Context files** — `~/.agent-mesh/context/profile.md` + `instructions.md` for user-specific prompts
- [x] **Pipeline runner** — processes messages in daemon loop with rate limit protection
- [ ] **LLM provider** — needs a working API key (tested z.ai but hit rate limits)

---

## What's Next

### Immediate (unblocked)

1. **LLM provider** — set up Anthropic API, OpenRouter, or local Ollama for pipeline testing
2. **Discord bot invite** — send invite link to server admin, test polling
3. **Slack mentions** — filter to DMs + mentions only (currently fetches all channel history)

### Phase 1b — macOS Status Bar App

- SwiftUI + AppKit, `NSStatusItem` menu bar app
- Dropdown: important messages with summaries (from evaluated pipeline output)
- macOS native notifications for high-importance messages
- DMG installer with onboarding wizard
- Targets non-technical users — no terminal exposure
- Daemon bundled inside .app, auto-launched via launchd

### Phase 2 — Analyze & Schedule

- LLM-powered daily schedule from incoming messages
- Interactive: confirm, reschedule, drop items

---

## Key Design Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Provider abstraction | `provider.Provider` interface | cmd/ layer is provider-agnostic |
| Config | Single TOML file | One place for everything, 0600 permissions |
| Service credentials | Per-connection `token` field | Bot tokens scoped to connection, not global secrets |
| LLM safety | Pure text in/out | No tool calling, no execution capability |
| LLM pipeline | Two stages | Stage 1 cheap/fast (standardize), Stage 2 smart (evaluate) |
| Storage | SQLite + FTS5 | Embedded, zero-ops, full-text search built in |
| Target user | Non-technical | macOS app is the product, CLI is for development |

---

## Connected Services (current test setup)

| Service | Provider | Status |
|---------|----------|--------|
| Gmail (personal) | Composio | Active |
| Gmail (work) | Composio | Active |
| Slack (work) | Composio | Active |
| Google Calendar (personal) | Composio | Active |
| Google Calendar (work) | Composio | Active |
| Discord (myserver) | Direct (bot) | Waiting on invite |
