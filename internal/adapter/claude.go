package adapter

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

//go:embed templates/claude/install.md
var claudeInstallTemplate string

//go:embed templates/claude/council-add.md
var claudeCouncilAddTemplate string

//go:embed templates/claude/council-assemble.md
var claudeCouncilAssembleTemplate string

//go:embed templates/claude/council-remove.md
var claudeCouncilRemoveTemplate string

func init() {
	Register(&Claude{})
}

// Claude is the adapter for Claude Code.
type Claude struct{}

func (c *Claude) Name() string {
	return "claude"
}

func (c *Claude) DisplayName() string {
	return "Claude Code"
}

func (c *Claude) Detect() bool {
	return DirExists(".claude")
}

func (c *Claude) Paths() Paths {
	return Paths{
		Agents:     ".claude/agents",
		Commands:   ".claude/commands",
		Deprecated: []string{},
	}
}

func (c *Claude) Templates() Templates {
	return Templates{
		Install: claudeInstallTemplate,
		Commands: map[string]string{
			"council-assemble": claudeCouncilAssembleTemplate,
			"council-add":      claudeCouncilAddTemplate,
			"council-remove":   claudeCouncilRemoveTemplate,
		},
	}
}

// FormatAgent creates Claude Code agent file content.
// For Claude Code, we use the original expert file content (preserves source format).
func (c *Claude) FormatAgent(e *expert.Expert) string {
	// Read the original expert file and return its content
	data, err := os.ReadFile(e.Path())
	if err != nil {
		// Fallback to regenerating
		return fmt.Sprintf("---\nid: %s\nname: %s\nfocus: %s\n---\n\n%s", e.ID, e.Name, e.Focus, e.Body)
	}
	return string(data)
}

// FormatCommand creates Claude Code command file content.
// Claude Code commands are plain markdown (no frontmatter needed).
func (c *Claude) FormatCommand(name, description, body string) string {
	return body
}

// CouncilCommandTemplate is exported for use by sync when generating the dynamic /council command.
// The template receives a CouncilTemplateData struct.
func CouncilCommandTemplate() string {
	return `# Council

Convene the council on: $ARGUMENTS

## Council Members

{{range .Experts}}
### {{.Name}}
**Focus**: {{.Focus}}
{{end}}
{{- if .Packs}}

## Available Packs

Use ` + "`--pack <name>`" + ` in your arguments to convene a specific pack instead of the full council.

{{range .Packs}}- **{{.Name}}**{{if .Description}} — {{.Description}}{{end}} ({{len .Members}} members)
{{end}}
{{- end}}

## Instructions

Everyone sits in the same room. Council gives you the room prompt; you write
the whole debate yourself, in one pass; Council checks it and renders it.

### 1. Decide what the council reviews

- **Files named in the arguments**: review those files.
- **Current changes, or no arguments**: review the diff (` + "`git diff HEAD`" + `).
- **A question, plan, or decision**: write a short brief to a temporary file:
  the question, the relevant context from this conversation, and the options
  being considered. The council reviews the brief.

### 2. Get the room prompt

Pass ` + "`--pack <name>`" + ` through if the arguments include one. If the user asks for
several councils (e.g. "product and security"), use ` + "`--councils product,security`" + `
instead: each council debates, then they answer each other.

` + "```bash" + `
git diff HEAD | council review [--pack <name>]   # changes
council review --file <path> [--pack <name>]    # a file or a brief
` + "```" + `

### 3. Write the debate

Follow the room prompt exactly: play each member in turn, true to their
persona, and write the whole debate as the one JSON object it asks for. Save
it to a temporary file (e.g. ` + "`/tmp/council-answer.json`" + `). Don't show it yet.

### 4. Record it

Use the same ` + "`--pack`" + ` or ` + "`--councils`" + ` as in step 2:

` + "```bash" + `
council review [--pack <name>] --record /tmp/council-answer.json
` + "```" + `

If Council says the answer needs fixing, fix the file and record again.

### 5. Present the review

Show the user what ` + "`--record`" + ` printed: each member's verdict, notes, and
replies in the order they spoke, the final words, "Where they disagree", and
"What you need to decide". Keep the disagreements visible; do not merge them
into a consensus. The user makes the call, not the council and not you.
`
}

// agentFilename returns the appropriate filename for an expert based on source
// This is exported for use by sync package
func AgentFilename(e *expert.Expert) string {
	switch {
	case e.Source == "custom":
		return "custom-" + e.ID + ".md"
	case strings.HasPrefix(e.Source, "installed:"):
		return "installed-" + e.ID + ".md"
	default:
		return e.ID + ".md"
	}
}
