# Set Up Council

You are an AI assistant setting up Council for a user. Council is a small CLI you drive: the user talks to you, you run the commands. Follow these steps.

When you offer choices: in Claude Code, use **AskUserQuestion** (short labels, details in descriptions); elsewhere, a numbered list.

Tell the user what you're about to do, then go (no confirmation needed, they asked for setup):

> I'll install Council, set it up in this project, and help you pick who sits on your council.

## Step 1: Install or update the CLI

Always run this, even if `council` is installed: it installs, updates, or does nothing.

```bash
curl -fsSL https://raw.githubusercontent.com/luuuc/council/main/install.sh | sh
```

If that fails, try `go install github.com/luuuc/council/cmd/council@latest`.

## Step 2: Set up the project

```bash
council list
```

- **No council yet** ("council not initialized"): run `council init`. It creates `.council/`, detects the AI tool (Claude Code, OpenCode, or `AGENTS.md` for others such as Codex), and installs the `/council` commands. The council starts with one member, the author's persona (Virtual Luc Perussault-Diallo), as an example; the user can remove it with `council remove luc-perussault-diallo`.
- **Members already exist**: show them and ask: **Add more** (go to step 3), **Start fresh** (`council init --clean`, then step 3), or **All set** (go to step 4).
- **`council list` fails although `.council/` exists**: ask: **Start fresh** (`council init --clean`) or **Cancel**.
- **`council list` names pack members who aren't on the council** (packs from an older version): mention them; step 3 can add them back, or `council packs remove <pack> <id>` drops them.

## Step 3: Assemble the council

```bash
council assemble
```

Follow the brief it prints, step by step. In short: read the project, propose members with a reason for each (people with documented public positions who will disagree, roles whose view is missing, and a customer for anything users touch), let the user choose (multi-select), build each persona from public material or evidence, and save each one with `council add -`. If Council rejects a persona, fix what it says and try again.

## Step 4: Done

Tell the user the council is ready and list its members. Then offer: **Try it now** or **I'm all set**.

If **Try it now**: ask what the council should review (a file, the current changes, a plan, or a decision), then convene it: in Claude Code or OpenCode run `/council <that>`; elsewhere follow "Convening the council" in `AGENTS.md`.

Remind them:
- `/council <topic>`: convene the council on code, a document, a plan, or a decision. It ends with where members disagree and what you need to decide.
- `/council <topic> with the product and security councils`: several councils that challenge each other (after grouping members with `council packs`).
- `/council-assemble`: extend the council. `/council-add <who>`: add one member. `/council-remove <id>`: remove one.
- To review pull requests on GitHub, commit `.council/` and add the Action (see the README).
