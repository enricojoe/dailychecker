# M4 — Occurrences, Today View & History

> Execution plan for Milestone 4. Parent plan: [dailychecker.md](dailychecker.md) §Milestone 4, §4 (API).

## Goal
Generate per-day occurrences for due activities (idempotent, Jakarta date), expose today's checklist, apply state changes with parent/child rollup, and serve two history views (calendar + by-activity).

## Existing building blocks (committed — reuse, don't alter schemas)
- `occurrences.Repository`: `Upsert(activityID, date)` (idempotent insert, state=pending), `GetByID`, `ListByActivityAndDateRange(activityID, from, to)`, `ListByUserAndDate(userID, date)` (single JOIN), `UpdateState(id, state)` (sets/clears completed_at). Constants `StatePending|StatePartial|StateDone`; sentinel `ErrNotFound`. `Occurrence{ID,ActivityID,OccurDate time.Time,State,CompletedAt*,PerActivityNotifiedAt*,DigestNotifiedAt*}`.
- `activities.Repository.ListByUser(userID)` (flat, parent-first ordered) + `activities.Service` (ownership-scoped). `Activity{...,ParentID*,Freq("daily"|"weekly"),DaysOfWeek pq.Int64Array(0=Sun..6=Sat),TimeOfDay,IsActive}`.
- `rollup.Apply(parent NodeState, children []NodeState, changedID, newState) []NodeState` — pure engine; returns the change-set to persist.
- `config.Config.Timezone` ("Asia/Jakarta"). Auth middleware + `respondError`/`{error}` envelope + DI'd `NewRouter`.

## Key design decisions (resolve these explicitly)
1. **Jakarta date:** load `time.LoadLocation(cfg.Timezone)` once; "today" = `time.Now().In(loc)` truncated to date. All `from`/`to`/`:date` params are Jakarta calendar dates (`YYYY-MM-DD`).
2. **Which activities are due on a date:**
   - Only `is_active = true`.
   - **Top-level** activity due if: `freq=daily` (always) OR `freq=weekly` AND weekday(date) ∈ `days_of_week`.
   - **Sub-activities inherit the parent's schedule and do NOT fire independently** (plan §3). So a child gets an occurrence **iff its parent is due on that date** (ignore the child's own freq/days). Only include active children of a due, active parent.
3. **Generation idempotency:** use `Upsert` per due (activity, date); running twice is a no-op (existing rows returned unchanged). Generation must NOT reset state of already-existing occurrences.
4. **Rollup group fetch:** to apply rollup on a PATCH you need the parent occurrence + all sibling child occurrences for that activity's group on that date. The current repo lacks this. **You MAY add new read-only repository methods** (e.g. fetch occurrences for a parent activity id + its children on a date, joined to activities) — keep them JOIN-based (no N+1), explicit columns, matching existing style. Do NOT change migrations or existing method signatures.

## Tasks
- [x] `occurrences.Service` (or extend) wiring repo + `rollup` + activities repo + Jakarta `*time.Location`:
  - `GenerateForDate(ctx, userID, date)` — idempotent; creates occurrences for all due active activities (parents + their active children per rule 2). Return nothing or the generated set.
  - `Today(ctx, userID)` — generate today's occurrences then return them as a **tree** (parent occurrence with nested child occurrences + activity title/meta needed by UI). Decide a clear response DTO (occurrence id, activity id, title, state, parent linkage, completed_at).
  - `SetState(ctx, userID, occurrenceID, newState)` — ownership-checked (occurrence's activity must belong to user → else 404). Validate state ∈ {pending,partial,done}. Load the rollup group (is the occ a parent or a child? fetch parent + children occurrences on same date), call `rollup.Apply`, persist every returned change via `UpdateState`. Return the updated group (or at least the changed + parent).
  - Calendar history: `CalendarSummary(ctx, userID, from, to)` — per-day state counts (e.g. `{date, pending, partial, done, total}`) via a single grouped JOIN query (no N+1). `CalendarDay(ctx, userID, date)` — that day's occurrence tree (reuse Today's tree shape).
  - Per-activity history: `ActivityHistory(ctx, userID, activityID, from, to)` — ownership-checked; timeline of `{date, state}` for one activity (use `ListByActivityAndDateRange`).
- [x] Handlers (all behind `RequireAuth`):
  - `GET /api/today`
  - `PATCH /api/occurrences/:id` body `{state}`
  - `GET /api/history/calendar?from=&to=`
  - `GET /api/history/calendar/:date`
  - `GET /api/history/activities/:id?from=&to=`
- [x] Validation: state value; date param format; `from<=to`; bounded range (>365 days rejected). 422 on bad input, 404 on cross-user/missing.
- [x] Wire service+handlers into `NewRouter` + `main.go` (load `*time.Location` from cfg there, fail fast on bad tz).

## DoD (parent plan)
Generating occurrences twice for the same day is a no-op; today + both history endpoints return correct data; state changes persist and roll up. `go build`/`go vet`/`go test ./...` clean.

## Constraints
- Clean architecture, SOLID, idiomatic Go matching existing style (sentinels, `%w`, doc comments, explicit SELECT columns, `time_of_day::TEXT` pattern already in activities repo).
- **No N+1** — calendar summary and group fetch must be JOIN/GROUP BY queries. DB access only via repos.
- Reuse the existing `rollup` engine — do NOT reimplement rollup logic in the service.
- Don't touch frontend, scheduler, or telegram. Don't commit — leave changes for orchestrator review.
- **Tests must be deterministic under the DEFAULT parallel `go test ./...`** (a prior shared-DB race was fixed by isolating destructive tests; do not reintroduce one — your integration tests should use unique user ids/data and must not drop/truncate shared tables). Dev DB on host port **5433**; use `testhelper`.

## Tests to include (must run, not skip)
- Generation idempotency: generate twice for a date → same occurrences, states preserved.
- Due logic: weekly activity only generates on its days; inactive activities/children excluded; child generates iff parent due.
- PATCH rollup over HTTP: all children done → parent done; parent done → children done; uncheck child → parent partial then pending; ownership 404; invalid state 422.
- Today tree shape correct.
- Calendar summary counts correct across a range; calendar/:date tree; per-activity timeline; from>to and bad-date → 422.

## Result
Completed 2026-06-20. All tasks done. `go build ./...`, `go vet ./...`, and two consecutive runs of `go test -count=1 ./...` all green (21 M4 tests pass, none skipped, no parallel races). Changes left in working tree per plan constraints.
