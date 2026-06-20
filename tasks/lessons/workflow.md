# Lessons — Workflow

## Delegated subagents can build/test but cannot `go get` (no network for module fetches)

- **Context:** DailyChecker M6 — the `go-gin-backend-architect` subagent needed to add `github.com/robfig/cron/v3`.
- **Mistake:** The subagent wrote all source (importing the new module) but could not run `go get`/`go mod tidy`, so `go.sum` lacked the hashes and the package wouldn't build. It came to rest blocked, having done no verification. Earlier subagents (M1–M5) ran `go build`/`go vet`/`go test` fine — those are LOCAL. The difference is `go get` needs NETWORK to the module proxy, which the subagent sandbox blocks.
- **Correct Pattern:** When a milestone introduces a NEW external dependency, the orchestrator runs `go get <module>` + `go mod tidy` itself (it has network), either before delegating or to finish a blocked delegation. Tell the subagent the dep "will be present" and to write code against it; the orchestrator owns adding it + running the final build/vet/test. Don't expect a subagent to fetch modules.

## Some delegated agents have Bash/Skill blocked entirely — orchestrator owns ALL verification

- **Context:** DailyChecker M7 — the `react-frontend-expert` subagent.
- **Mistake:** Assumed the subagent could run `npx tsc --noEmit` / `npm run build` / `npm run lint`. Its sandbox blocked Bash AND Skill, so it wrote every file but verified NOTHING and came to rest. The orchestrator then found a build break (TS parameter-property vs `erasableSyntaxOnly`), 2 lint errors, and a wrong type (`UserDto.id`).
- **Correct Pattern:** Treat delegated agents as code-writers, not verifiers. The orchestrator ALWAYS runs the build/typecheck/lint/tests itself after a delegation and fixes the inevitable small breakages (it's faster than a blind round-trip since the agent can't run the toolchain to iterate). Pre-install deps, then verify + fix locally. Budget for a fix pass every delegation. Behavioral DoDs (e.g. auth flow) also need an orchestrator-run live check — a green build doesn't prove the integration works.

## Orphaned background dev servers shadow published ports — clean them up before live tests

- **Context:** DailyChecker M9 — full-stack `docker compose` smoke test. The backend container reported all routes registered, but `POST /api/auth/register` on host :8080 returned 404 while `/healthz` returned 200.
- **Mistake:** Stale `go run`/`server` processes from earlier boot tests (and one from a session days earlier — the M0 scaffold with only `/healthz`) were still alive and holding host port 8080. The container's `8080:8080` publish lost the race, so the smoke test silently hit the OLD M0 server (healthz-only → 404 for /api). Wasted time suspecting the new code.
- **Correct Pattern:** Background `go run ... &` servers started for verification DON'T die on their own and persist across sessions, squatting on ports. Always tear them down (`pkill -f exe/server`, or kill the PID from `lsof -nP -iTCP:<port> -sTCP:LISTEN`) before a live/container test on the same port. Better: when smoke-testing, prefer a unique throwaway port for ad-hoc host runs and reserve the canonical port for the real artifact under test — and confirm WHICH process answers (`lsof` / a version marker / log line) before trusting a 200.
