# AGENTS.md

Guidance for AI coding agents (Claude Code, Codex, Cursor, etc.) working in
this repository.

## What this project is

`asksolr` is a Go CLI that wraps an Anthropic-powered agent loop around an
Apache Solr HTTP client. The CLI accepts natural language; the agent uses
Claude's tool-use API to call `get_schema` and `execute_query` against Solr
until it produces a satisfactory result.

Read `README.md` first for the user-facing model — the developer section there
covers layout, build commands, and conventions.

## Module / import paths

- Go module: `github.com/CodeYogiCo/asksolr`
- The `Makefile`'s `MODULE` variable currently lists `github.com/codeyogico/asksolr`
  (lowercase) for `-ldflags` only. **Do not "fix" the case mismatch** unless
  the user asks — the build flag is purely cosmetic for the `version` symbol
  and changing the canonical module path requires updating every import.

## Where to make changes

| If you want to…                                | Edit…                                  |
| ---------------------------------------------- | -------------------------------------- |
| Add or change a CLI command                    | `cmd/*.go` (cobra commands)            |
| Change agent behaviour or prompts              | `internal/agent/agent.go`, `prompts.go`|
| Add a new Solr operation                       | `internal/solr/client.go` + `types.go` |
| Add a CLI-wide flag or env variable            | `internal/config/config.go` + `cmd/root.go` |
| Add a unit test                                | sibling `*_test.go` in the same pkg    |
| Add an integration test (needs live Solr)      | `tests/integration/*_test.go` with `//go:build integration` |

## Hard rules

1. **Never call the Anthropic SDK from `cmd/`.** All Claude interaction goes
   through `internal/agent`. Commands construct an `agent.Agent` and call its
   methods.
2. **Never build raw Solr URLs in `cmd/` or `internal/agent/`.** Add a method
   to `internal/solr.Client` and call that. This keeps the HTTP surface
   testable.
3. **Always thread `context.Context`** through new code paths — Solr calls,
   Claude calls, and any I/O take a `ctx` as the first argument.
4. **Preserve the `--dev` / `--prod` confirmation prompts** on destructive
   operations (`collection delete`, `index delete`). The `cfg.IsProd()` guard
   in `cmd/collection.go` and `cmd/index.go` is the pattern.
5. **Don't downgrade the Claude model.** `defaultModel` in
   `internal/agent/agent.go` is pinned to `claude-opus-4-7`. If the user wants
   a different model, add a flag; don't silently change the default.
6. **Don't skip hooks or tests.** The repo's CI runs `make fmt vet test-unit`
   on every push. Match that locally before declaring a task done.

## Running things locally

```sh
make build                        # dist/asksolr
make test-unit                    # no Solr needed
make solr-up && make solr-wait    # boot local Solr
make test-integration             # runs the integration build tag
make solr-down                    # tear down
```

Integration tests live behind `//go:build integration`, so a plain
`go test ./...` is safe and fast and will not require Docker.

## Tool-use loop, in one paragraph

`Agent.Search` (in `internal/agent/agent.go`) seeds a Claude message with the
user's NL query, then loops up to `maxIter` times. On each iteration it calls
`Messages.New` with the `get_schema` and `execute_query` tools declared, then
walks `resp.Content`: text blocks become `result.Explanation`, tool-use blocks
are dispatched in `dispatchTool` which returns `(jsonOutput, isError, solrQuery)`.
Tool results are appended as a single new user message; if Claude returns no
tool calls the loop exits. The first executed query is captured in
`result.SolrQuery` for display.

When adding a new tool, mirror the existing pattern: declare it in
`buildTools`, handle it in `dispatchTool`, and update `prompts.go` only if
Claude needs new guidance about when to use it.

## Style

- Errors wrap with `fmt.Errorf("...: %w", err)`.
- User-facing output uses `github.com/fatih/color`; do not write ANSI escapes
  directly.
- JSON I/O uses the stdlib `encoding/json`; do not introduce third-party JSON
  libraries.
- No new dependencies without checking whether the stdlib or an existing dep
  already covers it.

## Things to confirm before doing them

- Renaming the module path or binary name (`BINARY` in the Makefile).
- Changing the Solr image tag in `docker-compose.yml`.
- Adding network calls outside `internal/solr` or `internal/agent`.
- Anything that touches `.github/workflows/` — CI changes ship for everyone.
