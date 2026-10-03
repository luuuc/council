package adapter

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

//go:embed templates/opencode/install.md
var opencodeInstallTemplate string

//go:embed templates/opencode/council-add.md
var opencodeCouncilAddTemplate string

//go:embed templates/opencode/council-assemble.md
var opencodeCouncilAssembleTemplate string

//go:embed templates/opencode/council-remove.md
var opencodeCouncilRemoveTemplate string

func init() {
	Register(&OpenCode{})
}

// OpenCode is the adapter for OpenCode.
type OpenCode struct{}

func (o *OpenCode) Name() string {
	return "opencode"
}

func (o *OpenCode) DisplayName() string {
	return "OpenCode"
}

func (o *OpenCode) Detect() bool {
	return DirExists(".opencode") || FileExists("opencode.json")
}

func (o *OpenCode) Paths() Paths {
	return Paths{
		Agents:     ".opencode/agents",
		Commands:   ".opencode/commands",
		Deprecated: []string{".opencode/agent"}, // Old singular path
	}
}

func (o *OpenCode) Templates() Templates {
	return Templates{
		Install: opencodeInstallTemplate,
		Commands: map[string]string{
			"council-assemble": opencodeCouncilAssembleTemplate,
			"council-add":      opencodeCouncilAddTemplate,
			"council-remove":   opencodeCouncilRemoveTemplate,
		},
	}
}

// FormatAgent creates OpenCode agent file content.
// OpenCode uses a different frontmatter format with description and mode.
func (o *OpenCode) FormatAgent(e *expert.Expert) string {
	var parts []string

	// OpenCode uses different frontmatter format
	parts = append(parts, "---")
	parts = append(parts, fmt.Sprintf("description: %s", e.Focus))
	parts = append(parts, "mode: subagent")
	parts = append(parts, "---")
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("# %s", e.Name))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("You are %s, known for expertise in %s.", e.Name, e.Focus))
	parts = append(parts, "")

	if e.Philosophy != "" {
		parts = append(parts, "## Philosophy")
		parts = append(parts, "")
		parts = append(parts, strings.TrimSpace(e.Philosophy))
		parts = append(parts, "")
	}

	if len(e.Principles) > 0 {
		parts = append(parts, "## Principles")
		parts = append(parts, "")
		for _, p := range e.Principles {
			parts = append(parts, fmt.Sprintf("- %s", p))
		}
		parts = append(parts, "")
	}

	if len(e.RedFlags) > 0 {
		parts = append(parts, "## Red Flags")
		parts = append(parts, "")
		parts = append(parts, "Watch for these patterns:")
		for _, r := range e.RedFlags {
			parts = append(parts, fmt.Sprintf("- %s", r))
		}
		parts = append(parts, "")
	}

	parts = append(parts, "## Review Style")
	parts = append(parts, "")
	parts = append(parts, "When reviewing code, focus on your area of expertise. Be direct and specific.")
	parts = append(parts, "Explain your reasoning. Suggest concrete improvements.")

	return strings.Join(parts, "\n")
}

// FormatCommand creates OpenCode command file content.
// OpenCode commands have frontmatter with description and mode.
func (o *OpenCode) FormatCommand(name, description, body string) string {
	var parts []string
	parts = append(parts, "---")
	parts = append(parts, fmt.Sprintf("description: %s", description))
	parts = append(parts, "mode: subagent")
	parts = append(parts, "---")
	parts = append(parts, "")
	parts = append(parts, body)
	return strings.Join(parts, "\n")
}
