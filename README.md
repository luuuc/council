# The Council Creator

**A room full of Virtual experts, colleagues, critics, and customers who review your work, argue with each other, and force better decisions.**

AI tools are eager to please. They validate your ideas and move fast. No one asks the hard questions, and your own judgment fades.

A Council fixes this. It is a team of AI reviewers modeled on real people, real roles, and real perspectives. Personas based on real people carry the **Virtual** prefix: Virtual DHH, Virtual Boris Cherny, Virtual Jason Fried. Mix them with roles like a security engineer or an SRE, and with the customers you build for.

Members are picked to disagree. Each one reads what the others said, then pushes back, adds what they missed, or changes their mind. You get the debate, not a consensus. You still make the call.

Code review is one use. Product, writing, architecture, and security decisions are others.

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
council add "Boris Cherny"           # Not in the library: researches Virtual Boris Cherny
council add --role "SRE"             # A role, with that role's incentives
council add --customer "freelancers who bill clients by the hour"
                                     # A customer: reacts as a user, not a reviewer
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
council start    # Detects your stack, suggests experts plus people who will disagree with them, lets you pick
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
| `council start` | Setup: detect stack, pick your council, sync (`--yes` or no terminal: no questions) |
| `council add "Name"` | Add from the library, research a real person, or create custom |
| `council add --role "SRE"` | Add a role (SRE, security engineer, product-minded CTO) |
| `council add --customer "..."` | Add a customer persona from a description of your users |
| `council add --interview` | AI-assisted persona creation |
| `council add --from ID` | Fork existing persona as starting point |
| `council list` | See your council members |
| `council remove <id>` | Remove an expert |
| `council sync` | Sync to your AI tool |
| `council personas` | Browse the curated library |
| `council export` | Export as portable markdown |

## Review

Experts review one at a time, in pack order. Each one reads the earlier reviews, then disagrees, backs them up, or adds what they missed:

```bash
git diff main | council review --pack go
council review --pack rails --file app/models/user.rb --json
```

Each expert returns a verdict (pass / comment / block / escalate), notes, and replies to the experts before them. Then the earlier experts get a final word on what came after them, and may change their verdict. A neutral moderator closes with **where they disagree** and **what you need to decide**. Council doesn't recommend an outcome: you make the call.

A review makes about two LLM calls per expert (review, final word) plus one for the moderator. `--quick` skips the final word and the moderator. `--mode collective` makes a single call that plays every expert at once: cheapest, but the debate is simulated.

Each expert's review prints as soon as it's done, so you watch the debate unfold.

Works with any LLM backend: runs an AI CLI headless (`claude -p`, `opencode run`, `codex exec`) on your existing subscriptions, or calls APIs directly (Anthropic, OpenAI, Ollama). The first CLI found is used; set `ai.command` in `.council/config.yaml` to pick one, and `--model` (or `ai.model`) to pick its model, e.g. `opencode` with `kimi-code-plan-global/k3`.

**Mix models.** Models from different labs disagree more honestly than one model playing everyone. `--mix` spreads members across CLIs, round-robin, and each member keeps their model for the whole review:

```bash
git diff main | council review --pack go --mix "claude,codex,opencode=kimi-code-plan-global/k3"
```

Or set it once in `.council/config.yaml`:

```yaml
ai:
  mix:
    - command: claude
    - command: codex
    - command: opencode
      model: kimi-code-plan-global/k3
```

**Councils of Councils.** `council review --councils product,security,code --file plan.md` runs several packs on the same submission. Each council debates on its own, then each council's spokesperson challenges the others' conclusions, and a moderator lists where the councils disagree and what you need to decide.

`/council` in Claude Code and OpenCode runs the same review: point it at files, your current changes, or a question, and it presents the debate and what you need to decide.

## Packs

Packs are reusable groupings of experts for targeted reviews:

```bash
council packs list                         # See all packs
council packs show go                      # See members
council packs create my-pack               # Create custom pack
council packs add my-pack kent-beck        # Add expert to pack
```

Built-in packs: `code` (any language), `product`, `growth`, `security`, `architecture`, `go`, `rails`, `writing`. Each mixes members with different incentives so they don't agree by default. Custom packs override built-ins with the same name.

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

Exposes these tools over stdin/stdout JSON-RPC, plus a `council` prompt for prompt menus. See [docs/integrations.md](docs/integrations.md) for Claude Desktop setup.
- `council_review` — sequential council review, returns structured verdict with replies
- `council_convene` / `council_turn` — the same review with the client's model taking each member's turn (no AI CLI or API key needed, e.g. Claude Desktop)
- `council_add_persona` — save a Virtual persona the client researched
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

**Free tier limits:** 150 requests/day. On the free tier the Action uses `--mode collective` (one request per review) by default; set `mode: sequential` for a real debate at one request per expert (about 25 reviews/day with a 6-expert pack). With an API key, sequential is the default. Files over 8K tokens are skipped. Max 25 files per review. Per-file review means cross-file issues are invisible — use BYOK for larger context.

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
