# M5 — Telegram Integration (linking + sending)

> Execution plan for Milestone 5. Parent plan: [dailychecker.md](dailychecker.md) §Milestone 5, §4 (API).
> **Live-delivery verification (real BotFather token + real DM) is deferred** — code is built and fully tested against a MOCKED Telegram API now; the orchestrator/user runs the live check later.

## Goal
Let a logged-in user connect their Telegram account via a deep-link, capture their `chat_id` from a `/start <token>` message, and provide a reusable, rate-limit-safe message-send helper (behind an interface so M6's scheduler reuses it). One-way only (no two-way command handling in v1), but keep the update handler extensible.

## Existing building blocks (committed — reuse, don't alter schemas)
- `users.User` has `TelegramChatID *int64`, `TelegramLinkToken *string`, `TelegramLinkedAt *time.Time`. `users.Repository.Update` already persists these three fields. Need a lookup by link token → **add `GetByLinkToken(ctx, token)` to the users repo** (read-only).
- `config.Config`: `TelegramBotToken`, `TelegramBotUsername` (for `t.me/<username>?start=<token>`), `AppPublicURL` (link back to web app from the bot).
- `auth.RequireAuth` middleware (user id in ctx via `auth.ContextKeyUserID`); `httpapi.respondError` + `{error}` envelope; DI'd `NewRouter` (currently `NewRouter(authSvc, actSvc, occSvc, jwtSecret)`); `cmd/server/main.go` constructs repos/services.
- `internal/telegram/doc.go` is a stub — implement the package here.

## Design
1. **Client interface (mockable):** `telegram.Client` interface with at least `SendMessage(ctx, chatID int64, text string) error`. Real impl `httpClient` calls `https://api.telegram.org/bot<token>/sendMessage` via an injected `*http.Client` (so tests inject a stub transport — NO real network in tests). Basic rate-limit safety (e.g. simple min-interval throttle or honor HTTP 429 `retry_after`; keep it simple, document it).
2. **Link-token issuance:** `telegram.Service.IssueLink(ctx, userID)` → generate a cryptographically random one-time token, store it on the user via `Update` (set `TelegramLinkToken`), return `{ url: "https://t.me/<botUsername>?start=<token>", token }`. Issuing again replaces the old token.
3. **Linking (consume token):** `telegram.Service.HandleStart(ctx, token string, chatID int64)` → look up user by link token (`GetByLinkToken`); if found, set `TelegramChatID`, `TelegramLinkedAt=now`, CLEAR `TelegramLinkToken` (single-use), `Update`; send a confirmation DM including `AppPublicURL`. Unknown/empty token → no-op (don't error the whole poll loop).
4. **Update handler (long-poll, dev):** `telegram.Poller` (or method) that calls `getUpdates` with offset, parses `/start <token>` messages, and dispatches to `HandleStart`. Started as a goroutine from `main.go` ONLY if `TelegramBotToken != ""`; otherwise log "telegram disabled (no token)" and skip — server must start fine without a token. Keep it cleanly stoppable (context cancel on shutdown). Webhook mode is M9 — just leave the handler logic reusable.
5. **HTTP route:** `POST /api/telegram/link` (protected) → `IssueLink`, returns the URL + token. Optionally `GET /api/me` already exposes linked state via user telegram fields (confirm it does; no change needed if so).

## Tasks
- [ ] `users` repo: add `GetByLinkToken(ctx, token)` (explicit columns; `ErrNotFound` when no row).
- [ ] `telegram` package: `Client` interface + real HTTP impl (injected `*http.Client`, base URL overridable for tests) + rate-limit safety; `Service` (`IssueLink`, `HandleStart`) over users repo + config + client; long-poll `getUpdates` loop with offset tracking + `/start <token>` parsing; mock client + (optional) stub HTTP transport for tests.
- [ ] `httpapi`: `telegramHandler` + `POST /api/telegram/link` (protected); response DTO; map any sentinels in `error.go`.
- [ ] Wire `telegram.Service` into `NewRouter` + construct in `main.go`; start the poller goroutine conditionally (token present) and stop it on graceful shutdown.
- [ ] Tests (mocked Telegram, no network; deterministic & parallel-safe — unique users, no shared-table teardown):
  - `IssueLink` stores a token and returns a correct `t.me/<username>?start=<token>` URL.
  - `HandleStart` with a valid token sets chat_id + linked_at, clears the token, and triggers a send (assert via mock); re-using the now-cleared token is a no-op.
  - `HandleStart` with unknown/empty token → no-op, no error.
  - `SendMessage` real client: with a stub transport, asserts correct URL/path/JSON body; handles a 429/`retry_after` and a non-200 gracefully.
  - `/start <token>` parsing extracts the token (and ignores non-start messages).
  - `POST /api/telegram/link` over HTTP: protected (401 unauth), returns URL+token for an authed user.

## DoD (parent plan)
A real user can connect their account; backend stores `chat_id`; a test send delivers a DM. (Live delivery deferred; everything else proven via mocked tests. Document the manual BotFather + live-DM steps in the plan Result / README note for later.)

## Constraints
- Clean architecture, SOLID, idiomatic Go matching existing style (sentinels, `%w`, doc comments, explicit SELECT columns). Client behind an interface for M6 reuse + testability.
- DB access only via repos. No real network calls in tests. Server must boot without a bot token.
- **Never log the bot token or link tokens.** Don't touch frontend/scheduler. Don't commit — leave changes for orchestrator review.
- Tests deterministic under DEFAULT parallel `go test ./...`; do not reintroduce the shared-DB race. Dev DB on host port **5433**; use `testhelper`.

## Result
_2026-06-20_ — Completed & verified (changes in working tree, not committed). All code tested against mocked Telegram API only; live-delivery with a real bot token is deferred.

**What was built:**
- `users.Repository` gained `GetByLinkToken(ctx, token)` (explicit SELECT columns, ErrNotFound on miss).
- `internal/telegram/client.go`: `Client` interface + `httpClient` real impl. Base URL injected at construction so tests point at `httptest.Server` — no real network. Rate-limit: 50 ms min-interval throttle + HTTP 429 → `ErrRateLimit` sentinel with `retry_after` preserved. Token never logged.
- `internal/telegram/service.go`: `Service.IssueLink` (32-byte crypto-random token, stored via `Update`, returns `t.me/<username>?start=<token>`); `HandleStart` (looks up by token, sets chat_id + linked_at, clears token single-use, sends confirmation DM; empty/unknown token → no-op).
- `internal/telegram/poller.go`: `Poller.Run` long-poll loop, offset tracking, `/start <token>` parsing, cleanly stopped via `context.Context`. `ParseStartToken` exported for unit tests. `Update`/`Message`/`Chat` types exported.
- `internal/httpapi/telegram_handler.go`: `POST /api/telegram/link` (protected) → `IssueLink` → 200 `{url, token}`.
- `internal/httpapi/router.go`: `NewRouter` gained `tgSvc *telegram.Service` param; telegram route registered only when `tgSvc != nil`.
- `cmd/server/main.go`: constructs `telegram.Client` + `telegram.Service` + `telegram.Poller` only when `TELEGRAM_BOT_TOKEN != ""`; logs "telegram: disabled" otherwise; starts poller goroutine conditionally; cancels poller context on graceful shutdown.
- Tests: 14 M5 tests across 3 files — all pass, no real network, deterministic under default parallel `go test ./...`. `go build ./...` + `go vet ./...` clean. Two consecutive default-parallel runs both green.

**Manual live-DM steps (deferred):** see DoD / Manual verification section above.
