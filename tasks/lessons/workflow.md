# Lessons — Workflow

## Delegated subagents can build/test but cannot `go get` (no network for module fetches)

- **Context:** DailyChecker M6 — the `go-gin-backend-architect` subagent needed to add `github.com/robfig/cron/v3`.
- **Mistake:** The subagent wrote all source (importing the new module) but could not run `go get`/`go mod tidy`, so `go.sum` lacked the hashes and the package wouldn't build. It came to rest blocked, having done no verification. Earlier subagents (M1–M5) ran `go build`/`go vet`/`go test` fine — those are LOCAL. The difference is `go get` needs NETWORK to the module proxy, which the subagent sandbox blocks.
- **Correct Pattern:** When a milestone introduces a NEW external dependency, the orchestrator runs `go get <module>` + `go mod tidy` itself (it has network), either before delegating or to finish a blocked delegation. Tell the subagent the dep "will be present" and to write code against it; the orchestrator owns adding it + running the final build/vet/test. Don't expect a subagent to fetch modules.
