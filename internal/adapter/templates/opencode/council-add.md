# Add Expert to Council

Add a new expert to the council: $ARGUMENTS

## Step 1: Classify Input

Determine what type of input $ARGUMENTS is:

- **Name**: Quoted string, or 2-3 capitalized words forming a person's name (e.g., "Ada Redgrave", Sable Okoro)
- **Description**: Contains "a ", "someone", "expert in", "help with" (e.g., "a testing expert")
- **Keyword**: Single word describing a domain (e.g., "testing", "APIs")

**If ambiguous, treat as description and search.**

## Step 2: Search Curated Personas

Run this command to get all available curated personas:
```bash
council personas --json
```

Search the output for matches against name, id, focus, triggers, and philosophy.
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

If the user provided a name and it matches a curated persona, present these options:

```
Found "{Name}" in the curated library.

Options:
1. Add {Name} - Install this expert from the curated library
2. Show alternatives - See related experts before deciding
3. Cancel - Don't add anyone

Which option? (1/2/3):
```

Wait for the user's response, then:
- Option 1: Run `council add "{name}"` to install
- Option 2: Present 4 options including the match and related experts
- Option 3: Exit

### Step 4b: Name Not Found

If the user provided a name but it's not in the curated library:

```
"{Name}" is not in the curated library.

Options:
1. Create {Name} - I'll research and build a custom profile
2. Show similar - Browse related curated personas
3. Cancel - Don't add anyone

Which option? (1/2/3):
```

Wait for the user's response, then:
- Option 1: Proceed to Step 5 to generate a custom profile
- Option 2: Search curated personas by inferred domain and present 4 options
- Option 3: Exit

### Step 4c: Description or Keyword

For description or keyword inputs, present 4 options:

```
Based on your request, here are 4 options:

1. {Name} - {focus} (curated)
2. {Name} - {focus} (curated)
3. {Name} - {brief expertise} (will create custom profile)
4. Custom - Create a persona matching your description

Which option? (1/2/3/4):
```

Wait for the user's response, then:
- Curated options: Run `council add "{name}"` to install
- Suggested options: Proceed to Step 5 to generate a custom profile
- Custom: Ask for additional details, then proceed to Step 5

## Step 5: Generate Custom Profile

Once an expert is confirmed for custom creation, generate a rich profile.

Research or use your knowledge of this person to generate:

1. **Philosophy** (2-4 sentences): What they believe about software/design. Write in first person.
2. **Principles** (4-6 items): Concrete, actionable guidelines they're known for.
3. **Red Flags** (3-5 items): Patterns they would call out during code review.

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
