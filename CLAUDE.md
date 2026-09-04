# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Argus is a Go microservice for centralized, tamper-evident audit logging. It exposes a REST API and a
reusable client library (`pkg/audit`) that other Go services import to emit audit events. It's designed
to be the audit source of truth for any microservice platform.

## Commands

```bash
# Build the service binary
go build -o service_binary ./cmd/argus

# Run the service locally (defaults: in-memory SQLite, port 3001)
go run ./cmd/argus

# Run all tests
go test ./...

# Run tests for a single package
go test ./internal/pipeline/...

# Run a single test
go test ./internal/api/v1/services/ -run TestAuditService_CreateLog

# Verbose / with coverage
go test -v -cover ./...

# Vet / format
go vet ./...
gofmt -l .
```

There is no Makefile or golangci-lint config in the repo — use the `go` toolchain directly. There is
no top-level CI workflow for Go build/test/lint (only Helm chart CI exists under `.github/workflows/`),
so local `go build`/`go vet`/`go test ./...` is the only verification signal before committing.

Local Postgres for manual testing: `docker-compose up` (brings up `argus` + a `postgres:15-alpine`
sidecar). By default (no `DB_TYPE`/`DB_PATH` set) the service uses an in-memory SQLite DB.

## Architecture

### Two audiences, two packages

- **`internal/`** — the Argus *service* itself (API, DB, pipeline). Not importable by other projects.
- **`pkg/audit`** — the *client library* other Go services import (`go get github.com/LSFLK/argus/pkg/audit@v1.0.0`). Always pin a tagged release; do not use commit pseudo-versions.
  to send audit events to a running Argus instance. This is a separate logical module from the service;
  don't leak service-internal types into it, and don't assume service-side dependencies (GORM, sinks) are
  available here. `pkg/audit/security.go` implements client-side request signing (RSA/Ed25519) that mirrors
  server-side verification in `internal/api/v1/services/security.go` — the two must stay in lock-step, since
  they both compute the same canonical byte representation of a request (see `CanonicalizeRequest`,
  pipe-delimited, not JSON, precisely so non-Go clients can reproduce it too).

### Request flow (service side)

```
cmd/argus/main.go (wiring only)
  → internal/middleware (Metrics → CORS → Auth, outer to inner)
  → internal/api/v1/handlers (HTTP decode/encode)
  → internal/api/v1/services (validation, signature verification, business rules)
  → internal/pipeline.Manager (fan-out dispatch)
        → internal/pipeline/sinks/* (PostgresSink, ConsoleSink, S3Sink...)
  → internal/api/v1/database (GormReader, read path only — queries bypass the pipeline)
```

`cmd/argus/main.go` is intentionally just composition root wiring (flags, env, sink construction,
route registration, graceful shutdown) — business logic does not belong there.

### Pipeline / Sink pattern (`internal/pipeline`)

- `Manager` fans a single write out to *all* registered `Sink`s concurrently (`Dispatch`/`DispatchBatch`),
  and separately supports async, backpressured dispatch (`DispatchAsync`/`DispatchBatchAsync`) via a
  bounded queue + worker pool, used so HTTP handlers aren't blocked on slow sinks.
- Adding a new sink = implement the `Sink` interface (`internal/pipeline/sinks/interface.go`): `Name`,
  `Write`, `WriteBatch`, `Close`, `IsCritical`. `IsCritical` sinks whose writes fail should fail the
  overall request (checked via `Manager.HasCriticalFailure`); non-critical sinks (e.g. `ConsoleSink`)
  are best-effort.
- **Context contract**: every `Sink.Write`/`WriteBatch` MUST respect `ctx.Done()` and return promptly.
  `Manager.Dispatch` relies on this to avoid goroutine leaks — a sink using a non-context-aware SDK
  needs a context-aware adapter wrapped around it (see the comment on `Manager.Dispatch`).
- Async dispatch applies backpressure (blocks up to 5s) rather than silently dropping logs when the
  queue is full — callers should surface a 503 to the caller on that error, not swallow it.

### Non-repudiation / tamper-evidence (the core security property)

- Every `AuditLog` row carries `PreviousHash`/`CurrentHash`, chained **per `ActorID`** (not globally) —
  this is what avoids global lock contention while still making tampering with any single actor's
  history detectable. A unique composite DB index prevents forked chains. Don't "fix" a slow write by
  removing per-actor partitioning without understanding this tradeoff.
- Optional request-level signing: if a request carries `PublicKeyID`/`Signature`, the server verifies
  it against a `PublicKeyRegistry` (`internal/api/v1/services/security.go`) before persisting. The hash/
  signature covers the full canonicalized payload (all metadata + message bytes), not just a subset —
  changes to `CanonicalizeRequest` are a breaking/security-relevant change and must be mirrored in
  `pkg/audit/security.go`.
- Auth middleware (`internal/middleware/auth.go`) uses `crypto/subtle.ConstantTimeCompare` over a
  SHA-256 pre-hash of the API key specifically to avoid length-based timing side channels — don't
  replace this with a plain `==` comparison.

### Database

- GORM-based, supports SQLite and PostgreSQL through the same code path (`internal/database`,
  `internal/api/v1/database`). Selection is env-driven (`DB_TYPE`/`DB_PATH`/`DB_HOST` — see
  `NewDatabaseConfig` in `internal/database/client.go` for exact precedence): no `DB_TYPE`/`DB_PATH` →
  in-memory SQLite; `DB_PATH` set or `DB_TYPE=sqlite` → file-based SQLite; `DB_TYPE=postgres` → Postgres.
  Write path uses GORM's `CreateInBatches` for high-throughput ingestion via `PostgresSink`; read path
  goes through `GormReader`, independent of the sink/pipeline abstraction.

### Config-driven enums

Valid `EventType`/`EventAction`/`ActorType`/`TargetType` values are loaded at startup from
`configs/enums.yaml` (path overridable via `AUDIT_ENUMS_CONFIG`) into `internal/config.AuditEnums`,
then pushed into `internal/api/v1/models` via `SetEnumConfig` for O(1) validation. Falls back to
`GetDefaultEnums()` if the file is missing/invalid. When adding a new enum value, update the YAML
(or the defaults), not a hardcoded switch statement in the models/validation code.

### Versioning

The API is versioned by Go package path (`internal/api/v1/...`), not just by URL prefix — a `v2` would
live alongside `v1` as a new package tree, mirroring the same handlers/services/models/database layers.
