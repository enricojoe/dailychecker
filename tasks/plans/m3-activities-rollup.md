# M3 — Activities & Sub-activities (CRUD + state rollup engine)

> Execution plan for Milestone 3. Parent plan: [dailychecker.md](dailychecker.md) §Milestone 3.

## Goal
Authenticated CRUD over activity templates (incl. sub-activities, schedule, ordering) with validation, **plus** a pure, isolated **state-rollup engine** (parent↔child) with documented precedence and exhaustive unit tests. The engine is DB-free; M4 wires it to occurrence persistence.

## Existing building blocks (committed — reuse, don't alter schemas)
- `activities.Repository`: `Create`, `GetByID(id)` (NOT user-scoped — service must enforce ownership), `ListByUser(userID)` (flat, ordered parent_id NULLS FIRST, sort_order, created_at — build tree in memory), `Update`, `Delete` (ON DELETE CASCADE children+occurrences); sentinel `activities.ErrNotFound`. `Activity{ID,UserID,ParentID*,Title,Notes*,Freq,DaysOfWeek pq.Int64Array,TimeOfDay string "HH:MM:SS",SortOrder,IsActive,...}`.
- `occurrences` state constants: `StatePending`, `StatePartial`, `StateDone`.
- Auth: `auth.RequireAuth(jwtSecret)` middleware sets the user id in gin context (key `auth.ContextKeyUserID`). `httpapi` has `respondError` + `{error}` envelope and DI'd `NewRouter`.

## Part A — Activities CRUD (HTTP)
- [x] `activities.Service` over the repo; all ops scoped to the authenticated user (ownership check on GetByID/Update/Delete — return not-found if `a.UserID != ctxUserID`, don't leak existence).
- [x] Handlers under protected `/api/activities` group (behind `RequireAuth`):
  - `GET /api/activities` → user's activities as a **tree** (parents with nested children).
  - `POST /api/activities` → create (sets UserID from ctx; optional parent_id).
  - `GET /api/activities/:id`
  - `PATCH /api/activities/:id` (partial update)
  - `DELETE /api/activities/:id`
- [x] Validation (422 on failure, `{error}` envelope):
  - `freq` ∈ {`daily`,`weekly`}; `weekly` requires non-empty `days_of_week` with values 0–6 (no dups); `daily` must have empty `days_of_week`.
  - `time_of_day` parseable as `HH:MM` or `HH:MM:SS`, normalised to `HH:MM:SS` on store.
  - `title` non-empty.
  - **Single-level depth:** if `parent_id` is set, the parent must exist, belong to the same user, and itself be top-level (parent's `parent_id` IS NULL). A sub-activity cannot be given children. Cannot set `parent_id` to self.

## Part B — State rollup engine (pure, DB-free)
- [x] New isolated package `internal/rollup` (no DB, no gin imports). Operates on an in-memory parent + its children states.
- [x] Documented precedence rules (written in the package doc):
  1. **All children `done` → parent auto `done`.**
  2. **Parent set `done` manually → all children set `done`.**
  3. **Un-checking a child** (done→not) recomputes parent: ≥1 child done → `partial`; 0 done → `pending`.
  4. `partial` is settable manually on any item.
  5. A leaf (no children) keeps whatever state it is set to.
  6. Manual-override vs auto precedence documented with concrete cases in the package doc.
- [x] Pure API: `Apply(parent NodeState, children []NodeState, changedID, newState string) []NodeState` returning the full set of state changes to persist.
- [x] 23 exhaustive table-driven unit tests covering all rules + edge cases (single child, mixed states, re-check, no-parent leaf, cascade then uncheck).

## DoD (parent plan)
Create a parent with children; marking all children done flips parent; manual parent-done flips children; un-checking recomputes. All covered by tests. `go build`/`go vet`/`go test ./...` clean.

## Constraints
- Clean architecture, SOLID, idiomatic Go matching existing style (sentinels, `%w`, doc comments, explicit columns).
- No N+1; DB access only via repos. Do NOT build occurrence HTTP endpoints / today view / generation — that's M4. The rollup engine is pure logic only in M3.
- Don't touch the frontend. Don't commit — leave changes for orchestrator review.
- Dev DB on host port **5433**; integration tests use `testhelper`.

## Result
_2026-06-20_ — Completed & verified. `activities.Service` with sentinel errors (`ErrInvalidSchedule`, `ErrInvalidParent`, `ErrHasChildren`) and `ActivityNode` tree type; `buildTree` O(n) in-memory from single `ListByUser` call. Five protected routes (`GET/POST /api/activities`, `GET/PATCH/DELETE /api/activities/:id`). Cross-field freq/days_of_week validation: handler handles full cross-field on Create; service merges and revalidates on Update so partial PATCH stays consistent. `time_of_day` normalised to `HH:MM:SS`. Pure `internal/rollup.Apply(parent, children, changedID, newState)` engine with 5 precedence rules + stateless manual-override docs; 23 table-driven tests. 15 activity HTTP integration tests (unauthenticated, empty list, daily/weekly/sub-activity create, 11 create-validation 422 cases, ownership rejection, get by id, ownership 404, patch title/schedule/validation, patch ownership, delete, cascade). `go build`/`go vet`/`go test ./...` all clean.
