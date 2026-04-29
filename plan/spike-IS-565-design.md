# Spike Design: Web App + Page Schema, CLI, Solo Queue, Doctor

**Issue:** IS-565 — Extract TanStack Router sitemap and register pages for periodic review  
**PoC reference:** `plan/spike-IS-565-routes.json` (60 patterns extracted from `ui/src/routeTree.gen.ts`)  
**Date:** 2026-04-29

---

## 1. Schema

Two new tables hang off `zdx_projects`.

### `zdx_web_apps`

```sql
CREATE TABLE zdx_web_apps (
    id           BIGSERIAL    PRIMARY KEY,
    project_id   TEXT         NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    name         TEXT         NOT NULL,       -- e.g. "ui", "admin"
    base_url     TEXT         NOT NULL,       -- e.g. "https://zdx.iodesystems.com"
    routes_path  TEXT         NOT NULL,       -- relative path to routeTree.gen.ts
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, name)
);
```

### `zdx_pages`

```sql
CREATE TABLE zdx_pages (
    id                    BIGSERIAL    PRIMARY KEY,
    web_app_id            BIGINT       NOT NULL REFERENCES zdx_web_apps(id) ON DELETE CASCADE,
    route_pattern         TEXT         NOT NULL,   -- e.g. "/project/$slug/issues/$id"
    file_path             TEXT         NOT NULL,   -- e.g. "ui/src/routes/project/$slug/issues/$id.tsx"
    dynamic               BOOLEAN      NOT NULL DEFAULT FALSE,
    last_reviewed_at      TIMESTAMPTZ,
    review_threshold_days INT          NOT NULL DEFAULT 90,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (web_app_id, route_pattern)
);
```

**Dynamic routes — once-per-pattern for v1.** `/project/$slug/issues/$id` is one row covering all
issue detail pages. The review intent is: does this *type* of page work correctly — not this specific
instance. Per-instance review would require data enumeration (crawling or a data-driven expansion);
out of scope for v1.

**Trailing-slash normalization.** TanStack Router's `FileRoutesByPath` emits both
`/project/$slug/tests` (layout shell, file `tests.tsx`) and `/project/$slug/tests/` (index within
that layout, file `tests/index.tsx`). During sync, `route_pattern = strings.TrimSuffix(fullPath, "/")`
with the unique constraint handling collisions. If a layout shell and its index are both emitted, the
index (trailing-slash form after stripping) survives as the canonical page.

Exception: `/` (root index) must not be stripped to empty string — leave the root path as-is.

---

## 2. Extraction Wiring

**Decision: Option A — `dx webapp sync` CLI parses `routeTree.gen.ts` on demand.**

Option B (vite plugin / build hook) couples extraction to the build pipeline, requires npm
dependencies calling the zdx API, and makes the sync implicit. The CLI approach is explicit,
testable, and matches how zdx handles other sync operations. A CI step can run
`dx webapp sync` post-build as a single line.

**Extraction algorithm** against `FileRoutesByPath`:

```
regex: fullPath: '([^']+)'
```

Applied to the `declare module '@tanstack/react-router' { interface FileRoutesByPath { ... } }` block
(lines 751–1175 in the current file). Each match yields a `fullPath`; the corresponding
`preLoaderRoute: typeof XRouteImport` line gives the import alias, from which the file path is
inferred by scanning the `import { Route as X } from './routes/...'` header block.

The PoC (`plan/spike-IS-565-routes.json`) extracted 60 patterns from zdx's 1401-line
`routeTree.gen.ts`: 10 static, 50 dynamic.

**Static routes (10):**
`/`, `/code`, `/cli-auth`, `/admin`, `/admin/websocket`, `/admin/users`,
`/admin/projects`, `/admin/llm`, `/admin/invites`, `/admin/activity`

**Dynamic examples:**
`/project/$slug`, `/project/$slug/issues/$id`, `/project/$slug/agents/$sessionId`,
`/project/$slug/todos/$key`, `/project/$slug/specs/$specId`

**Gotchas from the PoC:**

- Files prefixed `-` (e.g., `-blocker-questions.test.tsx`) are co-located test files ignored by
  TanStack Router. They do not appear in `routeTree.gen.ts`. The `FileRoutesByPath` approach
  automatically excludes them — no special handling needed.
- Layout routes (e.g., `$slug.tsx`) appear in `FileRoutesByPath` when they have a `path` property.
  V1 includes them; if a layout route turns out to be a pure `<Outlet>` shell with no reviewable UI,
  the reviewer can mark it reviewed once and forget it.
- Index routes (trailing `/`) and their layout siblings both appear. Strip trailing slash before
  upsert; the unique constraint silently wins on collision.

---

## 3. Sitemap.xml

**Mechanism:** `dx webapp sync --emit-sitemap [--sitemap-out=public/sitemap.xml]`

After upserting pages, iterate over `dynamic = FALSE` rows for the web app and emit a standard
`sitemap.xml` to the output path.

**Static routes only.** Dynamic routes (`$param`) cannot be expanded without data enumeration.
For zdx itself, the 10 static routes → 10 `<url>` entries.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://zdx.iodesystems.com/</loc></url>
  <url><loc>https://zdx.iodesystems.com/code</loc></url>
  <url><loc>https://zdx.iodesystems.com/admin</loc></url>
  ...
</urlset>
```

Note: `/admin/*` routes are typically not public. V1 includes all static routes; operators can
add a `public` boolean column to `zdx_pages` in a follow-up to filter the sitemap. Alternatively,
a `--exclude=^/admin` flag on `sync` handles the common case without a schema change.

---

## 4. CLI Surface

All commands are project-scoped (project inferred from `.zdx/config.yaml`).

```
dx webapp add --name=ui --base-url=https://zdx.iodesystems.com \
              --routes-path=ui/src/routeTree.gen.ts
    Registers a zdx_web_apps row. Does NOT sync pages automatically.

dx webapp list
    Table: id | name | base_url | routes_path | pages | overdue

dx webapp sync [--name=ui] [--emit-sitemap] [--sitemap-out=public/sitemap.xml]
    Parses routes_path, upserts zdx_pages rows.
    INSERT ... ON CONFLICT (web_app_id, route_pattern) DO UPDATE SET file_path, dynamic, updated_at
    Prints: N added, M updated, K unchanged.

dx page list [--webapp=ui] [--overdue]
    Table: id | route_pattern | dynamic | last_reviewed_at | days_since | overdue

dx page review <id> [--note="LGTM"]
    Sets last_reviewed_at = NOW(). Prints: reviewed /project/$slug/issues/$id
```

The `--name` flag on `sync` and `--webapp` flag on `page list` accept the `zdx_web_apps.name` value.
Omitting them operates across all web apps for the project.

---

## 5. Solo Queue Integration

Add a candidate block in `generateSoloQueue` (`internal/server/handlers/handlers_solo.go`).

**Trigger condition (SQL):**
```sql
SELECT p.*
FROM zdx_pages p
JOIN zdx_web_apps w ON w.id = p.web_app_id
WHERE w.project_id = $1
  AND (p.last_reviewed_at IS NULL
       OR p.last_reviewed_at < NOW() - (p.review_threshold_days * INTERVAL '1 day'))
ORDER BY p.last_reviewed_at ASC NULLS FIRST
```

**Candidate shape:**
```go
soloCandidate{
    Key:        fmt.Sprintf("page-review-%d", page.ID),
    Title:      fmt.Sprintf("Review page: %s", page.RoutePattern),
    Kind:       "review:page",
    TargetType: "page",
    TargetID:   strconv.Itoa(int(page.ID)),
    Priority:   350,
    Text: fmt.Sprintf(
        "Route: %s\nFile: %s\nLast reviewed: %s\n\n"+
        "Navigate to the page and verify it renders correctly.\n"+
        "Then: dx page review %d",
        page.RoutePattern, page.FilePath, nullableTime(page.LastReviewedAt), page.ID,
    ),
}
```

**Priority band:** 350 — below active dev tasks (100–300), above low-priority housekeeping (400+).
Pages overdue by more than 2× threshold → lower priority to 320 (more urgent).

**De-dup key:** `page-review-<id>` — stable per page row, survives queue re-evaluations.

**Persona:** unset (any agent or human can complete a page review).

**New sqlc query needed:** `ListOverduePages(ctx, projectID int32) ([]ZdxPage, error)` in
`queries/pages.sql`.

---

## 6. Doctor Rung

Applies to `saas` and `site` classifications (add to `saasRungs()` and `siteRungs()` in
`internal/doctor/vines.go`):

```go
{
    Name:        "web-apps",
    Description: "Web app pages are registered and periodically reviewed",
    Checks: []Check{
        {"has_web_apps",     "At least one web app registered",  ActionPropose},
        {"no_overdue_pages", "No pages overdue for review",      ActionInfo},
    },
},
```

**`ProjectState` additions** (`internal/doctor/doctor.go`):
```go
WebAppCount  int
OverduePages int
```

**Check function sketch** (new file `internal/doctor/webapp.go` or inline in `doctor.go`):
```go
func checkWebApps(state *ProjectState, rung string) []Finding {
    if !isWebClassification(state.Classification) {
        return nil
    }
    if state.WebAppCount == 0 {
        return []Finding{{
            Check:    Check{"has_web_apps", "At least one web app registered", ActionPropose},
            Rung:     rung,
            Status:   StatusFail,
            Message:  "No web apps registered.",
            Proposal: "dx webapp add --name=ui --base-url=<URL> --routes-path=ui/src/routeTree.gen.ts",
        }}
    }
    if state.OverduePages > 0 {
        return []Finding{{
            Check:   Check{"no_overdue_pages", "No pages overdue for review", ActionInfo},
            Rung:    rung,
            Status:  StatusFail,
            Message: fmt.Sprintf("%d page(s) overdue for review. Run: dx page list --overdue", state.OverduePages),
        }}
    }
    return []Finding{
        {Check: Check{"has_web_apps", ...}, Rung: rung, Status: StatusPass},
        {Check: Check{"no_overdue_pages", ...}, Rung: rung, Status: StatusPass},
    }
}
```

Populate `WebAppCount` and `OverduePages` in `DetectRemote` via two new lightweight API endpoints
or by expanding the existing project-state response.

---

## 7. Effort Estimate

| Piece             | What's involved                                                                 | Size |
|-------------------|---------------------------------------------------------------------------------|------|
| Schema            | 2 migrations, sqlc types + queries (`make build` + `sqlc generate`)             | S    |
| Extraction        | `cmd/route-extract` Go binary, regex on FileRoutesByPath, trailing-slash logic  | S    |
| API endpoints     | POST/GET web_apps, GET/PATCH pages — 4 huma handlers, OpenAPI → regen clients   | M    |
| CLI — webapp      | `dx webapp add/list/sync` — cobra commands, dxclient typed calls                | M    |
| CLI — page        | `dx page list/review` — 2 commands, 1 query each                                | S    |
| Solo queue        | 1 sqlc query + 1 candidate block in `generateSoloQueue`                         | S    |
| Doctor rung       | 2 check structs, 1 detect function, 2 ProjectState fields                       | S    |
| Sitemap emit      | ~50 LOC, XML template, static-route filter                                      | XS   |
| e2e demo test     | Happy path: add → sync → list → review → solo candidate present/absent          | S    |
| **Total**         |                                                                                 | **M** |

**Wall-clock estimate:** 2–3 focused sessions (~6–10 hours) for a developer familiar with
the codebase's huma/cobra/sqlc patterns.

---

### Recommendation: Proceed

Route extraction from `routeTree.gen.ts` via `FileRoutesByPath` is reliable — the PoC confirms
clean extraction of 60 patterns with no ambiguous cases in zdx's own codebase. The schema is
minimal (2 tables, straightforward foreign key chain). The CLI surface maps directly onto existing
dxclient/cobra patterns already established for every other entity in the system. Solo queue
integration is a single candidate block and one sqlc query.

Most importantly: zdx itself is the first consumer. It's classified `saas`, has ~60 pages, and
zero have ever been formally reviewed. This feature is immediately self-applicable.

**Pivot triggers:** none identified — no novel technical risk.

**Drop triggers:** If `dx page review` commands never get issued (fill rate of `last_reviewed_at`
stays near 0% after 30 days), the feature adds schema weight with no workflow benefit. Monitor
via a doctor check on fill rate; drop or simplify if the queue candidate is consistently snoozed.
