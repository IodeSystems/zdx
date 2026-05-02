# route-extract

Parses `routeTree.gen.ts` (TanStack Router generated file) and emits a JSON array of canonical route records.

## Usage

```
go run ./cmd/route-extract [--routeTree ui/src/routeTree.gen.ts] [--out <file>]
```

Omit `--out` to write to stdout.

## Output schema

```json
[{"pattern": "/project/$slug/issues/$id", "file": "ui/src/routes/project/$slug/issues/$id.tsx", "dynamic": true}]
```

## Gotchas found during spike (against zdx 60+ route surface)

**`$` params — keep TanStack literal.**
Patterns like `/project/$slug/issues/$id` use `$param` notation. Do not normalize to `:param`.
`dynamic` is set when the pattern contains any `$`.

**Index routes — trailing slash preserved from `FileRoutesByPath` key.**
`/admin/` and `/project/$slug/issues/` both appear as-is; the key in the generated interface IS the canonical pattern.

**Layout files (e.g. `tests.tsx`) and their index (`tests/index.tsx`) are separate entries.**
`/project/$slug/tests` maps to `tests.tsx`; `/project/$slug/tests/` maps to `tests/index.tsx`.
Both are real routes in TanStack Router — the parent layout and its index child coexist.

**Co-located test files are not routes.**
`find ui/src/routes -name '*.tsx' -not -name '__root.tsx'` returns 67 files; extractor emits 64 routes.
The 3-file delta is `.test.tsx` co-located specs (e.g. `-blocker-questions.test.tsx`).
TanStack Router ignores files with a leading `-` segment.

**Route groups (parenthesized dirs) — none currently in zdx.**
TanStack Router supports `(group)/route.tsx` where `(group)` is omitted from the URL.
No instances exist in zdx today; would appear in `FileRoutesByPath` keys without the group segment.

**`__root.tsx` is excluded.**
The root layout is not a routable path and has no `FileRoutesByPath` entry.
