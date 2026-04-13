---
name: Domain logic architecture direction
description: Business logic should live in one place with CLI and API as thin consumers over DomainAdapter
type: project
---

CLI commands should map 1:1 to REST endpoints. Business logic (todo derivation, triage rules, journal triggers, solo queue) defined once, data access abstracted via DomainAdapter. CLI and serve API are both thin consumers.

**Why:** Currently todo/issue/task logic is in context.zig with YAML manipulation inlined. DomainAdapter intercepts at edges but business rules are CLI-only. Remote mode should be remote-driven, not local-with-sync.

**How to apply:** When refactoring todo commands, extract business logic into a shared domain layer. FsAdapter (local YAML), HttpAdapter (remote API), and serve endpoints all call the same logic. Import endpoint should gain overwrite/merge mode param. Don't duplicate business rules across CLI and serve.
