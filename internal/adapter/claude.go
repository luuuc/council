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

The council debates for real: members speak one at a time, each reads what the
earlier ones said, then disagrees, backs them up, or adds what they missed.
Run it with the ` + "`council review`" + ` command instead of playing the members yourself.

### 1. Decide what the council reviews

- **Files named in the arguments**: review those files.
- **Current changes, or no arguments**: review the diff (` + "`git diff HEAD`" + `).
- **A question, plan, or decision**: write a short brief to a temporary file:
  the question, the relevant context from this conversation, and the options
  being considered. The council reviews the brief.

### 2. Run the council

Pass ` + "`--pack <name>`" + ` through if the arguments include one. If the user asks for
several councils (e.g. "product and security"), use ` + "`--councils product,security`" + `
instead: each council debates, then they challenge each other's conclusions.

` + "```bash" + `
git diff HEAD | council review [--pack <name>]   # changes
council review --file <path> [--pack <name>]    # a file or a brief
` + "```" + `

It makes one AI call per member, so it can take a few minutes. Use a long
timeout (10 minutes). Progress lines go to stderr; the review goes to stdout.

### 3. Present the debate

- Show each member's verdict, notes, and replies in the order they spoke,
  then the final words (who changed their mind, and why).
- Keep the disagreements visible. Do not merge them into a consensus.
- End with the review's "Where they disagree" and "What you need to decide".
  The user makes the call, not the council and not you.

If ` + "`council review`" + ` fails (for example, no AI backend is available), say so,
then review from each member's perspective yourself, one at a time, each
reacting to the ones before.
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
