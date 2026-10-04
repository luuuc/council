# Contributing to council

## Quick Start

```bash
git clone https://github.com/luuuc/council.git
cd council
make ci    # Build + test + lint (run before every PR)
```

Requires Go 1.23 or later. `make build` puts the binary in `bin/council`; don't `go build` to the project root. `make install` copies it to `/usr/local/bin/council`.

```bash
make test                          # All tests
go test -v ./internal/review/...   # One package
make lint                          # golangci-lint (installed on first run)
```

## One rule: Council ships no people

No real person's name or persona goes in the binary, repo, tests, or docs. Use placeholders: "Virtual Jane Doe" in docs, "Virtual Ada", "Ben", "Cleo" in tests. The one exception is the author's own persona in `internal/expert/defaults/`, shipped with his consent.

## Code Structure

```
council/
├── cmd/council/          # CLI entry point
├── internal/
│   ├── adapter/          # Per-tool files: Claude Code, OpenCode, generic AGENTS.md
│   ├── brief/            # The assembly brief the user's AI follows
│   ├── cmd/              # Cobra commands
│   ├── config/           # .council/config.yaml, API provider detection
│   ├── expert/           # Persona format, validation, the default member
│   ├── fs/               # File helpers
│   ├── mcp/              # MCP server (stdin/stdout JSON-RPC)
│   ├── pack/             # Named groups of members
│   ├── review/           # Room prompt, answer checking, rendering, API backend
│   └── sync/             # Writes members and commands to each tool
├── action/               # GitHub Action
└── install.sh            # Installer
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for how the pieces fit.

## Adding an AI tool

Implement the `Adapter` interface in `internal/adapter/` and call `Register()` in `init()`. Add tests next to the others in `internal/adapter/` and `internal/sync/`.

## PR Guidelines

- `make ci` passes
- Tests for new behavior (table-driven where there are several cases)
- Focused changes; docs updated when behavior changes

Questions: https://github.com/luuuc/council/issues
