package adapter

import (
	"fmt"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

func init() {
	Register(&Generic{})
}

// Generic is the fallback adapter for projects without a specific AI tool.
// It generates an AGENTS.md file in the project root.
type Generic struct{}

func (g *Generic) Name() string {
	return "generic"
}

func (g *Generic) DisplayName() string {
	return "Generic (AGENTS.md and .agents/skills)"
}

// Detect always returns true - generic is the fallback.
// However, it's excluded from automatic detection in Detect()
// and must be explicitly selected.
func (g *Generic) Detect() bool {
	return true
}

func (g *Generic) Paths() Paths {
	return Paths{
		Agents: ".", // AGENTS.md in project root
	}
}

func (g *Generic) Templates() Templates {
	// The OpenCode wording: plain numbered lists, no tool-specific UI.
	return Templates{
		Commands: map[string]string{
			"council-assemble": opencodeCouncilAssembleTemplate,
			"council-add":      opencodeCouncilAddTemplate,
			"council-remove":   opencodeCouncilRemoveTemplate,
		},
	}
}

// FormatAgent creates a simple markdown section for an expert.
// For generic, this is used as part of AGENTS.md generation.
func (g *Generic) FormatAgent(e *expert.Expert) string {
	var parts []string

	parts = append(parts, "### "+e.Name)
	parts = append(parts, fmt.Sprintf("- **ID**: %s", e.ID))
	parts = append(parts, fmt.Sprintf("- **Focus**: %s", e.Focus))
	parts = append(parts, "")

	if e.Philosophy != "" {
		parts = append(parts, strings.TrimSpace(e.Philosophy))
		parts = append(parts, "")
	}

	if len(e.Principles) > 0 {
		parts = append(parts, "**Principles:**")
		for _, p := range e.Principles {
			parts = append(parts, fmt.Sprintf("- %s", p))
		}
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// FormatCommand creates an Agent Skills SKILL.md, read by Codex and many
// other tools. Skills there take no arguments, so $ARGUMENTS becomes the
// user's request.
func (g *Generic) FormatCommand(name, description, body string) string {
	return skillFrontmatter(name, description) + strings.ReplaceAll(body, "$ARGUMENTS", "the user's request")
}

// CommandPath returns the skill's path: .agents/skills/<name>/SKILL.md.
func (g *Generic) CommandPath(name string) string {
	return skillPath(".agents/skills", name)
}

// GenerateAgentsMd creates the complete AGENTS.md file content.
// This is a special method for the generic adapter since it combines
// all experts into a single file rather than separate files.
func (g *Generic) GenerateAgentsMd(experts []*expert.Expert) string {
	var parts []string

	parts = append(parts, "# AGENTS.md - Expert Council")
	parts = append(parts, "")
	parts = append(parts, "This file defines expert personas for AI coding assistants.")
	parts = append(parts, "")
	parts = append(parts, "## Council Members")
	parts = append(parts, "")

	for _, e := range experts {
		parts = append(parts, g.FormatAgent(e))
	}

	parts = append(parts, agentsConvene)
	return strings.Join(parts, "\n")
}

// agentsConvene points AI tools that read AGENTS.md to the council skills.
const agentsConvene = `## Convening the council

To convene the council on code, changes, a document, a plan, or a decision, follow the ` + "`council`" + ` skill in ` + "`.agents/skills/council/SKILL.md`" + ` (in Codex: ` + "`$council <topic>`" + `). To assemble or extend the council, follow ` + "`.agents/skills/council-assemble/SKILL.md`" + `.
`
