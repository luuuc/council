# Architecture

Council is a small CLI the user's AI tool drives. It never names anyone and, inside an AI tool, never calls a model: the AI does the thinking, Council checks and stores the result.

```
 Assemble                                   Convene
 ────────                                   ───────
 council assemble ──▶ brief                 council review ──▶ room prompt
        │                                          │
   user's AI proposes, user picks,            user's AI writes the whole
   AI builds each persona                     debate in one pass (JSON)
        │                                          │
 council add ──▶ check, disclaimer,         council review --record ──▶ check,
                 .council/experts/                 render, .council/reviews/
        │
 council sync ──▶ .claude/, .opencode/, AGENTS.md
```

Unattended (the GitHub Action), `council review --api` sends the same room prompt to a model API with the user's key, checks the answer (asking once more if it needs fixing), and renders a PR review.

## Key Packages

| Package | Purpose |
|---|---|
| `brief` | The assembly brief (`assemble.md`), shared by the CLI and MCP |
| `expert` | Persona format: parse, check (`prepare.go`, `virtual.go`), save, list |
| `pack` | Named groups of members in `.council/packs/` |
| `review` | Room prompt (`room.md`, `BuildRoomPrompt`), answer checking (`ParseRoom`), synthesis, text and GitHub output, `APIBackend` |
| `adapter` | Per-tool formats and paths: Claude Code, OpenCode, generic `AGENTS.md` |
| `sync` | Writes members and slash commands to each tool |
| `mcp` | The same steps as MCP tools: `council_assemble`, `council_add`, `council_room`, `council_record`, `council_list` |
| `config` | `.council/config.yaml`: the AI tool, and the API provider and model for `--api` |

## The room

One prompt holds every member of one or several councils, the submission, and the debate rules: members speak in pack order and reply to each other (agree, disagree, adds), earlier members get a final word, and a neutral moderator lists disagreements and decisions. With several councils, each debates, then spokespersons answer each other and a moderator closes across councils.

`ParseRoom` is strict: every member reviews once, ids and verdicts are valid, replies stay inside the room, and each disagreement has two sides. On a bad answer it returns an `*AnswerError` listing what to fix, so the AI can answer again. Output never recommends an outcome; the overall verdict exists only for CI gating.

## Persona file

```markdown
---
id: jane-doe
name: Virtual Jane Doe
kind: person            # person, role, or customer
focus: Test-driven development
sources:
  - Talk: "Tests first", 2019
principles:             # documented positions
  - Red-green-refactor
inferred:               # read from their work, never stated by them
  - Prefers small commits
red_flags:
  - Tests written after code
disclaimer: Modeled on public material. Not affiliated with or endorsed by Jane Doe.
---

# Virtual Jane Doe - Test-driven development

Body content here...
```

People are named "Virtual {Name}" and need public sources; Council stamps the disclaimer. Customers ("Customer: {label}") need evidence in `sources`. Roles are a plain title.

## Adapter pattern

Each AI tool implements `Adapter` (name, detection, paths, agent and command formats). Adding a tool: implement it and call `Register()` in `init()`.
