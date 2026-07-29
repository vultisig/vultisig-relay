# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Vultisig Relay: a Go/Echo HTTP relay that routes TSS (Threshold Signature Scheme) keygen/keysign messages between participants, backed by Redis. It is stateless application logic — all session/message state lives in Redis (or in-memory for tests).

## Commands

```bash
go build -o vultisig-relay cmd/router/main.go   # build
go run cmd/router/main.go -config config.json   # run (needs config.json + reachable Redis)
go build ./...                                   # build all packages (CI does this)
go test -v ./...                                 # run tests (CI does this)
go vet ./...
docker-compose up -d                             # start local Redis (set REDIS_PORT env var first)
```

There are currently no `_test.go` files in the repo, so `go test ./...` is a no-op — CI (`.github/workflows/go.yml`) just builds and runs `go test -v ./...` on push/PR to `main`.

`config.json` (not committed) is required at runtime:
```json
{
  "port": 8080,
  "redis_server": { "addr": "localhost:6379", "user": "", "password": "", "db": 0 }
}
```

## Architecture

Four layers, wired together in `cmd/router/main.go`: load config → construct `storage.Storage` → construct `server.Server` → `StartServer()`.

- **`server/handler.go`** — single `Server` struct holds all Echo route handlers as methods. All routes are registered in `StartServer()`; there's no router package split by resource. Handlers are thin: parse/validate params, delegate to `storage.Storage`, map errors to HTTP status codes (no response body on errors — `c.NoContent(...)`).
- **`storage/storage.go`** — defines the `Storage` interface (the seam every handler codes against) and `RedisStorage`, the production implementation. Redis key scheme: sessions/TSS-state are Redis lists (`RPUSH`/`LRange`), payloads/setup-messages/keysign-completion are plain key-value (`SET`/`GET`). Sessions expire after 5 minutes, values (`SetValue`) after 1 hour (`defaultExpiration` / `defaultUserExpire` in `RedisStorage`).
- **`storage/in_memory_storage.go`** — an in-memory `Storage` implementation (go-cache based), used where a real Redis isn't available/desired. Note: it does **not** exactly replicate every Redis-specific edge case (e.g. Redis rejects zero-arg `RPUSH`; the in-memory version has no such constraint), so a bug reproducible against `RedisStorage` may not reproduce here.
- **`model/message.go`** — `Message` and `Session` structs, the wire/storage format.
- **`config/config.go`** — flat JSON config loader.
- **`contexthelper/context.go`** — `CheckCancellation(ctx)` helper called at the top of most handlers/storage methods to bail out early on a cancelled request context.

### Session/key naming convention

Several distinct "kinds" of session share the same underlying `SetSession`/`GetSession` storage calls, distinguished only by a Redis key prefix built with `fmt.Sprintf`:
- plain session: key = `sessionID`
- TSS start: key = `start-{sessionID}` (via `handleTSSSession`/`getTSSSession` helpers)
- TSS complete: key = `complete-{sessionID}`
- keysign finished flag: key = `keysign-{sessionID}-{messageID}-complete` (value store, not list)
- payload: key = the SHA-256 hash of the payload body itself (content-addressed; `HandlePayloadMessage`/`GetPayloadMessage` verify the hash matches on both write and read)
- setup message: key = `setup-{sessionID}` or `setup-{sessionID}-{messageID}` if a `message-id` header is present
- per-participant messages: key = `{sessionID}-{participantID}` or `{sessionID}-{participantID}-{messageID}` if a `message_id` header is present

When adding a new session-like concept, follow this prefix convention rather than introducing a new storage method, unless the access pattern genuinely differs (list vs. single value).

**Note:** message endpoints (`PostMessage`/`GetMessage`/`DeleteMessage`) read the header as `message_id` (underscore), while setup-message endpoints read `message-id` (hyphen) — this inconsistency is real, not a typo to silently "fix" without checking both client and server usage.

### `SetSession` semantics

`SetSession` (used by plain sessions, TSS start, and TSS complete) diffs the posted participant list against what's already stored and only `RPUSH`es the new ones — it's additive/idempotent, not a replace. An empty diff (all posted participants already registered) must return success as a no-op rather than issuing a zero-element `RPUSH`, which Redis rejects.
