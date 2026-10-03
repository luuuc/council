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

- If **Add more**: Skip to "Shape the Council" (Step 4)
- If **Start fresh**: Run `council init --clean`, then run `council start`
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

Run the zero-config setup:

```bash
council start
```

This single command:
1. Creates the `.council/` directory
2. Detects your AI tool (Claude Code, OpenCode, or generic)
3. Detects your project stack (languages, frameworks, testing tools)
4. Adds experts matched to your stack (real people, named "Virtual X")
5. Syncs everything to your AI tool

Output looks like:
```
✓ Detected: Claude Code
✓ Detected: Go
✓ Added 6 experts: Virtual Rob Pike, Virtual Kent Beck, Virtual Bruce Schneier, Virtual Gene Kim, Virtual Dieter Rams, Virtual Luc Perussault-Diallo

Your council is ready. Try: /council <topic>
```

## Step 4: Shape the Council

A good council mixes people with different incentives, and includes the people the work is for. Tell the user who was added, then use **AskUserQuestion** (multi-select):

| Label | Description |
|-------|-------------|
| "Add a customer" | Someone the work is for, who reacts as a user |
| "Add a role" | e.g. SRE, security engineer, product-minded CTO |
| "Add or remove people" | Real people from the library or researched |
| "Looks good" | Keep the council as it is |

Always pass `--yes`: your shell can't answer interactive prompts.

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

### Add or remove people

Browse the library:
```bash
council personas --json
```

Add someone from the library, or anyone with documented public work (Council researches them and names them "Virtual {Name}"):
```bash
council add "Kent Beck" --yes
council add "Boris Cherny" --yes
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
council remove kent-beck
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
