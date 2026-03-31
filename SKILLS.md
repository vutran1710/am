# Agent Mesh Server — Skill Reference

Use this skill to read and write messages to the Agent Mesh (am-server). The server aggregates messages from Gmail, Slack, Discord, Zalo, Google Calendar, and any custom source into a single searchable store.

## Connection

Set these environment variables before use:

```bash
export AM_SERVER="http://<host>:8090"   # am-server address
export AM_API_KEY="<your-api-key>"      # from ~/.agent-mesh/config.toml on the server
```

All endpoints except `/healthz` require the API key as a header:

```
X-API-Key: $AM_API_KEY
```

## Endpoints

### Health Check

```
GET /healthz
```

No auth required. Returns `ok` if the server is running.

### Ingest Messages

```
POST /ingest
Content-Type: application/json
X-API-Key: <key>
```

Body — array of messages:

```json
[
  {
    "source": "gmail",
    "sender": "alice@example.com",
    "subject": "Meeting tomorrow",
    "preview": "Let's sync at 2pm",
    "raw": {},
    "captured_at": "2026-03-31T10:00:00Z",
    "source_ts": "2026-03-31T09:55:00Z"
  }
]
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | yes | `gmail`, `slack`, `discord`, `zalo`, `gcal`, or any custom string |
| `sender` | string | no | Who sent it |
| `subject` | string | no | Title or subject line |
| `preview` | string | no | Text snippet / body preview |
| `raw` | object | no | Original payload (any JSON, preserved as-is) |
| `captured_at` | ISO 8601 | no | When captured (defaults to now) |
| `source_ts` | ISO 8601 | no | Original timestamp (defaults to now) |
| `id` | string | no | Auto-generated as `source:timestamp:index` if empty |

Response: `{"ingested": N}`

### List Messages

```
GET /api/messages?source=gmail&q=search+term&limit=20
X-API-Key: <key>
```

| Param | Description |
|-------|-------------|
| `source` | Filter by source (e.g. `gmail`, `slack`, `zalo`) |
| `q` | Full-text search across sender, subject, preview |
| `limit` | Max results (default: all) |

Returns JSON array of messages.

### Get Single Message

```
GET /api/messages/{id}
X-API-Key: <key>
```

Returns the full message including `raw` payload. 404 if not found.

### Stats

```
GET /api/stats
X-API-Key: <key>
```

Returns:

```json
{
  "total": 151,
  "sources": [
    {"source": "gmail", "count": 150},
    {"source": "slack", "count": 1}
  ]
}
```

## Common Tasks

### Check what's new

```bash
curl -H "X-API-Key: $AM_API_KEY" \
  "$AM_SERVER/api/messages?limit=10"
```

### Search for something specific

```bash
curl -H "X-API-Key: $AM_API_KEY" \
  "$AM_SERVER/api/messages?q=deploy+failed"
```

### Filter by source

```bash
curl -H "X-API-Key: $AM_API_KEY" \
  "$AM_SERVER/api/messages?source=zalo&limit=5"
```

### Post a note or task (use any custom source)

```bash
curl -X POST -H "X-API-Key: $AM_API_KEY" \
  -H "Content-Type: application/json" \
  "$AM_SERVER/ingest" \
  -d '[{"source":"note","sender":"me","subject":"TODO","preview":"Fix the CI pipeline"}]'
```

## Instructions for Claude

When the user asks you to check messages, read email, or look at notifications:

1. Call `GET /api/messages` with appropriate filters
2. Summarize the results — group by source, highlight important items
3. Skip duplicates (same subject + sender = same message)
4. Flag anything that looks urgent: failed CI, security alerts, direct messages from people (not bots)

When the user asks you to remember something or save a note:

1. Call `POST /ingest` with `source: "note"` and the content in `subject` + `preview`

When the user asks about a specific topic:

1. Use `GET /api/messages?q=<search>` to find relevant messages across all sources

The server is always running at `$AM_SERVER`. If it's unreachable, the droplet may be down.
