# The Council Creator

**A room full of Virtual experts, colleagues, critics, and customers who review your work, argue with each other, and force better decisions.**

AI tools are eager to please. They validate your ideas and move fast. No one asks the hard questions, and your own judgment fades.

A Council fixes this. It is a team of AI reviewers modeled on real people, real roles, and real perspectives. Personas based on real people carry the **Virtual** prefix: Virtual DHH, Virtual Boris Cherny, Virtual Jason Fried. Mix them with roles like a security engineer or an SRE, and with the customers you build for.

Members are picked to disagree. Each one reads what the others said, then pushes back, adds what they missed, or changes their mind. You get the debate, not a consensus. You still make the call.

Code review is one use. Product, writing, architecture, and security decisions are others. See [docs/direction.md](docs/direction.md) for where Council is heading.

## Get Started

**Tell your AI assistant:**

> Grab https://raw.githubusercontent.com/luuuc/council/main/AGENT-INSTALL.md and get me set up

That's it. Works with Claude Code, OpenCode, or any AI that can fetch URLs.

After setup, use `/council <topic>` to convene your experts.

## Create Your Council

Your council is yours. Add whoever helps you do better work:

- **Virtual experts**: personas built from real people's public talks, writing, and decisions
- **Roles**: a security engineer, an SRE, a product-minded CTO
- **Your customers**: the user types you are actually building for
- **Your team**: your CTO, your tech lead, your mentor

```bash
council add "Kent Beck"             # Adds Virtual Kent Beck from the library
council add "My Tech Lead"           # Create custom persona
/council-add a security expert       # AI-assisted discovery
```

## How It Works

```
Your Council                         Your AI Tool
┌─────────────────┐                  ┌─────────────────┐
│ Virtual DHH     │                  │ /council        │
│ Security Eng.   │───── sync ──────▶│ /council-add    │
│ Your CTO        │                  │ /council-remove │
│ Your Customer   │                  │                 │
└─────────────────┘                  └─────────────────┘
```

Councils live in your project (`.council/experts/`), sync to your AI tool's native format, and become slash commands you invoke anytime.

## Manual Installation

```bash
# Direct download
curl -fsSL https://raw.githubusercontent.com/luuuc/council/main/install.sh | sh

# Or via Go
go install github.com/luuuc/council/cmd/council@latest
```

Then:

```bash
council start    # Zero-config setup (creates council, detects stack, adds experts)
```

Or step by step:

```bash
council init     # Creates .council/ directory
council add "Kent Beck"   # Add experts one by one
council sync     # Syncs to your AI tool
```

## Commands

| Command | What it does |
|---------|--------------|
| `council start` | Zero-config setup (init + detect + add experts + sync) |
| `council add "Name"` | Add expert from library or create custom |
| `council add --interview` | AI-assisted persona creation |
| `council add --from ID` | Fork existing persona as starting point |
| `council list` | See your council members |
| `council remove <id>` | Remove an expert |
| `council sync` | Sync to your AI tool |
| `council personas` | Browse the curated library |
| `council export` | Export as portable markdown |

## Review

Run collective reviews where all experts review together and react to each other's perspectives:

```bash
git diff main | council review --pack go
council review --pack rails --file app/models/user.rb --json
```

Each expert returns a verdict (pass / comment / block / escalate). The tension between perspectives produces richer, more nuanced reviews with agreements, disagreements, and a final recommendation. Falls back to per-expert review for small-context models.

Works with any LLM backend — spawns CLI subprocesses (`claude`, `opencode`) or calls APIs directly (Anthropic, OpenAI, Ollama).

## Packs

Packs are reusable groupings of experts for targeted reviews:

```bash
council packs list                         # See all packs
council packs show go                      # See members
council packs create my-pack               # Create custom pack
council packs add my-pack kent-beck        # Add expert to pack
```

Built-in packs: `go`, `rails`, `writing`. Custom packs override built-ins with the same name.

## MCP Server

Use Council as a tool in any MCP-capable AI tool:

```json
{
  "mcpServers": {
    "council": {
      "command": "council",
      "args": ["mcp"]
    }
  }
}
```

Exposes three tools over stdin/stdout JSON-RPC:
- `council_review` — collective review, returns structured verdict
- `council_list` — list pack members (no LLM calls)
- `council_explain` — expand on a review note with expert reasoning

## GitHub Action

Get Council reviews on every pull request — zero config, zero cost:

```yaml
# .github/workflows/council-review.yml
name: Council Review
on:
  pull_request:
    types: [opened, synchronize, ready_for_review]

permissions:
  models: read
  pull-requests: write
  contents: read
  checks: write

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: luuuc/council/action@v1
        with:
          pack: code
```

**How it works:** The Action fetches the PR diff, runs Council with the specified pack, and posts a PR Review with inline comments + a Check Run status badge.

**LLM selection (automatic):**

| Secret set | Provider | Model | Cost |
|---|---|---|---|
| `ANTHROPIC_API_KEY` | Anthropic | `claude-sonnet-4-6` | BYOK |
| `OPENAI_API_KEY` | OpenAI | `gpt-4.1` | BYOK |
| Neither | GitHub Models | `gpt-4.1-mini` | Free (150 req/day) |

**Free tier limits:** ~15 PR reviews/day (10 files each). Files over 8K tokens are skipped. Max 25 files per review. Per-file review means cross-file issues are invisible — use BYOK for larger context.

See [`action/examples/`](action/examples/) for more workflow examples.

## Supported AI Tools

| Tool | Integration |
|------|-------------|
| GitHub Actions | PR reviews on every pull request |
| Claude Code | Slash commands + agents + MCP |
| Cursor | MCP |
| Claude Desktop | MCP |
| OpenCode | Agents |
| Others | `council export` for portable markdown |

## Philosophy

- **Real personas over invented ones.** Built from public talks, writing, principles, and decisions.
- **Friction by design.** Members hold different positions and should not agree by default.
- **Sequential debate.** Each member sees the previous reviews and reacts to them.
- **Customers have a seat.** Product reviews include the people expected to use the product.
- **The human decides.** Council exposes disagreements, blind spots, and trade-offs. It does not vote for you.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup.

## License

MIT
