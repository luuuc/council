# Add Expert to Council

Add a new expert to the council: $ARGUMENTS

## Step 1: Classify Input

Determine what type of input $ARGUMENTS is:

| Type | Pattern | Examples |
|------|---------|----------|
| **Name** | Quoted string, or 2-3 capitalized words forming a person's name | `"Ada Redgrave"`, `Sable Okoro` |
| **Description** | Contains "a ", "someone", "expert in", "help with" | `a testing expert`, `someone for API design` |
| **Keyword** | Single word describing a domain | `testing`, `APIs`, `security` |

**If ambiguous, treat as description and search.**

## Step 2: Search Curated Personas

Run this command to get all available curated personas:
```bash
council personas --json
```

Search the output for matches against:
- `name` and `id` - for name inputs (exact or close match)
- `focus` - primary match for descriptions/keywords
- `triggers` - semantic keywords that indicate relevance
- `philosophy` - deeper concept matching

Rank matches by relevance. A persona matching multiple fields ranks higher.

Also check who is already installed:
```bash
council list --json
```

Avoid suggesting experts already in the council.

## Step 3: Build 4 Suggestions (Rule of 4)

Always present exactly 4 options. This provides enough choice without overwhelming.

**If Name input found in curated personas:**
Skip to the Name Found flow (Step 4a).

**If Name input NOT found in curated personas:**
Skip to the Name Not Found flow (Step 4b).

**If Description or Keyword input:**

Count curated matches and build 4 options:

**0 curated matches:**
- Slots 1-3: AI-suggested well-known experts in the domain
- Slot 4: "Custom" - create persona from description

**1-3 curated matches:**
- Slots 1-N: Curated matches (most relevant first)
- Remaining slots: AI-suggested well-known experts to complement
- Slot 4: "Custom" (always last)

**4+ curated matches:**
- Slots 1-3: Top 3 curated matches (most relevant)
- Slot 4: "Show more curated" or "Custom"

## Step 4: Present Options

### Step 4a: Name Found in Curated

If the user provided a name and it matches a curated persona, tell the user what you found, then use **AskUserQuestion** with these options:

| Label | Description |
|-------|-------------|
| "Add {Name}" | Install this expert from the curated library |
| "Show alternatives" | See related experts before deciding |
| "Cancel" | Don't add anyone |

- If **Add {Name}**: Run `council add "{name}"` to install the curated persona.
- If **Show alternatives**: Present 4 options including the match and related experts.

### Step 4b: Name Not Found

If the user provided a name but it's not in the curated library, tell the user the name wasn't found, then use **AskUserQuestion** with these options:

| Label | Description |
|-------|-------------|
| "Create {Name}" | I'll research and build a custom profile |
| "Show similar" | Browse related curated personas |
| "Cancel" | Don't add anyone |

- If **Create {Name}**: Proceed to Step 5 to generate a custom profile.
- If **Show similar**: Search curated personas by inferred domain and present 4 options.

### Step 4c: Description or Keyword

For description or keyword inputs, build exactly 4 options and use **AskUserQuestion** to present them:

| Label | Description |
|-------|-------------|
| "{Name}" | {focus} - from curated library |
| "{Name}" | {focus} - from curated library |
| "{Name}" | {brief expertise} - will create custom profile |
| "Custom" | Create a persona matching your description |

Labels should indicate source:
- **(curated)** options use `council add "{name}"` - has pre-written philosophy/principles
- **(suggested)** options need custom profile generation

When the user selects:
- **Curated option**: Run `council add "{name}"` to install
- **Suggested option**: Proceed to Step 5 to generate a custom profile
- **Custom**: Ask for any additional details, then proceed to Step 5

## Step 5: Generate Custom Profile

Once an expert is confirmed for custom creation, generate a rich profile.

Research or use your knowledge of this person to generate:

1. **Philosophy** (2-4 sentences): What they believe about software/design. Write in first person as if they're speaking. Capture their distinctive worldview.

2. **Principles** (4-6 items): Concrete, actionable guidelines they're known for. These should be memorable and specific to their thinking.

3. **Red Flags** (3-5 items): Patterns they would call out during code review. Things that violate their principles.

### Output Format

Create the expert file at `.council/experts/{id}.md` with this structure:

```markdown
---
id: {kebab-case-id}
name: {Full Name}
focus: {focus area}
philosophy: |
  {philosophy text - first person, 2-4 sentences}
principles:
  - {principle 1}
  - {principle 2}
  - {principle 3}
  - {principle 4}
red_flags:
  - {red flag 1}
  - {red flag 2}
  - {red flag 3}
---

# {Name} - {focus}

You are {Name}, known for expertise in {focus}.

## Philosophy

{philosophy text}

## Principles

- {principle 1}
- {principle 2}
- ...

## Red Flags

Watch for these patterns:
- {red flag 1}
- {red flag 2}
- ...

## Review Style

{2-3 sentences describing how they approach code review}
```

## After Creating

1. Write the file using your file writing capability
2. Run `council sync` to update AI tool configurations
3. Confirm creation with: "Added {Name} ({id}) to the council"
4. Show the file path

**Note:** For curated personas, use `council add "{name}"` instead of writing files directly.
