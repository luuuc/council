# Integrations Guide

This guide covers how to use your council with different AI tools and platforms.

## Claude Desktop (MCP)

Claude Desktop connects to Council through MCP. Council hands Claude the room prompt (every member, the submission, the debate rules), Claude writes the whole debate in one pass, and Council checks it and returns the review. No API key or extra AI tool is needed.

### Setup

1. Ensure council is installed and in your PATH:

```bash
which council  # Should output the path to council
```

2. Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

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

Claude Desktop starts MCP servers outside any project, so `--dir` points Council at the project whose `.council/` you want.

3. Restart Claude Desktop completely (quit and reopen)

### Usage

Pick **council** from the prompt menu, optionally name a pack, and describe what to review. Or ask directly:

> "Convene the code council on this code"
> [paste code]

> "Ask the writing council about this launch post"

To assemble or extend your Council, pick **assemble** from the prompt menu, or ask:

> "Assemble a council for this project"

Claude reads the project, proposes members with reasons (people with documented public positions, roles, customers), lets you choose, builds each persona, and saves it with `council_add`. Run `council sync` in the project afterwards to update Claude Code and OpenCode.

### What's Exposed

| Feature | Description |
|---------|-------------|
| `council` prompt | Convenes a pack on a topic |
| `council_room` / `council_record` tools | The room prompt Claude answers in one pass, and recording that debate |
| `council_assemble` / `council_add` tools | The brief for building members, and saving each one |
| `council_list` tool | Lists a pack's members and their tensions |

### Troubleshooting

**Council not appearing:**
- Ensure `council` is in your PATH, or use the full path as `command`
- Check Claude Desktop logs: `~/Library/Logs/Claude/`

**"no council in this directory":**
- Add `--dir /path/to/project` to the server args, for a project that has run `council init`

## Codex and other AI tools

`council init` uses the generic target when it finds no Claude Code or OpenCode setup (or with `--tool generic`; add it to another project with `council sync generic`). It writes the council skills to `.agents/skills/` (the Agent Skills standard, read by Codex and many other tools) and lists the members in `AGENTS.md`. In Codex, type `$council <topic>` or `$council-assemble`; tools without skills can follow `.agents/skills/council/SKILL.md` from the pointer in `AGENTS.md`.

## Model APIs and local models

`council review --api` sends the room prompt straight to a model API with your key and prints the review. It's what the GitHub Action uses, and it works with local models through Ollama:

```bash
git diff main | council review --api --provider anthropic --pack product      # ANTHROPIC_API_KEY
git diff main | council review --api --provider ollama --model llama3.1       # local
```

Without `--provider`, Council uses `ai.provider` from `.council/config.yaml`, else the first key it finds (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GITHUB_TOKEN`). `--model` (or `ai.model`) picks the model. Small models often struggle to write the whole debate as valid JSON; Council asks once more with what to fix, then gives up with the errors.

## Tips

- **Keep councils small and opinionated.** 3 to 7 members who disagree beat a crowd that agrees.
- **Include a customer** for anything users touch.
- **Use packs** to convene the right people: `product`, `security`, `writing`.
- **Re-run `council sync`** after editing persona files by hand.
