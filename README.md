# The Council Creator

**A room full of Virtual experts, colleagues, critics, and customers who review your work, argue with each other, and force better decisions.**

AI tools are eager to please. They validate your ideas and move fast. No one asks the hard questions, and your own judgment fades.

A Council fixes this. It is a team of AI reviewers modeled on real people, roles, and the customers you build for. You choose who sits on it, and your AI builds each persona. Members are picked to disagree: each one reads what the others said, then pushes back, adds what they missed, or changes their mind. You get the debate, not a consensus. You still make the call.

Code review is one use. Product, writing, architecture, and security decisions are others.

## Three steps

**1. Install.** Tell your AI tool (Claude Code, OpenCode, Codex):

> Grab https://raw.githubusercontent.com/luuuc/council/main/AGENT-INSTALL.md and set me up

**2. Assemble your council.**

```
/council-assemble
```

Your AI reads the project (code, docs, goals, support tickets), proposes members with a reason for each, and you pick. It builds each persona; Council checks it and saves it in `.council/experts/`.

**3. Convene it.**

```
/council the pricing change in docs/pricing.md
/council my current changes
/council should we drop the free tier? with the product and security councils
```

Your AI writes the whole debate in one pass, following the room prompt Council gives it. Council checks the answer and shows each member's verdict, notes, and replies, their final words, **where they disagree**, and **what you need to decide**. Reviews are saved in `.council/reviews/`.

## Who sits on a council

- **Virtual people**: people with documented public positions, modeled on their talks, writing, and decisions. They carry the **Virtual** prefix ("Virtual Jane Doe") and a disclaimer: a model of their public positions, not affiliated with or endorsed by them. Positions read from their work, rather than stated by them, are kept apart.
- **Roles**: a security engineer, an SRE, a CFO.
- **Customers**: the users you build for, drawn from evidence (support threads, interviews, analytics), not just a description.

Council ships no people. It ships the engine, the persona format, and the instructions your AI follows. The one exception is the author's own persona, Virtual Luc Perussault-Diallo: every new council starts with it as an example, and `council remove luc-perussault-diallo` takes it out.

Add one member with `/council-add <who>`; remove one with `/council-remove <id>`.

## How a review runs

Everyone sits in the same room:

1. Members speak in order. Each gives a verdict (pass, comment, block, escalate), notes, and replies to the earlier members (agree, disagree, adds).
2. Earlier members get a final word on what came after them, and may change their verdict.
3. A neutral moderator lists where they disagree, what you need to decide, and what nobody disputed. No recommendation: you make the call.

**Packs** group members into named councils for focused reviews:

```bash
council packs create product
council packs add product jane-doe
```

**Councils of Councils.** With several packs in the room, each council debates, then each council's spokesperson challenges the others, and a moderator lists where the councils disagree.

Under the hood, `/council` runs two commands. Any AI tool that can run a shell can do the same:

```bash
git diff HEAD | council review --pack product              # prints the room prompt
council review --pack product --record answer.json         # checks the answer, shows it, saves it
```

If the answer is incomplete or malformed, `--record` says what to fix and the AI answers again.

## GitHub Action

Review every pull request with the council committed in your repo. Commit `.council/` after assembling it, then:

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
      - uses: actions/checkout@v5
      - uses: luuuc/council/action@v1
        with:
          pack: product   # optional: a pack from .council/packs/
          # anthropic-api-key: ${{ secrets.ANTHROPIC_API_KEY }}
```

The Action sends the room prompt for the PR diff to a model API (`council review --api`) and posts a PR review with inline comments, plus a check run.

| Input set | Provider | Default model |
|---|---|---|
| `anthropic-api-key` | Anthropic | `claude-sonnet-4-6` |
| `openai-api-key` | OpenAI | `gpt-4.1` |
| Neither | GitHub Models (free, 150 requests/day) | `openai/gpt-4.1-mini` |

Each review is one request (two if the first answer needs fixing). The free tier caps how much text a request can carry, so large diffs or big councils need an API key.

## Claude Desktop and other MCP clients

```json
{
  "mcpServers": {
    "council": {
      "command": "council",
      "args": ["mcp", "--dir", "/path/to/your/project"]
    }
  }
}
```

Pick **council** or **assemble** from the prompt menu. The client's own model writes the debate; no API key needed. See [docs/integrations.md](docs/integrations.md).

## Supported tools

| Tool | How |
|---|---|
| Claude Code | Skills: `/council`, `/council-assemble`, plus agents |
| OpenCode | Commands: `/council`, `/council-assemble`, plus agents |
| Codex and other Agent Skills tools | Skills in `.agents/skills/`: `$council` in Codex |
| Claude Desktop, Cursor | MCP |
| GitHub | The Action |

## Commands

Your AI tool drives these; you rarely type them.

| Command | Does |
|---|---|
| `council init` | Create `.council/` and install the council skills for your AI tool |
| `council assemble` | Print the brief your AI follows to propose and build members |
| `council add <file \| ->` | Check a persona, stamp the disclaimer, save it |
| `council list` / `show <id>` / `remove <id>` | Manage members |
| `council packs ...` | Group members into named councils |
| `council review` | Print the room prompt (`--pack`, `--councils`, `--file`) |
| `council review --record <file \| ->` | Check the debate, show it, save it |
| `council review --api` | Unattended: send the prompt to a model API with your key |
| `council sync` | Rewrite the skills, commands, and agent files |
| `council doctor` | Check the council setup and report problems |
| `council mcp` | The same, as MCP tools |

Manual install:

```bash
curl -fsSL https://raw.githubusercontent.com/luuuc/council/main/install.sh | sh
# or
go install github.com/luuuc/council/cmd/council@latest
```

## Philosophy

- **Friction by design.** Members hold different positions and should not agree by default.
- **One room.** Members react to each other in order, in one pass.
- **Customers have a seat.** Product reviews include the people who will use the product.
- **The human decides.** Council shows disagreements, blind spots, and trade-offs. It does not vote for you.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
