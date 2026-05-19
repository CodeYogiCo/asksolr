# asksolr

> Agentic Apache Solr CLI — query Solr in plain English using Claude.

`asksolr` is a Kubernetes-style command-line tool that lets you talk to a Solr
cluster using natural language. Claude inspects the collection schema, builds
the appropriate Solr (Lucene) query, executes it, and refines the query until
it returns useful results — without you ever writing a `q=*:*` by hand.

It also wraps the boring-but-necessary Solr admin tasks (collections, schema,
indexing) behind one consistent CLI surface so you don't need to remember
which `/solr/admin/...` endpoint does what.

## Features

- **Plain-English search.** `asksolr search "products under $50 with 4+ stars"`
  → Claude reads the schema, writes the Lucene query, runs it, refines if
  empty, and prints the documents.
- **Schema-aware.** The agent always inspects the live schema before building
  a query, so field types (date, numeric, text, multivalued) are respected.
- **Multi-environment.** Single `--dev` / `--prod` flag prints a banner and
  requires typed confirmation for destructive operations against production.
- **Full admin surface.** Create/list/delete collections, add/delete
  documents, inspect schemas — all from one binary.
- **JSON output.** Every output mode has a `--json` variant for piping into
  `jq` or other tooling.
- **Verbose tracing.** `-v` prints every tool call Claude makes, so you can
  see the agent's reasoning step by step.
- **Single static binary.** No runtime dependencies; cross-compiles to
  linux/darwin/windows on amd64/arm64.

## Example

```text
$ asksolr search "products under $50 that are in stock and have 4+ star rating" -c products
→  dev environment

🔍 Query: products under $50 that are in stock and have 4+ star rating
   Collection: products | env: dev

✓  Found 23 document(s)  [Solr query: rating:[4 TO *]]
   I filtered by rating ≥ 4, price < 50, and in_stock:true, sorted by rating desc.
   iterations: 2

  [1]
    id:                 SKU-9912
    name:               Wireless Mouse
    price:              19.99
    rating:             4.5
    in_stock:           true
  [2]
    id:                 SKU-4471
    ...
────────────────────────────────────────────────────────────
```

---

## Table of contents

- [How to use it](#how-to-use-it)
  - [Install](#install)
  - [Configure](#configure)
  - [How it works](#how-it-works)
  - [Commands](#commands)
  - [Environments](#environments)
  - [Output formats](#output-formats)
  - [Global flags](#global-flags)
  - [Environment variables](#environment-variables)
  - [Cost and model](#cost-and-model)
  - [Security notes](#security-notes)
  - [Troubleshooting](#troubleshooting)
  - [Limitations](#limitations)
- [Getting started for developers](#getting-started-for-developers)
  - [Prerequisites](#prerequisites)
  - [Repo layout](#repo-layout)
  - [Architecture](#architecture)
  - [Build and run](#build-and-run)
  - [Tests](#tests)
  - [Local Solr](#local-solr)
  - [Lint, format, tidy](#lint-format-tidy)
  - [Adding a new Claude tool](#adding-a-new-claude-tool)
  - [Adding a new CLI command](#adding-a-new-cli-command)
  - [Project conventions](#project-conventions)
  - [Releasing](#releasing)
  - [Contributing](#contributing)
  - [References](#references)

---

## How to use it

### Install

**Homebrew (macOS / Linux):**

```sh
brew install CodeYogiCo/tap/asksolr
```

This pulls the latest release from
[CodeYogiCo/homebrew-tap](https://github.com/CodeYogiCo/homebrew-tap) and
drops `asksolr` on your `$PATH`.

**From a release binary:** download the archive for your OS/arch from the
[Releases page](https://github.com/CodeYogiCo/asksolr/releases), verify
against `checksums.txt`, and drop the `asksolr` binary somewhere on your
`$PATH`.

```sh
# Example for macOS arm64
curl -L -o asksolr.tar.gz \
  https://github.com/CodeYogiCo/asksolr/releases/latest/download/asksolr_<VERSION>_darwin_arm64.tar.gz
tar -xzf asksolr.tar.gz
sudo mv asksolr /usr/local/bin/
asksolr --help
```

**From source:**

```sh
git clone https://github.com/CodeYogiCo/asksolr.git
cd asksolr
make build
sudo mv dist/asksolr /usr/local/bin/
```

**Verify the install:**

```sh
asksolr --help
asksolr collection list --solr-url http://localhost:8983
```

### Configure

`asksolr` needs two things:

1. **A reachable Solr cluster.** By default it talks to
   `http://localhost:8983` and appends `/solr` to form the base URL. Override
   with `--solr-url`. There is no implicit difference between `--dev` and
   `--prod` other than the banner and the confirmation prompts — point each
   flag at whichever URL you like via `--solr-url`, or wrap the binary in a
   shell alias / shim that sets a different `--solr-url` for production.
2. **An Anthropic API key** (required only for the `search` command). Set it
   via the `ANTHROPIC_API_KEY` environment variable (recommended) or pass
   `--api-key` on the command line. `collection`, `index`, and `schema`
   commands do not need an API key — they only talk to Solr.

```sh
export ANTHROPIC_API_KEY=sk-ant-...
# optional: pin your default Solr cluster via a shell alias
alias asksolr='asksolr --solr-url https://solr.internal.example.com'
```

### How it works

The `search` command runs an **agent loop**:

1. The CLI sends your natural-language query to Claude along with a
   description of two tools: `get_schema` and `execute_query`.
2. Claude typically calls `get_schema` first to learn the field names and
   types in your collection.
3. Claude then calls `execute_query` with a Lucene query built from the
   schema (using `q`, `fq`, `sort`, `rows`, and `fl` parameters).
4. The CLI runs the query against Solr and feeds the result count + first
   page of documents back to Claude.
5. If the result set is empty or clearly wrong, Claude refines the query and
   tries again — up to `--max-iterations` times (default 5).
6. When Claude stops calling tools, the CLI prints the final result, the
   actual Solr query it ran, and a short explanation.

All admin commands (`collection`, `index`, `schema`) skip the agent entirely
and call Solr directly — they are fast, deterministic, and free.

### Commands

#### `asksolr search` — agentic natural-language search

```sh
asksolr search "<plain-english query>" [--collection NAME] [--json]
```

The flagship command. Examples:

```sh
# Numeric + boolean filter
asksolr search "products under $100 with 4+ star rating" -c products

# Range + date filter
asksolr search "news articles about climate published in 2024" -c articles

# Negation + sort
asksolr search "users who signed up last week and haven't verified email" -c users

# Raw JSON output, piped through jq
asksolr search "top 5 most expensive in-stock items" -c products --json | jq '.docs[]'
```

Notable flags:

- `-c, --collection NAME` — override the default collection just for this
  call.
- `--json` — emit the raw `SearchResult` struct as JSON. Useful for
  piping/scripting.
- `-v, --verbose` (root flag) — print every tool call Claude makes, the
  arguments, and the response from Solr. Great for debugging "why did it
  build *that* query?"
- `--max-iterations N` (root flag) — cap the refinement loop. The default is
  5; you rarely need more.

Verbose output looks roughly like:

```text
[iter 1] stop_reason=tool_use
[tool] get_schema({})
[iter 2] stop_reason=tool_use
[tool] execute_query({"q":"*:*","fq":"price:[* TO 100] AND rating:[4 TO *]","rows":10})
[claude] Found 23 products matching the criteria. Returning the top 10 sorted by rating.
[iter 3] stop_reason=end_turn
```

#### `asksolr collection` — manage collections

```sh
asksolr collection list                              # list all collections
asksolr collection ls                                # alias for list
asksolr collection create products --shards 2 -r 2   # create with 2 shards × 2 replicas
asksolr collection delete products                   # delete (asks for confirmation in --prod)
```

Flags on `create`:

- `-s, --shards N` — number of shards (default 1)
- `-r, --replicas N` — replication factor (default 1)

`delete` against `--prod` prompts for a typed `yes` before proceeding.

#### `asksolr index` — add or delete documents

```sh
# Index a JSON file containing an array of objects
asksolr index add ./products.json --collection products

# Or a single document
asksolr index add ./one-product.json --collection products

# Delete by Solr query
asksolr index delete "price:[* TO 0]" --collection products

# Delete everything in a collection (use with care)
asksolr index delete "*:*" --collection products --dev
```

`index add` autodetects whether the JSON file is an array or a single object
and indexes accordingly. `commit=true` is sent on every batch, so changes are
immediately visible.

`index delete` in `--prod` requires a typed `yes` confirmation.

Sample input file (`products.json`):

```json
[
  { "id": "SKU-1", "name": "Widget",   "price": 9.99,  "in_stock": true },
  { "id": "SKU-2", "name": "Gadget",   "price": 19.99, "in_stock": false },
  { "id": "SKU-3", "name": "Thingamy", "price": 4.99,  "in_stock": true  }
]
```

#### `asksolr schema` — inspect the schema

```sh
asksolr schema show --collection products
```

Prints the field list with their type, indexed flag, and stored flag:

```text
Schema for collection [products] [dev]
  Name:       default-config
  UniqueKey:  id
  Fields (12):
  Field                      Type           Indexed  Stored
  ─────────────────────────────────────────────────────
  id                         string         true     true
  name                       text_general   true     true
  price                      pfloat         true     true
  rating                     pfloat         true     true
  in_stock                   boolean        true     true
  ...
```

The same call backs the agent's `get_schema` tool, so anything Claude sees,
you see.

### Environments

Every command accepts `--dev` (default) or `--prod`. They are mutually
exclusive. The flag is printed in the banner so you always know which
cluster you're hitting:

```text
→  dev environment        # cyan, printed for --dev
⚠  PRODUCTION environment # yellow, printed for --prod
```

Destructive operations against `--prod` (`collection delete`,
`index delete`) require a typed `yes` confirmation before proceeding.

> **Note:** `--dev` / `--prod` is currently a UX-level flag — it changes the
> banner and the confirmation behavior. The actual Solr URL still comes from
> `--solr-url`. If you operate against multiple clusters, wire the URL into
> a wrapper script or shell alias.

### Output formats

| Command                  | Default               | `--json` |
| ------------------------ | --------------------- | -------- |
| `search`                 | human-readable        | yes      |
| `collection list`        | bullet list           | no       |
| `collection create/delete` | success line        | no       |
| `index add/delete`       | success line          | no       |
| `schema show`            | human-readable table  | no       |

`search --json` emits the full agent result:

```json
{
  "nl_query": "products under $100 with 4+ star rating",
  "solr_query": "*:*",
  "num_found": 23,
  "docs": [ { "id": "SKU-9912", "name": "Wireless Mouse", ... } ],
  "explanation": "I filtered by rating ≥ 4, price < 100, ...",
  "iterations": 2
}
```

### Global flags

| Flag                | Description                                            | Default                  |
| ------------------- | ------------------------------------------------------ | ------------------------ |
| `--dev` / `--prod`  | Target environment (mutually exclusive)                | `--dev`                  |
| `--solr-url`        | Solr base URL (`/solr` is appended automatically)      | `http://localhost:8983`  |
| `-c, --collection`  | Default collection for the command                     | `default`                |
| `--api-key`         | Anthropic API key (or set `ANTHROPIC_API_KEY`)         | from env                 |
| `-v, --verbose`     | Print agent tool calls and Claude messages             | `false`                  |
| `--max-iterations`  | Maximum agentic refinement loop iterations             | `5`                      |

### Environment variables

| Variable             | Read by    | Purpose                                                |
| -------------------- | ---------- | ------------------------------------------------------ |
| `ANTHROPIC_API_KEY`  | CLI        | Anthropic API key for the `search` agent.              |
| `SOLR_URL`           | integration tests | Used by `tests/integration` to find the test Solr. The CLI itself does **not** read `SOLR_URL` — use `--solr-url`. |

Precedence for the API key: `--api-key` flag > `ANTHROPIC_API_KEY` env var.

### Cost and model

The agent uses `claude-opus-4-7` (pinned in `internal/agent/agent.go`). A
typical natural-language search runs 2–4 tool-call iterations and consumes a
few thousand tokens — costs are dominated by the schema payload returned by
`get_schema`. If your schema is very large, the first iteration's input
tokens grow proportionally; consider trimming fields or splitting the
collection.

There is no built-in caching or batching yet — every `asksolr search`
invocation is an independent agent session.

### Security notes

- **Never bake your API key into a Dockerfile or commit it.** Use the
  `ANTHROPIC_API_KEY` env var.
- The `--prod` confirmation prompts for `collection delete` and
  `index delete` are advisory — they protect against fat-fingered local
  invocations, not against scripted misuse. Restrict who has the production
  Solr URL.
- `asksolr` does not authenticate to Solr beyond the URL. If your Solr is
  behind basic auth, run it behind a local proxy or extend
  `internal/solr.Client` to add credentials.
- The agent will happily generate `*:*` if it can't find a better match.
  Pair the CLI with a Solr-side query log if you need an audit trail.

### Troubleshooting

**`Anthropic API key required: use --api-key or set ANTHROPIC_API_KEY`** —
the `search` command needs the key. Other commands (`collection`, `index`,
`schema`) do not.

**`failed to list collections: Get "...": connection refused`** — Solr isn't
running, or `--solr-url` points at the wrong host. Try
`curl http://localhost:8983/solr/admin/ping`.

**Search returns 0 documents but you expected matches** — rerun with `-v` to
see the actual Lucene query. Common causes: case-sensitive text field, field
not indexed, date format mismatch (Solr wants `2024-01-01T00:00:00Z`).

**`max iterations reached`-style behavior** — the agent stopped before
finding a result. Bump `--max-iterations`, or rephrase the query to be more
specific. Verbose mode usually makes the cause obvious.

**`schema show` lists 0 fields** — the collection exists but has no managed
schema yet. Index a document first; Solr will auto-create fields if you're
on a default config.

### Limitations

- One collection per `search` call. Cross-collection queries aren't
  supported.
- No streaming output — the CLI waits for the full agent loop, then prints.
- No facet, highlighting, or grouping support in the agent tools yet (the
  Solr client itself can do these; they aren't exposed to Claude).
- The agent does not currently call `update` / `delete` tools — `search` is
  read-only by design.
- Lucene query escaping is left to Claude. If your data contains characters
  like `:`, `(`, `[`, the agent may need a hint in the query.

---

## Getting started for developers

### Prerequisites

- **Go 1.25+** (see `go.mod`)
- **Docker + Docker Compose** for the local Solr container used by
  integration tests
- **An Anthropic API key** if you want to exercise the `search` command end
  to end
- **(Optional) `staticcheck`** for `make lint`:
  `go install honnef.co/go/tools/cmd/staticcheck@latest`
- **(Optional) `goreleaser`** for local release dry-runs

### Repo layout

```
asksolr/
├── main.go                       # entrypoint — delegates to cmd.Execute
├── cmd/                          # cobra commands
│   ├── root.go                   #   global flags + persistent pre-run
│   ├── search.go                 #   `search` — agentic NL search
│   ├── collection.go             #   `collection list|create|delete`
│   ├── index.go                  #   `index add|delete`
│   └── schema.go                 #   `schema show`
├── internal/
│   ├── agent/
│   │   ├── agent.go              # agent loop + tool dispatch
│   │   ├── agent_test.go         # unit tests (mocked SDK)
│   │   └── prompts.go            # system prompt for the agent
│   ├── config/
│   │   └── config.go             # Config struct + Env type
│   └── solr/
│       ├── client.go             # thin Solr HTTP client
│       ├── client_test.go        # unit tests (httptest)
│       └── types.go              # Solr response types
├── tests/integration/            # integration tests (build tag: integration)
│   ├── helpers_test.go           #   shared setup/teardown
│   └── search_test.go            #   end-to-end query tests
├── docker-compose.yml            # local Solr 9.7
├── Makefile                      # build / test / lint targets
├── .goreleaser.yaml              # release config (binaries + Homebrew tap)
└── .github/workflows/            # CI + release workflows
    ├── ci.yml                    #   lint, vet, unit + integration tests
    └── release.yml               #   goreleaser on tag push
```

### Architecture

```
┌────────────────┐    ┌──────────────────────┐    ┌──────────────┐
│ cobra commands │ →  │ agent.Agent          │ →  │ Anthropic    │
│ (cmd/*.go)     │    │ (internal/agent)     │    │ Messages API │
└────────┬───────┘    └──────────┬───────────┘    └──────────────┘
         │                       │
         │                       │ tool calls
         │                       ▼
         │            ┌──────────────────────┐
         └──────────► │ solr.Client          │ ───► Apache Solr
                      │ (internal/solr)      │      HTTP API
                      └──────────────────────┘
```

- `cmd/` parses flags, prints output, never calls the Anthropic SDK
  directly.
- `internal/agent` owns the Claude agentic loop (`Agent.Search`) and
  dispatches the two tool calls (`get_schema`, `execute_query`).
- `internal/solr` is a plain HTTP client — no agent knowledge, no CLI
  knowledge. Easy to unit-test with `httptest`.

### Build and run

```sh
make build                                   # produces dist/asksolr
./dist/asksolr --help
make run ARGS="collection list --dev"        # build + run with args
```

Versioning: `make build` injects `git describe --tags --always --dirty` into
the binary via `-ldflags -X .../cmd.version=$(VERSION)`. (The variable
itself isn't currently exposed in `cmd/` — feel free to add one.)

### Tests

```sh
make test-unit                  # fast — pure Go unit tests, no network
make test-integration           # requires Solr running on :8983
make test-all                   # unit + boot Solr + integration + teardown
```

The integration suite is gated by `//go:build integration`, so plain
`go test ./...` only runs unit tests. Integration tests read `SOLR_URL`
(default `http://localhost:8983/solr`).

```sh
SOLR_URL=http://localhost:8983/solr go test -tags integration ./tests/integration/...
```

Coverage:

```sh
go test -race -coverprofile=cover.out ./internal/... ./cmd/...
go tool cover -html=cover.out
```

### Local Solr

```sh
make solr-up                    # docker compose up -d
make solr-wait                  # block until /admin/ping is healthy
make solr-logs                  # tail container logs
make solr-down                  # docker compose down -v  (drops the volume)
```

The compose file pre-creates a `yogi_test` collection on Solr 9.7. To talk
to it from the CLI:

```sh
asksolr collection list --solr-url http://localhost:8983
asksolr schema show -c yogi_test --solr-url http://localhost:8983
```

### Lint, format, tidy

```sh
make fmt                  # gofmt -w -s .
make vet                  # go vet ./...
make lint                 # staticcheck (install separately)
make tidy                 # go mod tidy
```

CI fails if any source file isn't `gofmt`'d, if `go vet` reports anything,
or if `go.mod`/`go.sum` aren't tidy.

### Adding a new Claude tool

The agent's tool list lives in `internal/agent/agent.go` (`buildTools` +
`dispatchTool`). To add, say, a `facet_query` tool:

1. **Declare the tool** in `buildTools` — name, description, JSON-Schema
   input. Be precise in the description; Claude reads it to decide when to
   call the tool.
2. **Handle it** in `dispatchTool` with a new `case`. Unmarshal the input
   JSON, call a method on `internal/solr.Client`, and return
   `(jsonOutput, isError, solrQuery)`. Errors are returned as
   `{"error": "..."}` with `isError=true`.
3. **Tell Claude about it** in `internal/agent/prompts.go` if the system
   prompt needs to mention when to use the new tool.
4. **Add unit tests** in `internal/agent/agent_test.go` with a mocked
   Anthropic client.

### Adding a new CLI command

1. Create `cmd/<name>.go` with a `cobra.Command`.
2. Register it in `cmd/root.go`'s `init()` via `rootCmd.AddCommand(...)`.
3. Use `solrClient` (already initialized in `root.go`'s `PersistentPreRunE`)
   rather than calling Solr directly.
4. Reuse the `--collection` flag pattern from existing commands so users
   don't have to relearn flags.
5. For destructive operations, gate them with `cfg.IsProd()` + a typed `yes`
   prompt — see `cmd/collection.go:delete` for the canonical pattern.

### Project conventions

- Anthropic SDK calls go through `internal/agent`; commands never call the
  SDK directly.
- HTTP calls to Solr go through `internal/solr.Client`; commands and the
  agent should not build raw URLs.
- Errors bubble up with `fmt.Errorf("...: %w", err)` so the CLI top-level
  prints a single clean line.
- All user-facing output uses `github.com/fatih/color` — no raw ANSI
  escapes.
- All I/O takes a `context.Context` as the first argument.
- New dependencies require an explanation in the PR (stdlib first; existing
  deps second; new deps last).

### Releasing

Releases are driven by [GoReleaser](https://goreleaser.com) (see
`.goreleaser.yaml`) and triggered by pushing a `vX.Y.Z` tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `release` GitHub Action then:

1. Cross-compiles binaries for linux/darwin/windows × amd64/arm64.
2. Attaches the archives + `checksums.txt` to a GitHub Release with an
   auto-generated changelog.
3. Bumps the Homebrew formula in `CodeYogiCo/homebrew-tap` so
   `brew install CodeYogiCo/tap/asksolr` picks up the new version.

**One-time setup before the first release:**

- Create the `CodeYogiCo/homebrew-tap` repo with a `Formula/` directory
  (an empty repo with that folder is fine).
- Add a `HOMEBREW_TAP_TOKEN` repository secret on `CodeYogiCo/asksolr` — a
  classic PAT with `contents: write` scope on the tap repo. The default
  `GITHUB_TOKEN` cannot push to another repo.

**Dry run locally:**

```sh
goreleaser release --snapshot --clean --skip=publish
ls dist/        # inspect the artifacts that would be uploaded
```

### Contributing

1. Fork and create a feature branch from `main`.
2. `make fmt vet test-unit` must pass.
3. If you change the Solr client or agent loop, run `make test-all` against
   the docker-compose Solr.
4. Open a PR — CI (`.github/workflows/ci.yml`) runs lint + unit + integration
   on every push and PR.
5. Keep commits small and focused; we prefer Conventional Commits-style
   subject lines (`feat:`, `fix:`, `docs:`, `chore:`, ...).

See [AGENTS.md](AGENTS.md) for guidance specifically aimed at AI coding
agents working in this repo.

### References

- [Apache Solr docs](https://solr.apache.org/guide/solr/latest/) —
  especially the query syntax reference.
- [Anthropic Messages API](https://docs.claude.com/en/api/messages) and
  [tool use guide](https://docs.claude.com/en/docs/build-with-claude/tool-use).
- [Cobra](https://github.com/spf13/cobra) — CLI framework.
- [GoReleaser Homebrew tap docs](https://goreleaser.com/customization/homebrew/).
