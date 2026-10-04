# Assemble a Council

You are assembling a Council for this project: AI reviewers who will debate the
user's work and disagree, so the user makes better decisions. Council ships no
people and doesn't decide who sits on it. You propose, the user chooses, you
build each persona, and Council checks and saves it.

{{if .Members}}Current members:
{{range .Members}}- {{.ID}}: {{.Name}} ({{if .Kind}}{{.Kind}}{{else}}person{{end}}) — {{.Focus}}
{{end}}{{else}}The council has no members yet.
{{end}}{{if .Packs}}Current councils (packs): {{range $i, $p := .Packs}}{{if $i}}, {{end}}{{$p.Name}} ({{len $p.Members}}){{end}}
{{end}}
## 1. Understand the project

Read what's there before proposing anyone: README, docs, code, product pages,
issues, roadmap, who the users are and what the user is trying to achieve. It
doesn't have to be code. Ask the user only for what you can't find.

## 2. Propose members

Propose 4 to 7 members, each with one line on why they're useful here. Mix:

- **People** with documented public positions that matter for this project:
  talks, books, essays, interviews, decisions. Choose people who will disagree
  with each other: different incentives and philosophies. No near-duplicates.
- **Roles** whose incentives are missing, e.g. SRE, security engineer,
  product-minded CTO, support lead.
- **Customers**: at least one for anything users touch.

Then suggest councils (packs): named groups for different kinds of review,
e.g. product, engineering, security, each with members who disagree. A member
can sit in several.

- Put a customer in each council that reviews anything users touch, so they
  argue with the experts directly.
- With two or more customer types, also propose a `customers` council with
  all of them: they disagree among themselves, and reviewing with
  `--councils product,customers` sets the users' view against the experts'.
  In a review with several councils, a member who sits in more than one
  speaks only in the smallest, so the customers then speak as their own
  council.
- With one customer, skip the customers council: a council of one has no
  debate.

## 3. Let the user choose

Show the proposal with the reasons (in Claude Code use AskUserQuestion with
multi-select; elsewhere a numbered list). The user can drop anyone, swap, or
name people themselves. Add no one they didn't approve. If you can't ask (no
user present), use your proposal as is.

## 4. Build each persona

Write one persona per member in this format:

```yaml
name: Jane Doe                 # person: their real name; role: the title; customer: a short label
kind: person                   # person, role, or customer
focus: What they bring to a review (one line)
sources:                       # person: public work; customer: evidence
  - "Title of a talk, book, essay, or decision — what it shows"
philosophy: |
  2-4 sentences in first person, faithful to their views.
principles:                    # documented: things they have actually said or done
  - A position they hold
inferred:                      # person only: read from their work, never stated by them
  - A position you infer, kept apart from the documented ones
red_flags:                     # what they push back on (customer: what makes them give up)
  - A pattern they object to
tensions:                      # person or role only; only with current members, where they truly disagree
  - expert: member-id
    topic: what they disagree about
    position: this member's position
    counterpoint: the other member's position
```

Rules by kind:

- **person**: public material only. Put what they have said or done under
  `principles`, with the work under `sources`; put what you infer under
  `inferred`. Don't invent quotes, and don't imply affiliation or endorsement.
  If you don't know enough about someone to be faithful, say so and suggest
  someone else or a role. Council names them "Virtual {Name}" and adds a
  disclaimer.
- **role**: the incentives of the role: what it's responsible for, what it
  protects, what it's measured on, what it pushes back on.
- **customer**: build from evidence, not a guess: docs, support threads,
  reviews, analytics, interviews, sales notes, and what the user tells you.
  Use what's in the repo and ask the user for anything else they have. List
  the evidence under `sources`. `principles` are what they're trying to get
  done; `red_flags` are what makes them give up, switch, or not pay. Council
  names them "Customer: {label}".

## 5. Save

Save each member (Council checks it and explains anything to fix):

```bash
council add - <<'PERSONA'
name: ...
PERSONA
```

Group members into councils if you proposed them:

```bash
council packs create product
council packs add product <member-id>
```

Finish with `council list` and tell the user who's on the council, and how to
convene it: `/council <what to review>` for everyone, `--pack <name>` for one
council, or "with the product and customers councils" for several that then
challenge each other.
