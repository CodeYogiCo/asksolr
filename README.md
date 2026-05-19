# asksolr

> Agentic Apache Solr CLI — query Solr in plain English using Claude.

`asksolr` is a Kubernetes-style command-line tool that lets you talk to a Solr
cluster using natural language. Claude inspects the collection schema, builds
the appropriate Solr (Lucene) query, executes it, and refines the query until
it returns useful results.

```
$ asksolr search "products under $50 that are in stock and have 4+ star rating" -c products
🔍 Query: products under $50 that are in stock and have 4+ star rating
   Collection: products | env: dev

✓  Found 23 document(s)  [Solr query: rating:[4 TO *]]
   iterations: 2
  [1]
    id:                 SKU-9912
    name:               Wireless Mouse
    price:              19.99
    rating:             4.5
    ...
```

---

## How to use it

### Install

**Homebrew (macOS / Linux):**

```sh
brew install CodeYogiCo/tap/asksolr
```

**From a release binary:** download the archive for your OS/arch from the
[Releases page](https://github.com/CodeYogiCo/asksolr/releases) and drop the
`asksolr` binary somewhere on your `$PATH`.

**From source:**

```sh
git clone https://github.com/CodeYogiCo/asksolr.git
cd asksolr
make build
sudo mv dist/asksolr /usr/local/bin/
```

### Configure

`asksolr` needs two things:

1. **A reachable Solr cluster.** By default it talks to `http://localhost:8983`.
   Override with `--solr-url` or by pointing `--dev` / `--prod` at your own
   environment defaults.
2. **An Anthropic API key.** Set it via the `ANTHROPIC_API_KEY` environment
   variable or pass `--api-key` on the command line.

```sh
export ANTHROPIC_API_KEY=sk-ant-...
```

### Commands

#### Agentic natural-language search

```sh
asksolr search "<plain-english query>" [--collection NAME] [--json]
```

Examples:

```sh
# Find products by price + rating — Claude builds the Solr query for you
asksolr search "products under $100 with 4+ star rating" -c products

# Range + date filters
asksolr search "news articles about climate published in 2024" -c articles

# Get raw JSON output (great for piping into jq)
asksolr search "users who signed up last week and haven't verified email" --json | jq
```

Use `-v` / `--verbose` to see each tool call Claude makes against Solr, and
`--max-iterations` to cap how many query-refinement rounds the agent runs.

#### Collections

```sh
asksolr collection list                              # list all collections
asksolr collection create products --shards 2 -r 2   # create a collection
asksolr collection delete products                   # delete (asks for confirmation in --prod)
```

#### Indexing documents

```sh
# Index a JSON file (array of objects, or a single object)
asksolr index add ./data.json --collection products

# Delete documents matching a Solr query
asksolr index delete "price:[* TO 0]" --collection products
```

#### Inspecting the schema

```sh
asksolr schema show --collection products
```

### Environments

Every command accepts `--dev` (default) or `--prod`. The flag is printed in
the banner so you always know which cluster you're hitting. Destructive
operations against `--prod` require a typed `yes` confirmation.

### Global flags

| Flag                | Description                                            |
| ------------------- | ------------------------------------------------------ |
| `--dev` / `--prod`  | Target environment (mutually exclusive)                |
| `--solr-url`        | Override the Solr base URL                             |
| `-c, --collection`  | Default collection for the command                     |
| `--api-key`         | Anthropic API key (or set `ANTHROPIC_API_KEY`)         |
| `-v, --verbose`     | Print agent tool calls and Claude messages             |
| `--max-iterations`  | Maximum agentic refinement loop iterations (default 5) |

---

## Getting started for developers

### Prerequisites

- **Go 1.25+** (see `go.mod`)
- **Docker + Docker Compose** (for the local Solr container used by
  integration tests)
- **An Anthropic API key** if you want to exercise the `search` command end to
  end

### Repo layout

```
asksolr/
├── main.go                   # entrypoint — delegates to cmd.Execute
├── cmd/                      # cobra commands (search, collection, index, schema)
├── internal/
│   ├── agent/                # Claude agentic loop + tool dispatch
│   ├── config/               # CLI config / env flags
│   └── solr/                 # thin Solr HTTP client
├── tests/integration/        # integration tests (build tag: integration)
├── docker-compose.yml        # local Solr 9.7
└── Makefile                  # build / test / lint targets
```

### Build and run

```sh
make build               # produces dist/asksolr
make run ARGS="collection list --dev"
```

### Tests

```sh
make test-unit           # fast — pure Go unit tests
make test-integration    # requires Solr running on :8983
make test-all            # unit + boot Solr + integration + teardown
```

The integration suite is gated by the `integration` build tag, so plain
`go test ./...` will only run unit tests.

### Local Solr

```sh
make solr-up             # docker compose up -d
make solr-wait           # block until /admin/ping is healthy
make solr-logs           # tail container logs
make solr-down           # docker compose down -v
```

### Lint, format, tidy

```sh
make fmt                 # gofmt -w -s .
make vet                 # go vet ./...
make lint                # staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
make tidy                # go mod tidy
```

### Adding a new Claude tool

The agent's tool list lives in `internal/agent/agent.go` (`buildTools` +
`dispatchTool`). To add a capability:

1. Declare the tool name, description, and JSON-Schema input in `buildTools`.
2. Add a `case` to `dispatchTool` that unmarshals the input, calls the
   `internal/solr` client, and returns a `(jsonOutput, isError, solrQuery)`
   triple.
3. Update `systemPrompt` in `internal/agent/prompts.go` if Claude needs new
   guidance on when to use it.

### Project conventions

- Anthropic SDK calls go through `internal/agent`; commands should never call
  the SDK directly.
- HTTP calls to Solr go through `internal/solr.Client`; commands and the agent
  should not build raw URLs.
- Errors bubble up with `fmt.Errorf("...: %w", err)` so the CLI top-level can
  print a single clean line.

### Releasing

Releases are driven by [GoReleaser](https://goreleaser.com) (see
`.goreleaser.yaml`) and triggered by pushing a `vX.Y.Z` tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `release` GitHub Action then:

1. Cross-compiles binaries for linux/darwin/windows × amd64/arm64.
2. Attaches the archives + `checksums.txt` to a GitHub Release.
3. Bumps the Homebrew formula in `CodeYogiCo/homebrew-tap` so
   `brew install CodeYogiCo/tap/asksolr` picks up the new version.

One-time setup before the first release:

- Create the `CodeYogiCo/homebrew-tap` repo (must contain a `Formula/`
  directory; an empty repo with that folder is fine).
- Add a `HOMEBREW_TAP_TOKEN` repository secret on `CodeYogiCo/asksolr` — a
  classic PAT with `contents: write` scope on the tap repo.

To dry-run the release locally:

```sh
goreleaser release --snapshot --clean --skip=publish
```

### Contributing

1. Fork and create a feature branch.
2. `make fmt vet test-unit` must pass.
3. If you change the Solr client or agent loop, run `make test-all` against
   the docker-compose Solr.
4. Open a PR — CI (see `.github/workflows/ci.yml`) will run the same checks.

See [AGENTS.md](AGENTS.md) for guidance specifically aimed at AI coding agents
working in this repo.
