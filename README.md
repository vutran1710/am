# AMesh

A personal message silo that aggregates communications from Gmail, Slack, Discord, and Google Calendar into a single queryable store.

## Architecture

```
Browser (Chrome)                        Desktop
┌──────────────────────┐              ┌──────────────┐
│ Gmail tab            │              │  am-client   │
│ Slack tab     Claude │   POST      │  (CLI/tray)  │
│ Discord tab   Ext.   │──/ingest──► │              │
│ Calendar tab  + JS   │             │  local SQLite │
└──────────────────────┘              └──────┬───────┘
                                             │ GET /api
                                      ┌──────▼───────┐
                                      │  am-server   │
                                      │  HTTP API    │
                                      │  SQLite+FTS5 │
                                      └──────────────┘
                                             │
                                      ┌──────▼───────┐
                                      │ Claude App   │
                                      │ (mobile)     │
                                      │ reads + evals│
                                      └──────────────┘
```

**No third-party integrations.** Data flows through the browser — where you're already authenticated.

### How it works

1. **Ingest**: Chrome tabs with Claude Browser Extension inject JS that polls for new messages and POSTs them to am-server
2. **Store**: am-server saves everything to SQLite with full-text search
3. **Query**: am-client reads from am-server, keeps its own local DB for structured notes
4. **Evaluate**: Claude app on mobile accesses the API, reads messages, and summarizes what's important

## Components

### am-server

Headless HTTP service. Receives raw messages, stores them, serves queries.

```
POST /ingest              Accept messages (API key auth)
GET  /api/messages        List/search messages
GET  /api/messages/{id}   Get single message
GET  /api/stats           Message counts by source
GET  /healthz             Health check
```

```bash
# Build and run
go build -o bin/am-server ./cmd/am-server/
./bin/am-server

# API key auto-generated in ~/.agent-mesh/config.toml
```

### am-client

Desktop CLI tool. Queries am-server and manages a local SQLite database.

**Server queries:**

```bash
am list                          # recent messages
am list --source gmail -n 5      # filter by source
am search "meeting"              # full-text search
am get <id>                      # single message + raw payload
am stats                         # counts by source
```

**Local database (user-defined tables):**

```bash
am db create tasks title:text status:text priority:int due:text
am db write tasks '{"title":"Review PR","status":"pending","priority":1}'
am db read tasks --where "status=pending" --limit 5
am db tables                     # list all tables
am db drop tasks                 # remove a table
```

```bash
# Build
go build -o bin/am ./cmd/am-client/

# Configure
export AM_SERVER=http://localhost:8090
export AM_API_KEY=your-key-from-config
```

## Config

Single file at `~/.agent-mesh/config.toml`:

```toml
[server]
  addr = ":8090"
  api_key = "auto-generated-on-first-run"
```

## Project structure

```
cmd/am-server/       HTTP server (ingest + query)
cmd/am-client/       CLI client (query + local DB)
pkg/silo/            Message types + Store interface
pkg/store/sqlite/    SQLite storage with FTS5
pkg/config/          TOML config
pkg/log/             Structured logger
```

## Development

```bash
# Build both
go build -o bin/am-server ./cmd/am-server/
go build -o bin/am ./cmd/am-client/

# Run server
./bin/am-server

# Test ingest
curl -X POST http://localhost:8090/ingest \
  -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '[{"source":"gmail","sender":"alice@test.com","subject":"Hello","preview":"Hi there"}]'

# Query via client
AM_API_KEY=your-key am list
```
