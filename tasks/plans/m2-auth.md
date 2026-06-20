# M2 — Auth (register, login, refresh, logout, me)

> Execution plan for Milestone 2. Parent plan: [dailychecker.md](dailychecker.md) §Milestone 2.

## Goal
Full auth cycle over the existing repositories: register → login → access protected route → refresh (rotated) → logout (revoked). JWT access tokens (short TTL) + DB-stored, hashed refresh tokens with rotation & revocation. Gin auth middleware protecting `/api/me`.

## Existing building blocks (already committed, do not duplicate)
- `users.Repository`: `Create`, `GetByID`, `GetByPhone`, `Update`; sentinels `ErrNotFound`, `ErrConflict` (duplicate phone → 23505).
- `auth.TokenRepository`: `Create`, `GetByHash`, `Revoke`; `RefreshToken{ID,UserID,TokenHash,ExpiresAt,RevokedAt,CreatedAt}` + `IsValid()`; `ErrTokenNotFound`.
- `config.Config`: `JWTSecret`, `AccessTokenTTL` (15m), `RefreshTokenTTL` (720h).
- `httpapi.NewRouter()` currently takes no args and registers only `/healthz`; `cmd/server/main.go` calls it. Both must be refactored to inject dependencies.

## Tasks
- [x] Add JWT dep: `go get github.com/golang-jwt/jwt/v5`. bcrypt via existing `golang.org/x/crypto/bcrypt`.
- [x] `auth` package: password hash/verify (bcrypt); JWT issue/parse (HS256, `JWTSecret`, subject=user id, short TTL); refresh-token generation (random secret) + hashing (SHA-256 is fine; store only hash) + TTL.
- [x] `auth` Gin middleware: validate `Authorization: Bearer <access>`, set user id in context; 401 on missing/invalid/expired.
- [x] `auth.Service` (or `users.Service`): `Register(name,phone,password)`, `Login(phone,password) -> (access, refresh)`, `Refresh(refresh) -> (access, refresh)` with rotation (revoke old, issue new), `Logout(refresh)` (revoke), `Me(userID)`. Map repo sentinels → typed service errors.
- [x] `httpapi`: DTOs + handlers for `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/refresh`, `POST /api/auth/logout`, `GET /api/me` (protected). Consistent JSON error shape. Input validation (gin validator already available).
- [x] Refactor `NewRouter` to accept dependencies (e.g. `*sqlx.DB` + `*config.Config`, or constructed services) and register the `/api` group; update `main.go` wiring.
- [x] Tests: unit (hashing, JWT issue/parse incl. expired/invalid/wrong-secret) + integration over HTTP for the full cycle, including expired/invalid/revoked refresh and protected-route rejection.

## DoD (from parent plan)
Full auth cycle works via API tests; protected route rejects missing/invalid tokens; refresh rotation + revocation verified; `go build`/`go vet`/`go test ./...` clean.

## Constraints
- No N+1; repos/migrations only for DB access. Clean architecture, SOLID, idiomatic Go matching existing style.
- Do not start M3+ (no activities/occurrences handlers). Don't touch frontend.
- Dev DB on host port **5433**; tests use the `testhelper`.

## Result
_2026-06-20_ — Completed & verified. New files: `internal/auth/token.go` (bcrypt hash/verify, HS256 JWT issue/parse, SHA-256 refresh token generation), `internal/auth/service.go` (Register/Login/Refresh/Logout/Me with sentinel errors), `internal/auth/middleware.go` (RequireAuth Gin middleware), `internal/httpapi/dto.go`, `internal/httpapi/error.go`, `internal/httpapi/auth_handler.go`. Updated: `internal/httpapi/router.go` (NewRouter now accepts `*auth.Service` + JWT secret; `/api/auth/*` routes + protected `/api/me`), `cmd/server/main.go` (wires repos → service → router). Bug found and fixed: `Logout` must check `RevokedAt != nil` before calling `Revoke` or a second logout with the same token silently succeeds (UPDATE matches the row regardless). JWT dep added: `github.com/golang-jwt/jwt/v5 v5.3.1`. All 29 auth tests pass (unit + integration): `go build`, `go vet`, `go test ./...` clean.
