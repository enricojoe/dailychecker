# Change: Replace `phone` with `username` (login identifier)

> Post-M9 change. Drop the `phone` field entirely; introduce `username` as the
> unique login identifier. Decided with the user: login by **username**.

## New contract (source of truth for both backend & frontend)
- **username:** required, unique, 3–30 chars, trimmed, no spaces. Stored as-is.
- `POST /api/auth/register` `{ name, username, password }` → `201 { id, name, username, created_at, updated_at }`; `409` username taken; `422` validation (name 1–100, username 3–30, password ≥ 8).
- `POST /api/auth/login` `{ username, password }` → `200 { access, refresh }` | `401`.
- `GET /api/me` → `{ id, name, username, telegram_chat_id?, telegram_linked_at?, created_at, updated_at }` (NO phone).
- Error envelope unchanged: `{ "error": msg }`.

## Backend (go-gin-backend-architect)
- DB: NEW migration `000005_replace_phone_with_username.{up,down}.sql` (do NOT edit 000001). up: drop `phone` (+ its unique index), add `username TEXT NOT NULL UNIQUE` (backfill existing rows so up runs on a populated DB, e.g. `username = 'user_' || id` before NOT NULL). down: reverse. `migrate up`/`down` must run cleanly.
- `users`: `User.Phone`→`Username`; `GetByPhone`→`GetByUsername`; `Create` inserts username; `ErrConflict` = username taken. Explicit SELECT columns.
- `auth/service.go`: Register takes username; Login by username.
- `httpapi`: `RegisterRequest`/`LoginRequest` phone→username (+ binding tags); handler; `error.go` if needed; `/me` response.
- Tests: update ALL fixtures across packages (users/auth/activities/occurrences/scheduler/telegram/httpapi) that set `Phone:`/`phone` to use username; keep deterministic + parallel-safe (unique usernames per test). `go build`/`vet`/`go test ./...` ×2 green.
- Update `backend/.env.example`? No phone there — n/a.

## Frontend (react-frontend-expert)
- `auth/api.ts`: `UserDto.phone`→`username`; `RegisterDto.phone`→`username`; `login(username, password)`.
- `AuthContext.tsx`: login/register signatures phone→username.
- `LoginPage.tsx` / `RegisterPage.tsx`: phone field → username field (label, state, validation min 3, error copy).
- tsc/build/lint clean (orchestrator runs).

## Verification (orchestrator)
- Backend: build/vet/test ×2; migrate up/down. Frontend: tsc/build/lint.
- Live: register `{name, username, password}` → 201; login `{username, password}` → 200; `/me` shows username, no phone.
- Note: existing dev DB rows get backfilled `user_<id>` usernames (unknown to users) — for a clean slate reset the volume.

## Result

Backend refactor complete (2026-06-20).

### What was changed
- **Migration 000005 (up):** `ADD COLUMN username TEXT`, backfill `'user_' || id::text`, `SET NOT NULL`, `CREATE UNIQUE INDEX users_username_unique`, `DROP CONSTRAINT users_phone_key`, `DROP COLUMN phone`.
- **Migration 000005 (down):** reverse — re-adds `phone` with backfill `'unknown_' || id::text`, restores `users_phone_key` constraint, drops `users_username_unique` index and `username` column.
- **`internal/users/repository.go`:** `User.Phone` → `User.Username` (`db:"username" json:"username"`); `ErrConflict` message updated; `GetByPhone` → `GetByUsername`; `GetByID` and `GetByLinkToken` switched to explicit SELECT columns (no more `SELECT *`); `Create` inserts `username`.
- **`internal/auth/service.go`:** `Register(name, username, password)`, `Login(username, password)`, doc comments updated.
- **`internal/httpapi/dto.go`:** `RegisterRequest.Phone` → `Username` (`binding:"required,min=3,max=30"`); `LoginRequest.Phone` → `Username`.
- **`internal/httpapi/auth_handler.go`:** `req.Phone` → `req.Username` in register + login handlers; comment updated.
- **`internal/httpapi/error.go`:** 409 message "phone already registered" → "username already registered".
- **All 9 test files:** `Phone:` fixtures replaced with `Username:` using unique `fmt.Sprintf` values; auth integration tests updated to send/assert `username` field; `uniquePhone()`/`uniquePhone` helpers renamed to `uniqueUsername()`.

### Verification
- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test -count=1 ./...` — all 11 packages pass (×2 runs).
- Migration cycle test (`internal/db.TestMigrationsUpDown`) passes both runs (isolated throwaway DB).

### Risk note
Dev DB rows get backfilled to `username = 'user_<uuid>'` on `migrate up`. To start fresh, drop the volume (`docker compose -f docker/docker-compose.yml down -v && make db-up`).

### Orchestrator verification (2026-06-20) — DONE
- Frontend: zero `phone` refs in `src/`; tsc/build/lint clean.
- Backend: build/vet clean; `go test ./...` ×2 green (incl. migration up/down cycle); fixed one stale "phones" test comment.
- **Migration applied to the populated dev DB** (not just fresh test DBs): schema now `username NOT NULL UNIQUE`, no `phone` column.
- **Live E2E:** register `{name,username,password}`→201 (username in body, no phone); duplicate username→409; username "ab"→422; login `{username,password}`→200; `/me`→keys `[id,name,username,…]`, no phone; old `{phone,…}` login→422 (rejected).
- Only the historical migration `000001` still defines `phone` (correct — `000005` drops it). Plan §1/§3/§4 updated to reflect username. Committed.
