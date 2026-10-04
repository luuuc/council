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

Pick **council** from the prompt menu, choose a pack, and describe what to review. Or ask directly:

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

## Local LLMs (Ollama, LM Studio, etc.)

Use your council as a system prompt for local language models.

### Setup

1. Export your council:

```bash
council export > system-prompt.md
```

2. Configure your local LLM to use this as the system prompt

**Ollama example:**

```bash
# Create a Modelfile
cat > Modelfile << 'EOF'
FROM llama3.1
SYSTEM """
You have access to an expert council for code review.

$(cat system-prompt.md)

When asked to review code, consider each expert's perspective.
"""
EOF

ollama create council-reviewer -f Modelfile
ollama run council-reviewer
```

**LM Studio:**
Copy the contents of `system-prompt.md` into the System Prompt field.

## Anthropic API

```python
import anthropic

with open('council.md', 'r') as f:
    council = f.read()

client = anthropic.Anthropic()
message = client.messages.create(
    model="claude-sonnet-4-20250514",
    max_tokens=1024,
    system=f"""You have access to an expert council:

{council}

Review code from each expert's perspective.""",
    messages=[
        {"role": "user", "content": "Review this code: ..."}
    ]
)
```

## Best Practices

1. **Keep your council focused** - 3-5 experts is usually optimal
2. **Update regularly** - Re-sync when you add or modify experts
3. **Match experts to project** - Your Rails project council differs from your Go project council
4. **Test with real code** - Verify experts give useful, distinct perspectives
