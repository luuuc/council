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

Run:

```bash
council assemble
```

and follow the brief it prints. In short: read the project, propose 4 to 7 members with a reason each (people with documented public positions who will disagree, roles whose incentives are missing, and a customer for anything users touch), let the user choose with **AskUserQuestion** (multi-select), build each persona from public material or evidence, and save it with `council add -`.

Later, the user can run `/council-assemble` to extend the council, or `/council-add <who>` to add one member.

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
