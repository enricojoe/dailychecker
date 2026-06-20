# Lessons — Database

## lib/pq: array columns and TIME scanning

- **Context:** Go + sqlx + `github.com/lib/pq` against PostgreSQL (DailyChecker M1 repositories).
- **Mistake:** A repository used `pq.Int64Array` for an `int[]` column (`days_of_week`) but never imported `github.com/lib/pq`, breaking the build (`undefined: pq`). Separately, `SELECT *` into structs with a `time`/`TIME` column scanned ambiguously under lib/pq's TIME→`time.Time` handling.
- **Correct Pattern:**
  - When using `pq.Int64Array` / `pq.StringArray` for Postgres array columns, import `github.com/lib/pq` and tag the field, e.g. `DaysOfWeek pq.Int64Array \`db:"days_of_week"\``.
  - Avoid `SELECT *`; list explicit columns. For `TIME` columns scanned into a string, cast in SQL: `time_of_day::TEXT AS time_of_day`. This sidesteps lib/pq's TIME→`time.Time` scan ambiguity.

## golang-migrate runner: don't let m.Close() kill the app pool

- **Context:** Programmatic golang-migrate runner sharing the application's `*sql.DB`.
- **Mistake:** Passing the live `*sql.DB` into the migrate instance meant `m.Close()` could close the connection the rest of the app relies on.
- **Correct Pattern:** Have the migration runner take the `databaseURL string` and open its own short-lived `*sql.DB` for the migration, then close that — never the caller's pool. Provide both `RunMigrations` (up) and `RunMigrationsDown` (down).

## Integration tests need the DB on the right host port

- **Context:** DailyChecker dev Postgres remapped from host port 5432 → **5433** (5432 occupied by an unrelated local container).
- **Correct Pattern:** `DATABASE_URL` in `backend/.env`/`.env.example` and `docker-compose.yml` all target 5433. Test helper auto-loads `backend/.env` via `godotenv` so integration tests actually run (not silently skipped). Bring the DB up with `make db-up` (or `docker compose up -d`) before `go test ./...`.

## SQL UPDATE matches already-updated rows — check application state first

- **Context:** DailyChecker M2 — `Logout` service method calling the `TokenRepository.Revoke` method on an already-revoked refresh token.
- **Mistake:** `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1` matches the row regardless of whether `revoked_at` is already set, returning 1 row affected and no error. A second logout with the same token silently succeeded.
- **Correct Pattern:** Before calling `Revoke`, fetch the token with `GetByHash` and inspect its application state (`RevokedAt != nil`) to detect the already-revoked case. Return a sentinel error (`ErrTokenInvalid`) rather than letting the DB mutation determine business validity.

## Destructive migration tests race the shared DB under parallel `go test ./...`

- **Context:** DailyChecker — `internal/db` `TestMigrationsUpDown` ran an up→down→up cycle against the shared `dailychecker` test DB; all other integration-test packages use the same DB.
- **Mistake:** `go test ./...` runs package test binaries IN PARALLEL by default. The `down` phase dropped all tables mid-flight, so concurrent packages failed intermittently with `pq: relation "occurrences" does not exist (42P01)`. `go test -p 1` masked it; the default run failed. Test caching/timing hid it on the first M3 run.
- **Correct Pattern:** Any test that DROPs/recreates schema must run against its OWN isolated, throwaway database — never the shared one. Derive an admin DSN (point the URL path at the `postgres` maintenance DB), `CREATE DATABASE test_xxx_<millis>_<pid>` on a short-lived `*sql.DB` (CREATE/DROP DATABASE can't run in a transaction), run the cycle there, then terminate connections + `DROP DATABASE` in cleanup. Never make the suite depend on `-p 1`. Requires the dev role to have `CREATEDB`.
