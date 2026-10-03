# Install Council CLI

You are an AI assistant helping a user install council. Follow these steps.

**Tool-Specific Notes:**
- **Claude Code**: Use `AskUserQuestion` to present choices as clickable options where indicated. Keep labels short (2-4 words), put details in descriptions.
- **OpenCode**: Present choices as numbered lists. Wait for user to type a number.

## Tell the User What's Happening

Briefly explain what you're about to do, then proceed (no confirmation needed - they asked for setup):

> I'll set up council: install the CLI, create your council with experts matched to your project, and sync to your AI tool. Here we go!

## Step 1: Install or Update the CLI

**Always run this first**, regardless of whether council is already installed. The script is idempotent: it installs if missing, updates if outdated, and does nothing if current.

```bash
curl -fsSL https://raw.githubusercontent.com/luuuc/council/main/install.sh | sh
```

If that fails (permissions, curl not available, etc.), try:

```bash
go install github.com/luuuc/council/cmd/council@latest
```

## Step 2: Check Existing Setup

After ensuring the CLI is current, check for existing configuration:

```bash
council list 2>/dev/null
```

### If experts already exist:

Tell the user which experts exist, then use **AskUserQuestion**:

| Label | Description |
|-------|-------------|
| "Add more" | Keep current experts and add new ones |
| "Start fresh" | Remove everything and set up from scratch |
| "All set" | Keep everything as is |

- If **Add more**: Skip to "Assemble the Council" (Step 4)
- If **Start fresh**: Run `council init --clean`, then continue to Step 4
- If **All set**: Skip to "Done"

### If .council/ exists but council list fails:

A `.council/` directory exists from a previous or incompatible installation. Use **AskUserQuestion**:

| Label | Description |
|-------|-------------|
| "Start fresh" | Remove and reinitialize |
| "Cancel" | Stop here |

- If **Start fresh**: Run `council init --clean`, then continue to Step 3
- If **Cancel**: Stop here

### If no .council/ directory:

Continue to Step 3.

## Step 3: Set Up Your Council

```bash
council init
```

This creates the `.council/` directory, detects your AI tool (Claude Code, OpenCode, or generic), and installs the `/council` commands. Council doesn't pick people, you and the user do. The council starts with one member, the author's persona (Virtual Luc Perussault-Diallo), as a working example; the user can keep it or remove it with `council remove luc-perussault-diallo`.

## Step 4: Assemble the Council

A good council mixes people with different incentives, and includes the people the work is for.

1. **Read the project**: code, README, docs, goals, who the users are. It doesn't have to be code.
2. **Propose 4 to 6 members**, each with one line on why they're useful:
   - people with documented public positions relevant to this project, chosen to disagree with each other
   - a role whose incentives are missing (e.g. SRE, security engineer, product-minded CTO)
   - for anything users touch, a customer
3. **Let the user choose** with **AskUserQuestion** (multi-select). They can swap anyone, or name people themselves.
4. **Add each chosen member** (always pass `--yes`: your shell can't answer interactive prompts):

### Add a customer

Ask in one question who the users are (e.g. "freelancers who bill clients by the hour"), then:
```bash
council add --customer "<their description>" --yes
```

### Add a role

Ask which role, then:
```bash
council add --role "<role, e.g. SRE>" --yes
```

### Add a person

Anyone with documented public work. Council researches them and names them "Virtual {Name}":
```bash
council add "Jane Doe" --yes
```

Or use the `/council-add` skill to search by description:
```
/council-add a security expert
/council-add someone for API design
```

### Mix AI models (if more than one AI CLI is installed)

Check which AI CLIs are installed:
```bash
command -v claude codex opencode
```

If two or more are present, offer to spread members across them: models from different labs disagree more honestly. With the user's OK, add to `.council/config.yaml` under `ai:` (opencode takes a `provider/model` from `opencode models`):
```yaml
  mix:
    - command: claude
    - command: codex
    - command: opencode
```

### Removing Experts

List current experts:
```bash
council list
```

Remove by ID:
```bash
council remove jane-doe
```

### Syncing Changes

After adding or removing experts, sync to update your AI tool:
```bash
council sync
```

## Done

Tell the user setup is complete and list their experts, then use **AskUserQuestion**:

| Label | Description |
|-------|-------------|
| "Try it now" | Run /council on a file or topic |
| "I'm all set" | End the setup flow |

If **Try it now**: Ask what they'd like the council to review (a file, function, or topic), then run `/council` for them.

Remind them of available commands:
- `/council <topic>` - Convene the council on code, a plan, or a decision. It ends with where members disagree and what you need to decide
- `/council <topic> with the product and security councils` - Several councils that challenge each other
- `/council-add <description>` - Search and add experts by description
