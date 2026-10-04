# The Council Creator

**A room full of Virtual experts, colleagues, critics, and customers who review your work, argue with each other, and force better decisions.**

AI tools are eager to please. They validate your ideas and move fast. No one asks the hard questions, and your own judgment fades.

A Council fixes this. It is a team of AI reviewers modeled on real people, real roles, and real perspectives. You choose who sits on it; your AI builds each persona from public material. Personas based on real people carry the **Virtual** prefix ("Virtual Jane Doe"): a model of their public positions, not the person, and not affiliated with or endorsed by them. Mix them with roles like a security engineer or an SRE, and with the customers you build for.

Council ships no people. It ships the engine, the persona format, and the instructions your AI follows. The one exception is the author's own persona, Virtual Luc Perussault-Diallo: every new council starts with it as a ready-to-use example, and you can remove it like any other member.

Members are picked to disagree. Each one reads what the others said, then pushes back, adds what they missed, or changes their mind. You get the debate, not a consensus. You still make the call.

Code review is one use. Product, writing, architecture, and security decisions are others.

## Get Started

**Tell your AI assistant:**

> Grab https://raw.githubusercontent.com/luuuc/council/main/AGENT-INSTALL.md and get me set up

That's it. Works with Claude Code, OpenCode, or any AI that can fetch URLs.

After setup, use `/council <topic>` to convene your experts.

## Create Your Council

Your council is yours. Add whoever helps you do better work:

- **Virtual experts**: people you choose, modeled on their public talks, writing, and decisions
- **Roles**: a security engineer, an SRE, a product-minded CTO
- **Your customers**: the user types you are actually building for
- **Your team**: your CTO, your tech lead, your mentor

In your AI tool:

```
/council-assemble                    # Your AI reads the project, proposes members with reasons, you choose
/council-add Jane Doe                # Add one person, researched from public work
/council-add an SRE                  # Add a role
/council-add freelancers who bill by the hour
                                     # Add a customer, built from evidence (docs, support, analytics)
```

Your AI builds each persona and saves it with `council add`. People need public sources and get a no-affiliation disclaimer; customers need evidence.

## How It Works

```
Your Council                         Your AI Tool
┌─────────────────┐                  ┌─────────────────┐
│ Virtual J. Doe  │                  │ /council        │
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
council init               # Creates .council/ and installs the slash commands
council assemble           # The brief your AI follows to build the council
council sync               # Syncs changes to your AI tool
```

## Commands

| Command | What it does |
|---------|--------------|
| `council init` | Create `.council/` and install the slash commands for your AI tool |
| `council assemble` | Print the brief your AI follows to propose and build members |
| `council add <file \| ->` | Check and save a persona your AI wrote (person, role, or customer) |
| `council list` | See your council members |
| `council remove <id>` | Remove an expert |
| `council sync` | Sync to your AI tool |
| `council export` | Export as portable markdown |

## Review

Type `/council` in Claude Code or OpenCode and point it at files, your current changes, or a question. Everyone sits in the same room: your AI tool writes the whole debate in one pass, following the room prompt Council gives it, and Council checks it and shows it.

Members speak in pack order. Each one gives a verdict (pass / comment / block / escalate), notes, and replies to the members before them (agree, disagree, adds). Then the earlier members get a final word on what came after them, and may change their verdict. A neutral moderator closes with **where they disagree** and **what you need to decide**. Council doesn't recommend an outcome: you make the call. Reviews are saved in `.council/reviews/`.

Under the hood, `/council` runs:

```bash
git diff main | council review --pack code             # prints the room prompt
council review --pack code --record answer.json        # checks the AI's answer, shows it, saves it
```

**Councils of Councils.** `--councils product,security` puts several packs in the room. Each council debates, then each council's spokesperson challenges the others' conclusions, and a moderator lists where the councils disagree and what you need to decide.

**Unattended.** `council review --api` sends the room prompt to a model API with your own key (Anthropic, OpenAI, GitHub Models, Ollama) and shows the review. The GitHub Action uses it.

## Packs

Packs group your members into named councils (`product`, `security`) for targeted reviews:

```bash
council packs list                         # See your packs
council packs show product                 # See members
council packs create product               # Create a pack
council packs add product jane-doe         # Add a member to it
```

Mix members with different incentives so they don't agree by default. Council ships no packs: they hold your members.

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
- `council_room` / `council_record` — the room prompt the client's model answers in one pass, and recording that debate (no API key needed, e.g. Claude Desktop)
- `council_assemble` / `council_add` — the brief for building members, and saving each one
- `council_list` — list pack members

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

**How it works:** The Action fetches the PR diff, runs Council with the specified pack from the `.council/` committed in your repository, and posts a PR Review with inline comments + a Check Run status badge.

**LLM selection (automatic):**

| Secret set | Provider | Model | Cost |
|---|---|---|---|
| `ANTHROPIC_API_KEY` | Anthropic | `claude-sonnet-4-6` | BYOK |
| `OPENAI_API_KEY` | OpenAI | `gpt-4.1` | BYOK |
| Neither | GitHub Models | `gpt-4.1-mini` | Free (150 req/day) |

The Action reviews with the council committed in the repo: commit `.council/` after assembling it. Each review is one request (two if the first answer needs fixing).

**Free tier limits:** 150 requests/day. The free tier also caps how much text one request can carry, so large PR diffs or big councils may fail there; use an API key for those.

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
