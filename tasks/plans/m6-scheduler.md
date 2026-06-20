# M6 — Scheduler (per-activity reminders + nightly digest)

> Execution plan for Milestone 6. Parent plan: [dailychecker.md](dailychecker.md) §Milestone 6.

## Goal
An in-process scheduler that (a) sends a per-activity Telegram reminder once at each activity's scheduled time, and (b) sends a nightly digest of everything still not done. Isolated behind an interface, with an **injected clock** so the tick logic is unit-testable without real time passing.

## Existing building blocks (committed — reuse, don't alter schemas)
- `telegram.Client` interface: `SendMessage(ctx, chatID int64, text string) error` (real impl + mock already exist). REUSE — scheduler depends only on this interface.
- `occurrences` schema columns already present: `per_activity_notified_at *time.Time`, `digest_notified_at *time.Time` (dedup flags). State constants `StatePending|StatePartial|StateDone`.
- `users.User.TelegramChatID *int64` (only notify users with a non-nil chat id).
- `activities.Activity`: `ParentID *string`, `TimeOfDay string` ("HH:MM:SS", Jakarta), `IsActive`, `Title`. Children inherit parent schedule (don't fire independently).
- `config.Config`: `DigestHour int` (default 22, Jakarta), `Timezone` ("Asia/Jakarta"), `AppPublicURL`.
- `cmd/server/main.go` builds repos/services + a Jakarta `*time.Location`; graceful shutdown cancels contexts on SIGINT/SIGTERM.

## Design decisions (resolve explicitly)
1. **Clock injection:** define a small clock seam (e.g. `type Clock func() time.Time`, or a `Clock` interface with `Now()`). Production uses `time.Now`; tests inject a fixed time. The scheduler stores the Jakarta `*time.Location`.
2. **Two directly-callable core methods** (cron just calls these — tests call them with a fixed `now`, no cron, no sleeping):
   - `RunReminderTick(ctx, now time.Time) error` — per-activity reminders.
   - `RunDigest(ctx, now time.Time) error` — nightly digest.
3. **Cron wiring:** `Start(ctx)` registers a minute-tick (`* * * * *`) → `RunReminderTick(ctx, clock.Now())`, and a daily job at `DigestHour:00` → `RunDigest`. Use `robfig/cron/v3` with the Jakarta location. `Stop()`/context cancel halts it cleanly on shutdown. Keep cron usage thin; all logic lives in the two methods.
4. **Reminder selection (fires once, robust to restarts):** for "today" (Jakarta date), select occurrences where: the activity is **top-level** (`parent_id IS NULL`), active; `time_of_day <= now`'s Jakarta time; occurrence `state != 'done'`; `per_activity_notified_at IS NULL`; owner has `telegram_chat_id`. Send one reminder, then set `per_activity_notified_at = NOW()`. (Using `<=` not `= exact minute` means a reminder missed during downtime still fires on the next tick — still exactly once thanks to the dedup flag. Document this choice.)
5. **Digest selection:** for today, per user **with a chat id**, gather all occurrences with `state != 'done'` AND `digest_notified_at IS NULL` (parents + children). One message per user listing the items; then mark every included occurrence's `digest_notified_at = NOW()`. Users with nothing outstanding get no message.
6. **No N+1:** selection queries are JOINs across occurrences⋈activities⋈users returning the chat id + title needed; marking is by occurrence id (batch if convenient).

## Tasks
- [ ] Add dep: `go get github.com/robfig/cron/v3`; `go mod tidy`.
- [ ] Repo methods (add to `occurrences` repo, JOIN-based, return small notification row DTOs):
  - `ListDueReminders(ctx, date, asOf time.Time) ([]ReminderRow, error)` — ReminderRow{OccurrenceID, ChatID, Title} per selection rule 4.
  - `MarkPerActivityNotified(ctx, occurrenceID) error`.
  - `ListDigestItems(ctx, date) ([]DigestRow, error)` — DigestRow{UserID, ChatID, Title} (or grouped), per rule 5, ordered by user.
  - `MarkDigestNotified(ctx, occurrenceID) error` (or batch by ids).
- [ ] `internal/scheduler` package: `Scheduler` struct (deps: occurrences repo, `telegram.Client`, `*time.Location`, clock, `DigestHour`, `AppPublicURL`); `RunReminderTick`, `RunDigest`, `Start(ctx)`, `Stop()`. Compose reminder/digest message text (include activity title(s); digest includes `AppPublicURL`).
- [ ] Wire into `main.go`: construct the scheduler with the real clock + Jakarta loc, and `Start` it **only when a `telegram.Client` is available** (i.e. `TELEGRAM_BOT_TOKEN` set — mirror M5's conditional); cancel/stop on shutdown. Server must still boot with no token (scheduler simply not started).
- [ ] Tests (injected clock + mock `telegram.Client` + real DB; deterministic & parallel-safe — unique users, no shared-table teardown):
  - Reminder fires exactly once: call `RunReminderTick` twice with the same `now` → only one send (2nd is no-op via flag).
  - Reminder respects time: `time_of_day` in the future relative to `now` → no send.
  - Reminder skips done occurrences and users without a chat id; skips sub-activities.
  - Digest: one message per user summarizing only not-done items; running twice → no second send; user with all-done or no chat id → no message.
  - Message text contains the activity title (+ digest contains AppPublicURL).

## DoD (parent plan)
With a manipulated clock, due activities trigger exactly one reminder; digest sends one summary per user of remaining items. `go build`/`go vet`/`go test ./...` clean.

## Constraints
- Clean architecture, SOLID, idiomatic Go matching existing style (sentinels, `%w`, doc comments, explicit SELECT columns, `time_of_day::TEXT` cast pattern if selecting it). Scheduler isolated behind its own package + interface for future upgrade (plan notes a `FOR UPDATE SKIP LOCKED` multi-instance path later — leave room, don't build it).
- Reuse `telegram.Client`; do NOT call the Telegram HTTP API directly. No real network in tests.
- DB access only via repos. No N+1. Times in Jakarta. Don't double-send (dedup via the two columns).
- Don't touch frontend. Don't commit — leave changes for orchestrator review.
- Tests deterministic under DEFAULT parallel `go test ./...`; don't reintroduce the shared-DB race. Dev DB on host port **5433**; use `testhelper`.

## Result
_TBD_
