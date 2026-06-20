# DailyChecker

A multi-user daily activity tracker with nested sub-activities, per-activity
scheduling, completion history, and Telegram reminders.

Define recurring **activities** (e.g. _"Morning routine — every day at 06:00"_),
optionally with **sub-activities**. Each day an activity is due it produces a
checkable **occurrence** you mark `pending`, `partial`, or `done`. Completion is
recorded per day so you can browse your **history** (calendar + per-activity).
A shared **Telegram bot** sends per-activity reminders at each activity's time
plus a nightly digest of everything still not done.

## Stack

| Layer | Tech |
|-------|------|
| Backend | Go + Gin + sqlx, PostgreSQL, golang-migrate, robfig/cron, JWT (golang-jwt) + bcrypt |
| Frontend | React 19 + Vite + TypeScript, TanStack Query, React Router, Tailwind v4 + shadcn/base-ui |
| Scheduling | In-process `robfig/cron` (per-activity reminders + nightly digest) |
| Telegram | Single shared bot; deep-link account linking; long-poll (dev) or webhook (prod) |
| Timezone | Asia/Jakarta (global, v1) |

## Architecture

```
   Browser ──► React + Vite SPA ──REST/JSON (JWT)──► Go + Gin API ──sqlx──► PostgreSQL
                                                        │
                                              scheduler (cron) ──► Telegram Bot API
```

```
backend/
  cmd/server/        # wiring, config, graceful shutdown
  internal/
    config/  db/  auth/  users/  activities/  occurrences/
    rollup/          # pure parent/child state-propagation engine
    telegram/        # bot client, linking, poller + webhook
    scheduler/       # cron: reminders + nightly digest
    httpapi/         # router, middleware (CORS, recovery), handlers, DTOs
  migrations/        # *.up.sql / *.down.sql (run programmatically on boot)
frontend/
  src/api/  src/auth/  src/routes/  src/components/
  src/features/{activities,today,history,telegram}/
docker/             # all container assets
  docker-compose.yml  Dockerfile.backend  Dockerfile.frontend
  nginx.conf  .env.example   # .env (gitignored) holds the compose environment
```

## Prerequisites

- Go 1.26+
- Node 22+
- Docker (for PostgreSQL, and optionally the full stack)

## Local development

```bash
# 1. Start PostgreSQL (host port 5433 — avoids clashing with a local 5432)
make db-up

# 2. Backend
cd backend
cp .env.example .env          # adjust JWT_SECRET etc.
cd .. && make run             # migrations run automatically on boot; serves :8080

# 3. Frontend (separate terminal)
cp frontend/.env.example frontend/.env   # VITE_API_BASE_URL=http://localhost:8080/api
make frontend-install                    # npm install
make frontend                            # npm run dev — serves :5173
```

Open http://localhost:5173 — register an account, then create activities and check
them off on the Today page. The backend already allows the `:5173` origin via
`CORS_ALLOWED_ORIGINS`.

## Running with Docker (full stack)

All container assets live in `docker/`. The compose environment is a single file,
`docker/.env` (copied from `docker/.env.example` automatically by the make targets).

```bash
make docker-up      # builds + starts db + backend + frontend
#   frontend → http://localhost:8081
#   backend  → http://localhost:8080
make docker-down
```

The full stack is gated behind a Compose `full` profile, so plain
`docker compose -f docker/docker-compose.yml up -d` (and `make db-up`) still starts
**only** Postgres for the local dev workflow above.

> Note: the frontend image bakes `VITE_API_BASE_URL` at build time (Vite inlines
> `VITE_*` vars). The bundled default is `http://localhost:8080/api`; change
> `VITE_API_BASE_URL` in `docker/.env` to point at another API origin.

## Environment variables

### `backend/.env`

| Var | Default | Notes |
|-----|---------|-------|
| `APP_ENV` | `development` | |
| `PORT` | `8080` | |
| `DATABASE_URL` | — (required) | Postgres DSN; dev points at `localhost:5433` |
| `JWT_SECRET` | — (required) | Sign/verify access tokens |
| `ACCESS_TOKEN_TTL` | `15m` | |
| `REFRESH_TOKEN_TTL` | `720h` | |
| `TIMEZONE` | `Asia/Jakarta` | |
| `DIGEST_HOUR` | `22` | Nightly digest hour (Jakarta) |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Comma-separated SPA origins |
| `TELEGRAM_BOT_TOKEN` | _(empty)_ | Empty → Telegram fully disabled (app still works) |
| `TELEGRAM_BOT_USERNAME` | _(empty)_ | For building `t.me/<username>?start=<token>` links |
| `TELEGRAM_MODE` | `polling` | `polling` (dev) or `webhook` (prod) |
| `TELEGRAM_WEBHOOK_URL` | _(empty)_ | Public base URL; required in webhook mode |
| `TELEGRAM_WEBHOOK_SECRET` | _(empty)_ | Validated per incoming update in webhook mode |
| `APP_PUBLIC_URL` | `http://localhost:5173` | Link back to the web app from the bot |

### `frontend/.env`

| Var | Default | Notes |
|-----|---------|-------|
| `VITE_API_BASE_URL` | `http://localhost:8080/api` | Only `VITE_*` vars reach the browser — never put secrets here |

`.env` files are git-ignored; `.env.example` files are committed templates.

## Telegram bot setup

The app runs fine **without** a bot (`TELEGRAM_BOT_TOKEN` empty). To enable reminders:

1. In Telegram, message **@BotFather** → `/newbot` → copy the **bot token** and **username**.
2. Set in `backend/.env`:
   ```
   TELEGRAM_BOT_TOKEN=<token>
   TELEGRAM_BOT_USERNAME=<username>   # no @
   APP_PUBLIC_URL=http://localhost:5173
   ```
3. Restart the backend. With `TELEGRAM_MODE=polling` you should see `telegram poller: starting`.
4. In the app: open **Telegram** → **Connect Telegram** → click the deep-link → press
   **Start** in the bot chat. The backend captures your `chat_id`; the page flips to
   _Connected_ and you receive a confirmation DM.

### Polling vs webhook

- **Polling (default):** the server long-polls `getUpdates`. No public URL needed — ideal for dev.
- **Webhook (prod):** set
  ```
  TELEGRAM_MODE=webhook
  TELEGRAM_WEBHOOK_URL=https://your-public-host
  TELEGRAM_WEBHOOK_SECRET=<random-secret>
  ```
  On boot the server registers `https://your-public-host/api/telegram/webhook` with
  Telegram and validates the `X-Telegram-Bot-Api-Secret-Token` header on each update.
  (Requires the server to be reachable over HTTPS at that URL.)

## Tests

```bash
make test            # backend: go test ./...  (needs `make db-up` first)
make frontend-lint   # frontend: tsc --noEmit + eslint
make frontend-build  # frontend: production build
```

Backend integration tests run against the dev Postgres on port 5433 and are safe
under the default parallel `go test ./...`.

## Project status

All milestones M0–M9 are complete. The one manual step that requires your own
infrastructure is the **live Telegram DM** verification (needs a real BotFather
token); everything else — including the bot linking/sending logic — is covered by
automated tests against a mocked Telegram API. See `tasks/plans/dailychecker.md`
for the full milestone log.
