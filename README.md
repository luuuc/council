# The Council Creator

**Better feedback. Broader perspective. Voices that push back.**

AI tools are eager to please. They validate your ideas, agree with your approach, and move fast. That's useful—but you lose something. Your own judgment fades within the AI's rapid flow. No one asks the hard questions.

A council fixes this. You create a group of expert voices—real people, fictional characters, your team members, anyone—who review your work from their unique perspectives. Rob Pike asks if it's clear. Kent Beck asks where the tests are. Jason Fried asks if you really need that feature.

The AI stops being one agreeable voice and becomes many distinct, challenging voices.

## Get Started

**Tell your AI assistant:**

> Grab https://raw.githubusercontent.com/luuuc/council-cli/main/AGENT-INSTALL.md and get me set up

That's it. Works with Claude Code, OpenCode, or any AI that can fetch URLs.

After setup, use `/council <topic>` to convene your experts.

## Create Your Council

Your council is yours. Add whoever helps you do better work:

- **Curated experts**: Kent Beck, Sandi Metz, Dieter Rams, DHH, and 100+ others
- **Your team**: Add your CTO, your tech lead, your mentor
- **Fictional characters**: Donald Duck for your cartoon project? Valid.
- **Custom voices**: Create personas with specific philosophies and red flags

```bash
council add "Kent Beck"              # From curated library
council add "My Tech Lead"           # Create custom persona
/council-add a security expert       # AI-assisted discovery
```

## How It Works

```
Your Council                         Your AI Tool
┌─────────────────┐                  ┌─────────────────┐
│ Rob Pike        │                  │ /council        │
│ Kent Beck       │───── sync ──────▶│ /council-add    │
│ Your CTO        │                  │ /council-remove │
│ Custom Expert   │                  │                 │
└─────────────────┘                  └─────────────────┘
```

Councils live in your project (`.council/experts/`), sync to your AI tool's native format, and become slash commands you invoke anytime.

## Manual Installation

```bash
# Direct download
curl -fsSL https://raw.githubusercontent.com/luuuc/council-cli/main/install.sh | sh

# Or via Go
go install github.com/luuuc/council-cli/cmd/council@latest
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

Run blind parallel reviews where each expert reviews independently — no expert sees another's output:

```bash
git diff main | council review --pack go
council review --pack rails --file app/models/user.rb --json
```

Each expert returns a verdict (pass / comment / block / escalate). The engine synthesizes agreements, tensions, and a final recommendation.

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
- `council_review` — blind parallel review, returns structured verdict
- `council_list` — list pack members (no LLM calls)
- `council_explain` — expand on a review note with expert reasoning

## Supported AI Tools

| Tool | Integration |
|------|-------------|
| Claude Code | Slash commands + agents + MCP |
| Cursor | MCP |
| Claude Desktop | MCP |
| OpenCode | Agents |
| Others | `council export` for portable markdown |

## Philosophy

- **Your council, your voices.** We provide a curated library; you decide who sits on your council.
- **Outcome over mechanism.** The tool should disappear. You just say `/council` and get feedback.
- **Openness over prescription.** Code review, writing feedback, business decisions—councils work for any domain.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup.

## License

MIT
