# zdx integration

Push timings, errors, and logs from your service into a zdx project. This
document covers the timings ingest API; errors and logs follow the same
auth model and will be documented as their endpoints stabilize.

zdx itself uses this API to track its own timings — the zdx-server process
is a client of its own ingest endpoint. If it works for us, it works for you.

## Concepts

- **Project**: the dashboard namespace (e.g. `zdx`, `acme-app`). Created in
  the zdx UI.
- **Component**: a subsystem within the project. Optional. Typical values:
  `zdx-server`, `worker`, `cron-reports`. Lets one project aggregate
  distinct services under the same umbrella.
- **Environment**: deployment slot (e.g. `current`, `next`, `staging`,
  `prod`). Optional, free-text.
- **Integration token**: bearer credential that identifies a sender and
  resolves to a project. Tokens may carry a default component that the
  client can override per batch.

## Issue a token

An admin issues one token per integration:

```bash
curl -X POST https://zdx.example.com/api/admin/integration-tokens \
  -H "X-Api-Key: $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "slug":      "my-project",
    "component": "api-server",
    "name":      "prod us-east-1"
  }'
```

Response:

```json
{
  "id":            42,
  "project_id":    7,
  "component":     "api-server",
  "name":          "prod us-east-1",
  "token_prefix":  "zdxk_8f2d9a1b",
  "created_at":    "2026-04-14T00:00:00Z",
  "token":         "zdxk_8f2d9a1b6c..."
}
```

The `token` field is shown **once**. Store it; it cannot be recovered. The
`token_prefix` is safe to log and display — it's how you'll identify the
token in the dashboard. To rotate or revoke:

```bash
# Revoke (sets revoked_at; token stops working immediately)
curl -X POST https://zdx.example.com/api/admin/integration-tokens/42/revoke \
  -H "X-Api-Key: $ADMIN_KEY"

# Delete (removes the record entirely)
curl -X DELETE https://zdx.example.com/api/admin/integration-tokens/42 \
  -H "X-Api-Key: $ADMIN_KEY"
```

## Ingest timings

`POST /api/ingest/timings`

- **Auth**: `Authorization: Bearer <token>`. No `X-Api-Key`.
- **Body**: JSON batch of events. Every field except `events` is optional.
- **Rate limit**: per-token token bucket; over-limit returns `429`.

```bash
curl -X POST https://zdx.example.com/api/ingest/timings \
  -H "Authorization: Bearer $ZDX_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "component":   "api-server",
    "environment": "prod",
    "host":        "web01.us-east-1",
    "events": [
      {"name": "db:users.lookup", "duration_ms": 4,  "source": "/api/user/42"},
      {"name": "http:GET /api/user/42", "duration_ms": 31, "source": "/api/user/42"},
      {"name": "cache:miss", "duration_ms": 12, "tags": {"key": "user:42"}}
    ]
  }'
```

Response:

```json
{"accepted": 3}
```

Per-event `component` / `environment` fall back to the batch-level value,
and the batch-level falls back to the token's default. `host` is folded
into `context_json` along with any `tags`.

## Request schema

### Batch

| Field         | Type                  | Notes                                            |
|---------------|-----------------------|--------------------------------------------------|
| `component`   | string, optional      | Overrides the token's default for every event.   |
| `environment` | string, optional      | Free-text; commonly `prod`, `staging`, a slot.   |
| `host`        | string, optional      | Instance identifier. Stored in `context_json`.   |
| `events`      | array of Event        | Required. Empty array is a valid no-op.          |

### Event

| Field         | Type                  | Notes                                            |
|---------------|-----------------------|--------------------------------------------------|
| `name`        | string, required      | The metric name. Stable identifier for aggregation. Examples: `sql:GetUser`, `http:GET /api/x`, `job:nightly-rollup`. |
| `duration_ms` | int32, required       | Milliseconds. Sub-millisecond work should be skipped client-side or reported as 1. |
| `source`      | string, optional      | Where the work originated — usually a request path or job label. Kept on the slowest sample. |
| `tags`        | object, optional      | Flat map of string → string. Merged into `context_json`. |

## Semantics

Each event upserts against the unique key
`(project_id, component, environment, name)`. The stored row tracks:

- `duration_ms` — the max observed (slowest sample).
- `count` — total samples.
- `total_ms` — sum of all samples; `avg = total_ms / count`.
- `source`, `context_json` — overwritten from the slowest sample only.

That means one row per metric per component+env, with aggregate stats.
It's deliberately not a time series — zdx is designed around "here's the
slow thing" not "here are its percentiles over time."

## Batching guidance

- **Client-side buffer**: aggregate events in memory, flush on a timer or
  when the batch reaches ~500 events. Don't POST per event.
- **Overflow**: drop rather than block. Timing emission must never stall
  your hot path.
- **Retries**: 5xx and 429 are retriable with exponential backoff; 4xx
  other than 429 indicates a configuration problem (bad token, malformed
  body). Don't retry those.
- **Shutdown**: flush the buffer on process exit so the last few seconds
  of data aren't lost.

## Go client

A reference client lives at `github.com/iodesystems/zdx-go/pkg/zdxclient`.
It implements the batching, retries, and shutdown-flush above.

```go
import "github.com/iodesystems/zdx-go/pkg/zdxclient"

c, err := zdxclient.New(zdxclient.Config{
    Endpoint:    "https://zdx.example.com",
    Token:       os.Getenv("ZDX_TOKEN"),
    Component:   "api-server",
    Environment: "prod",
    Host:        mustHostname(),
})
if err != nil { log.Fatal(err) }
defer c.Close(context.Background())

c.Record("db:users.lookup", 4*time.Millisecond, nil)
c.RecordWithSource("http:GET /api/user/42", "/api/user/42", 31*time.Millisecond, nil)
```

## Non-Go integrations

The wire format above is the whole contract; any HTTP client can integrate.
The full OpenAPI spec is served live at `/openapi.json` and `/openapi.yaml`.
Key behaviors to replicate:

1. Buffer events in memory; POST in batches of ≤500.
2. Flush on a timer (5–10s works well) and at shutdown.
3. Authorization header is `Bearer <token>`. Never `X-Api-Key`.
4. On `429`, back off; on `401`/`403`, stop retrying and surface the error.
5. Drop on local overflow — timing emission is best-effort.

If you write one, consider upstreaming it alongside `pkg/zdxclient`.
