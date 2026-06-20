# DailyChecker — Implementation Plan

> A multi-user daily activity tracker with nested sub-activities, per-activity
> scheduling, completion history, and Telegram reminders.
>
> **Status:** Planning complete — ready to start Milestone 0.
> Update the checkboxes and the "Result" notes as each milestone is delivered.

---

## 1. Product Summary

DailyChecker lets users define recurring **activities** (e.g. "Do the dishes — every
day at 19:00"). Each activity may contain optional **sub-activities**. Every day an
activity is "due", it produces a checkable **occurrence** that the user marks as
`pending`, `partial`, or `done`. Completion is recorded per day so users can browse
their **history**. A shared **Telegram bot** sends per-activity reminders at each
activity's time, plus a nightly catch-all digest of everything still not done.

### Locked-in Decisions

| Area | Decision |
|------|----------|
| Backend | Go + Gin + sqlx |
| Database | PostgreSQL |
| Frontend | React + Vite + TypeScript, TanStack Query, Tailwind + shadcn/ui |
| Auth | name + phone + password (bcrypt); JWT access token + DB-stored refresh token |
| Timezone | Asia/Jakarta (GMT+7), global for all users (v1) |
| Item states | `pending` \| `partial` \| `done` |
| Parent/child | All children `done` → parent auto `done`; parent manually `done` → all children `done`; `partial` settable manually on any item |
| History | Per-occurrence records (activity + date + state), generated daily |
| Telegram | Single shared bot; users connect via deep-link `/start <token>` to capture `chat_id`; one-way reminders + link to web app |
| Reminders | Per-activity reminder at the activity's scheduled time (fires once) **+** nightly digest at 22:00 (env-configurable) of all not-done items |
| Scheduler | In-process `robfig/cron` minute-tick + dedicated digest job; isolated package for future upgrade |

### Out of Scope (v1)

- Two-way Telegram (checking items off from chat) — design leaves room for it later.
- Phone/OTP verification.
- Per-user timezones (global Jakarta for now).
- Horizontal scaling / multi-instance dedup (single instance for v1).

---

## 2. Architecture Overview

```
                    ┌──────────────────────────┐
   Browser  ─────►  │  React + Vite frontend    │
                    └────────────┬─────────────┘
                                 │ REST/JSON (JWT)
                    ┌────────────▼─────────────┐
                    │   Go + Gin API server     │
                    │  ┌────────────────────┐   │
                    │  │ auth | activities  │   │
                    │  │ occurrences | tg   │   │
                    │  └────────────────────┘   │
                    │  ┌────────────────────┐   │
                    │  │ scheduler (cron)   │───┼──► Telegram Bot API
                    │  └────────────────────┘   │
                    └────────────┬─────────────┘
                                 │ sqlx
                    ┌────────────▼─────────────┐
                    │        PostgreSQL         │
                    └──────────────────────────┘
```

### Backend Package Layout (proposed)

```
backend/
  cmd/server/main.go          # wiring, config, graceful shutdown
  internal/
    config/                   # env loading
    db/                       # sqlx connection, migrations runner
    auth/                     # hashing, JWT, refresh tokens, middleware
    users/                    # user repo + service
    activities/               # activity + sub-activity repo/service/handlers
    occurrences/              # daily occurrence generation, state, rollup, history
    telegram/                 # bot client, deep-link linking, message sending
    scheduler/                # cron ticks: per-activity reminders + nightly digest
    httpapi/                  # router, middleware, error mapping, DTOs
  migrations/                 # *.up.sql / *.down.sql
  go.mod
```

### Frontend Layout (proposed)

```
frontend/
  src/
    api/            # typed API client, TanStack Query hooks
    auth/           # login/register, token storage, refresh logic
    components/     # shadcn/ui-based building blocks
    features/
      today/        # today's checklist (check/partial/done, expand children)
      activities/   # create/edit activities + schedules + sub-activities
      history/      # per-day history view
      telegram/     # "Connect Telegram" flow
    routes/
    main.tsx
```

---

## 3. Data Model (draft)

> Final column types/constraints refined during Milestone 1. IDs use `uuid`.

**users**
- `id`, `name`, `phone` (unique), `password_hash`
- `telegram_chat_id` (nullable), `telegram_link_token` (nullable, for deep-link), `telegram_linked_at`
- `created_at`, `updated_at`

**refresh_tokens**
- `id`, `user_id` FK, `token_hash`, `expires_at`, `revoked_at` (nullable), `created_at`

**activities** (templates; self-referencing for sub-activities)
- `id`, `user_id` FK, `parent_id` (nullable FK → activities)
- `title`, `notes` (nullable)
- `freq` (`daily` | `weekly`), `days_of_week` (int[] for weekly, e.g. {0=Sun..6=Sat})
- `time_of_day` (`time`, Jakarta), `sort_order`
- `is_active` (bool), `created_at`, `updated_at`
- Sub-activities inherit the parent's schedule (they don't fire independently).

**occurrences** (per-day instances / history)
- `id`, `activity_id` FK, `occur_date` (`date`, Jakarta)
- `state` (`pending` | `partial` | `done`)
- `completed_at` (nullable)
- `per_activity_notified_at` (nullable), `digest_notified_at` (nullable)
- Unique `(activity_id, occur_date)`

**Rollup rule (occurrences of a parent + its children on the same date):**
- All child occurrences `done` → parent occurrence auto `done`.
- Parent occurrence set `done` manually → all child occurrences set `done`.
- Un-checking a child of a previously-auto-done parent → parent recomputes to
  `partial` (if some children done) or `pending` (if none). Manual override
  precedence documented in Milestone 3.

---

## 4. API Surface (draft)

```
POST   /api/auth/register          { name, phone, password }
POST   /api/auth/login             { phone, password } -> { access, refresh }
POST   /api/auth/refresh           { refresh } -> { access, refresh }
POST   /api/auth/logout            (revokes refresh)
GET    /api/me

GET    /api/activities             # list templates (tree)
POST   /api/activities             # create (with schedule, optional parent_id)
GET    /api/activities/:id
PATCH  /api/activities/:id
DELETE /api/activities/:id

GET    /api/today                  # today's occurrences (tree + state)
PATCH  /api/occurrences/:id        { state }   # triggers rollup

# History — two views:
GET    /api/history/calendar?from=&to=     # all occurrences grouped by date (calendar)
GET    /api/history/calendar/:date         # detail for a single day (occurrence tree)
GET    /api/history/activities/:id?from=&to=  # one activity's timeline of states

POST   /api/telegram/link          # issues deep-link token + bot URL
# Telegram updates handled via long-poll (dev) / webhook (prod) in telegram pkg
```

---

## 5. Milestones

Each milestone is independently testable. Mark `[x]` when its **Definition of
Done** is met and verified, then fill in the Result note.

> **Version control workflow:** Repo is on GitHub at
> `git@github.com:enricojoe/dailychecker.git` (branch `main`).
> When a milestone's DoD is met and verified, **commit and push** before moving on:
> mark its checkboxes/Result, then `git add -A && git commit && git push`. Commit
> messages are plain (no AI-attribution trailers), e.g. `M2: auth (register/login/refresh)`.

### Milestone 0 — Project Scaffolding & Tooling
- [x] Repo structure: `backend/`, `frontend/`, `tasks/` present
- [x] `docker-compose.yml` with Postgres for local dev (postgres:16)
- [x] Backend: Go module (`github.com/enricojoe/dailychecker`), Gin server with `/healthz`, config from **`backend/.env`** (+ `backend/.env.example`)
- [x] DB connection via sqlx + programmatic golang-migrate runner (library, no CLI dependency)
- [x] Frontend: Vite + React + TS scaffold, Tailwind v4 + shadcn/ui, TanStack Query, config from **`frontend/.env`** (Vite `VITE_*` vars, + `frontend/.env.example`)
- [x] Each project owns its own env file — no shared root `.env`; both `.env` files git-ignored, both `.env.example` committed (see §7)
- [x] Makefile targets for run, build, test, db-up, db-down
- **DoD:** ✅ backend `go build`/`go vet` pass and `/healthz` returns 200; frontend `npm run build` + `tsc --noEmit` clean, renders placeholder page; each project loads its own env.
- **Result:** _2026-06-19_ — Backend & frontend scaffolds delivered by go-gin-backend-architect + react-frontend-expert agents in parallel, both self-verified. Stack: React 19 / Vite 8 / Tailwind v4 / shadcn (base-ui) / TanStack Query 5; Go 1.26 / Gin / sqlx / lib/pq / golang-migrate. **Note:** local port 5432 is occupied by an unrelated `lotrack-db` container — stop it before `make db-up`, or remap to 5433 (deferred, not blocking). Migration runner no-ops cleanly until M1 adds `.sql` files.

### Milestone 1 — Database Schema & Migrations
- [x] Migrations for `users`, `refresh_tokens`, `activities`, `occurrences`
- [x] Indexes: `activities(user_id, is_active)`, `occurrences(activity_id, occur_date)` unique, `occurrences(occur_date, state)` for digest
- [x] sqlx repositories with basic CRUD + integration tests against a test DB
- **DoD:** `migrate up`/`down` run cleanly; repo tests pass.
- **Result:** _2026-06-20_ — Completed & verified (commit `de4877a`). 4 migrations (uuid PKs, FKs, `pending|partial|done` state, unique `(activity_id, occur_date)`); indexes per DoD present. Programmatic golang-migrate runner gained `RunMigrationsDown` + isolated short-lived connection so `m.Close()` never touches the app pool; up→down→up cycle test passes against real Postgres. sqlx repos for users/auth/activities/occurrences with 27 passing integration subtests (testhelper auto-loads `backend/.env` via godotenv). Fixed prior session's build break (missing `lib/pq` import) and switched activities queries off `SELECT *` to explicit columns with `time_of_day::TEXT` to avoid lib/pq TIME→time.Time scan issues. `go build`/`go vet` clean. Tests run against the dev DB on host port 5433.

### Milestone 2 — Auth (register, login, refresh, middleware)
- [x] bcrypt password hashing
- [x] JWT access tokens (short TTL, ~15m); refresh tokens hashed + stored in DB
- [x] `/register`, `/login`, `/refresh`, `/logout`, `/me`
- [x] Gin auth middleware; refresh rotation + revocation
- [x] Unit/integration tests incl. expired/invalid/revoked token paths
- **DoD:** Full auth cycle works via API tests; protected route rejects missing/invalid tokens.
- **Result:** _2026-06-20_ — Completed & verified (see [m2-auth.md](m2-auth.md)). HS256 JWT access tokens (15m, validated in-process, no per-request DB hit); refresh tokens are random + stored SHA-256-hashed, rotated on every refresh (old revoked before new issued; reused/revoked/unknown rejected). `auth` pkg: `token.go` (bcrypt + JWT + refresh gen), `service.go` (Register/Login/Refresh/Logout/Me + typed sentinels), `middleware.go` (`RequireAuth`). `httpapi`: DTOs, `{error}` envelope, handlers; `NewRouter` rewired for DI (`*auth.Service` + JWT secret), `main.go` wires repos→service→router. Bug caught & fixed: double-logout silently succeeded because `UPDATE … SET revoked_at` matches already-revoked rows — service now checks `RevokedAt != nil` first (lesson recorded). 29 auth tests pass (unit + HTTP integration incl. expired/invalid/revoked/reused paths); `go build`/`go vet`/`go test ./...` clean. Dep added: `github.com/golang-jwt/jwt/v5`.

### Milestone 3 — Activities & Sub-activities (templates + state rollup)
- [x] CRUD for activities incl. `parent_id`, schedule fields, ordering
- [x] Validation (weekly requires `days_of_week`; sub-activity can't have its own children beyond one level — confirm depth in build)
- [x] State rollup engine (parent↔child) with documented precedence + unit tests
- **DoD:** Can create a parent with children; marking all children done flips parent; manual parent-done flips children; un-checking recomputes. All covered by tests.
- **Result:** _2026-06-20_ — Completed & verified (see [m3-activities-rollup.md](m3-activities-rollup.md)). `activities.Service` (ownership-scoped, returns not-found on cross-user access — no existence leak) + 5 protected routes; `GET /api/activities` returns a nested tree built in-memory from one `ListByUser`. Validation: freq/days_of_week cross-field (handler on create, service merge+revalidate on PATCH), `time_of_day` normalized to `HH:MM:SS`, single-level depth (parent must exist, same user, top-level; sub-activity can't get children; no self-parent). Pure DB-free `internal/rollup.Apply(parent, children, changedID, newState)` engine with 5 documented precedence rules (manual-override is stateless/indistinguishable from auto by design) + 23 table-driven tests; 15 activities HTTP integration tests. **Also fixed a pre-existing M1 test-isolation race:** the `internal/db` up→down→up migration test dropped tables in the shared DB while other packages ran in parallel (`42P01`); it now runs against an isolated throwaway database, so default `go test ./...` is deterministic (verified 3× green). Sentinels: `ErrInvalidSchedule`, `ErrInvalidParent`, `ErrHasChildren`. Note: PATCH can't null out `notes`/`parent_id` (pointer/JSON limitation) — deferred to M9. `go build`/`go vet`/`go test ./...` clean.

### Milestone 4 — Occurrences, Today View & History
- [x] Daily occurrence generation for due activities (Jakarta date), idempotent
- [x] `GET /api/today` returns occurrence tree + state
- [x] `PATCH /api/occurrences/:id` applies state + rollup
- [x] **Calendar history:** `GET /api/history/calendar?from=&to=` (per-day rollup, e.g. state counts by date) + `GET /api/history/calendar/:date` (that day's occurrence tree)
- [x] **Per-activity history:** `GET /api/history/activities/:id?from=&to=` (one activity's timeline of states across dates)
- **DoD:** Generating occurrences twice for the same day is a no-op; today + both history endpoints return correct data; state changes persist and roll up.
- **Result:** _2026-06-20_ — Completed & verified (see [m4-occurrences-today-history.md](m4-occurrences-today-history.md)). `occurrences.Service` wires repo + `rollup` + activities repo + injected Jakarta `*time.Location` (loaded once in main.go, fail-fast). `GenerateForDate(userID, date)` idempotent via Upsert; due-logic: active top-level due if daily or weekly∧weekday∈days_of_week; active children fire iff parent due (inherit schedule). `Today` generates then returns occurrence tree. `SetState` ownership-checked (404 cross-user), fetches the parent+children group via new JOIN method `ListGroupByParentAndDate` (parent-first ORDER BY → groupOccs[0] is parent), runs `rollup.Apply`, persists every change, re-fetches for fresh completed_at. Calendar summary via `ListCalendarSummary` (single `COUNT(*) FILTER` grouped query, no N+1); calendar/:date tree; per-activity timeline. Validation: state value, date format, from≤to, range ≤365 days → 422; missing/cross-user → 404. 5 protected routes. New repo methods (read-only, JOIN-based): `ListGroupByParentAndDate`, `ListCalendarSummary` (+`CalendarDay` row struct). 21 subagent integration tests + **1 added by orchestrator** (`TestGenerateForDateWeeklyDueLogic`: passing an explicit Monday vs Tuesday proves weekly generates only on scheduled days — closed a DoD test gap the subagent had left, since `GenerateForDate` takes the date directly and needs no clock injection). `go build`/`go vet` clean; default-parallel `go test ./...` deterministic (verified multiple runs).

### Milestone 5 — Telegram Integration (linking + sending)
- [ ] Create bot via BotFather (manual, documented in README)
- [ ] `POST /api/telegram/link` issues one-time token + `t.me/<bot>?start=<token>` URL
- [ ] Bot update handler (long-poll for dev) captures `/start <token>` → saves `chat_id`
- [ ] Message-send helper with basic rate-limit safety
- [ ] Bot includes a link back to the web app
- **DoD:** A real user can connect their account; backend stores `chat_id`; a test send delivers a DM.
- **Result:** _TBD_

### Milestone 6 — Scheduler (per-activity reminders + nightly digest)
- [ ] `scheduler` package with `robfig/cron`, isolated behind an interface
- [ ] Minute-tick: find activities due now & not done → send per-activity reminder once (`per_activity_notified_at`)
- [ ] Nightly digest job at 22:00 (env `DIGEST_HOUR`): per user, list all not-done occurrences → single message (`digest_notified_at`)
- [ ] Only notify users with `telegram_chat_id`; all times in Jakarta
- [ ] Tests with injected clock; dedup so no double-send
- **DoD:** With a manipulated clock, due activities trigger exactly one reminder; digest sends one summary per user of remaining items.
- **Result:** _TBD_

### Milestone 7 — Frontend: Auth & Shell
- [ ] Login/register pages; token storage; auto-refresh on 401; logout
- [ ] App shell, routing, protected routes, API client + TanStack Query setup
- **DoD:** User can register, log in, stay logged in across refresh, and log out.
- **Result:** _TBD_

### Milestone 8 — Frontend: Activities, Today, History, Telegram
- [ ] Activity CRUD UI incl. schedule editor + sub-activities
- [ ] Today checklist: tri-state controls, expand/collapse children, optimistic updates with rollup reflected
- [ ] History — **Calendar view:** month grid with per-day status; click a day → that day's occurrence detail
- [ ] History — **By-activity view:** pick an activity → its timeline/streak of states over a date range
- [ ] "Connect Telegram" flow (deep-link button + connected state)
- **DoD:** End-to-end: create activity → appears in Today → check/partial/done with rollup → shows in both calendar and by-activity history → connect Telegram.
- **Result:** _TBD_

### Milestone 9 — Hardening, Docs & Deployment
- [ ] Input validation, consistent error responses, request logging
- [ ] Switch Telegram to webhook for prod (documented); secrets via env
- [ ] README: setup, env vars, bot creation, running locally
- [ ] Optional: Dockerfiles for backend/frontend; basic CI (build + test)
- **DoD:** Fresh clone → follow README → working app locally; tests green in CI.
- **Result:** _TBD_

---

## 6. Key Risks & Open Questions

- **Two-way Telegram (Q5):** still undecided; design keeps handler extensible.
- **Sub-activity depth:** plan assumes a single level (parent → children). Confirm during Milestone 3 whether deeper nesting is needed.
- **Per-activity reminder cadence:** fires once at scheduled time; nightly digest is the only follow-up (confirmed).
- **Multi-instance dedup:** deferred (single instance v1); upgrade path is `SELECT ... FOR UPDATE SKIP LOCKED`.
- **Manual override vs auto-rollup precedence:** to be finalized with concrete test cases in Milestone 3.

---

## 7. Environment Configuration

Each project owns its **own** env file — there is **no shared root `.env`**.
The `.env` files are git-ignored; the `.env.example` files are committed as templates.

### `backend/.env`
```
APP_ENV=development
PORT=8080
DATABASE_URL=postgres://dailychecker:dailychecker@localhost:5432/dailychecker?sslmode=disable
JWT_SECRET=change-me
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h
TIMEZONE=Asia/Jakarta
DIGEST_HOUR=22                     # nightly catch-all digest hour (Jakarta)
TELEGRAM_BOT_TOKEN=
TELEGRAM_BOT_USERNAME=             # for building t.me/<username>?start=<token> links
APP_PUBLIC_URL=http://localhost:5173   # link back to web app from the bot
```

### `frontend/.env`
```
VITE_API_BASE_URL=http://localhost:8080/api
```

> Note: Vite only exposes vars prefixed with `VITE_` to the browser. Never put
> secrets (JWT secret, bot token) in the frontend env — those live only in `backend/.env`.

---

## 8. Change Log

- _2026-06-18_ — Initial plan created from discussion. Status: ready for Milestone 0.
- _2026-06-19_ — Separate per-project env files (`backend/.env`, `frontend/.env`; no shared root) documented in §7 and M0. History split into two views — **calendar** and **by-activity** — across API (§4), M4, and M8.
