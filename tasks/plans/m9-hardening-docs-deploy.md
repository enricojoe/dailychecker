# M9 — Hardening, Docs & Deployment

> Execution plan for Milestone 9 (final). Parent plan: [dailychecker.md](dailychecker.md) §Milestone 9.
> **Scope decisions (user):** Dockerfiles YES, CI NO. Telegram webhook mode: **implement fully** (env-selectable), keep long-poll as the dev default.

## Part A — Backend hardening + webhook (→ go-gin-backend-architect subagent)
### A1. CORS (functional gap — the SPA needs it)
- The frontend (`http://localhost:5173`, configurable) calls the API cross-origin; the router currently has no CORS. Add CORS middleware allowing the app origin(s) from config (new `CORS_ALLOWED_ORIGINS`, comma-separated; default `http://localhost:5173`), methods GET/POST/PATCH/DELETE/OPTIONS, headers incl. `Authorization, Content-Type`, and handle preflight `OPTIONS`. Don't use `*` together with credentials.

### A2. Consistent errors / validation / logging
- Audit handlers: every error path returns the `{ "error": msg }` envelope with the right status (already mostly done — fix any that leak raw text or wrong codes). Ensure request-body bind failures → 422 (or 400) with a clean message, never a 500.
- Request logging: keep `gin.Recovery()`; ensure a structured-ish request log (method, path, status, latency). `gin.Logger()` is acceptable; optionally switch to a small custom middleware. Don't log secrets/tokens/Authorization headers.
- Confirm panics are recovered → 500 `{error}` (Recovery is present; verify the body shape or add a custom recovery that emits the envelope).

### A3. Telegram webhook mode (env-selectable; reuse existing dispatch)
- New config: `TELEGRAM_MODE` = `polling` (default) | `webhook`; `TELEGRAM_WEBHOOK_URL` (public base URL, e.g. `https://example.com`); `TELEGRAM_WEBHOOK_SECRET` (validates incoming updates).
- Webhook HTTP endpoint `POST /api/telegram/webhook` (public route, registered only when mode=webhook + token set): validate the `X-Telegram-Bot-Api-Secret-Token` header == `TELEGRAM_WEBHOOK_SECRET` (reject 401/403 otherwise), decode an `Update`, dispatch via the SAME `ParseStartToken` + `svc.HandleStart` already used by the poller (refactor the shared dispatch into a reusable method, e.g. `Service.HandleUpdate(ctx, Update)` or reuse poller's logic). Always respond 200 quickly so Telegram doesn't retry storm.
- `main.go`: in `polling` mode start the poller (current behavior); in `webhook` mode DO NOT start the poller — instead call Telegram `setWebhook(url + "/api/telegram/webhook", secret)` once on startup (add a small client method) and register the webhook route. Server must still boot with no bot token (telegram fully disabled, neither mode active). Add a telegram client method `SetWebhook(ctx, url, secret) error` (and optionally `DeleteWebhook`).
- Tests (mocked, no network): webhook handler with correct secret + a `/start <token>` update → calls HandleStart (assert via existing user/mock pattern); wrong/missing secret → rejected, no dispatch; non-start update → 200 no-op; `SetWebhook` builds the correct request against an httptest.Server. Keep deterministic + parallel-safe (unique users, no shared-table teardown).

### A4. Verification (orchestrator runs; subagent can run build/vet/test locally)
- `go build ./...`, `go vet ./...` clean; default-parallel `go test ./...` green (×2). Server boots in all three telegram states: disabled (no token), polling, webhook (webhook setWebhook call can fail gracefully without a real token — guard so boot doesn't crash; document).

## Part B — Dockerfiles + full-stack compose (→ orchestrator; needs `docker build`)
- `backend/Dockerfile`: multi-stage (golang build → minimal runtime, e.g. distroless/alpine); copies the `migrations/` dir (runner needs the .sql files at runtime); exposes `PORT`; runs the server. Non-root user. Build context handles go modules cache.
- `frontend/Dockerfile`: multi-stage (node build `npm ci && npm run build` → nginx serving `dist/` with SPA fallback `try_files ... /index.html`). `VITE_API_BASE_URL` is a build-time arg.
- Extend `docker-compose.yml` (or add `docker-compose.full.yml`) with `backend` + `frontend` services wired to `db`, env via `backend/.env`, healthchecks, depends_on. Keep the existing dev `db` service usable standalone (don't break `make db-up`).
- `.dockerignore` for both (node_modules, .git, bin, etc.).
- Makefile targets: `docker-build`, `docker-up`, `docker-down` (full stack).
- Verify: `docker build` both images succeed; optionally `docker compose up` smoke (healthz + frontend serves index).

## Part C — README + docs (→ orchestrator; needs whole-system knowledge)
- Root `README.md`: project overview; architecture diagram (reuse plan §2); prerequisites; **local dev setup** (clone → `make db-up` (port 5433) → `backend/.env` from example → `make run` → frontend `npm i && npm run dev`); **env var reference** (backend + frontend, incl. new CORS_* and TELEGRAM_* webhook vars); **Telegram bot setup** (BotFather steps + the live-DM verification from M5; polling vs webhook mode + how to switch); **Docker** (build/run full stack); running tests; project layout; milestone status. No secrets committed.
- Update `backend/.env.example` with the new vars (CORS_ALLOWED_ORIGINS, TELEGRAM_MODE, TELEGRAM_WEBHOOK_URL, TELEGRAM_WEBHOOK_SECRET).

## DoD (parent plan)
Fresh clone → follow README → working app locally; tests green. (CI is out of scope per user. Live Telegram DM remains a manual step needing a real token.)

## Constraints
- Backend: clean architecture, idiomatic Go, match existing style; reuse telegram dispatch (don't duplicate). No secrets in logs. Tests deterministic under default parallel `go test ./...`; dev DB on 5433.
- Don't break existing milestones (all current tests must stay green; `make db-up`/`make run` still work).
- Commit only after orchestrator verifies each part.

## Result
_TBD_
