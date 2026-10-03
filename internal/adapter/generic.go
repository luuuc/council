package adapter

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

//go:embed templates/generic/install.md
var genericInstallTemplate string

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
	return "Generic (AGENTS.md)"
}

// Detect always returns true - generic is the fallback.
// However, it's excluded from automatic detection in Detect()
// and must be explicitly selected.
func (g *Generic) Detect() bool {
	return true
}

func (g *Generic) Paths() Paths {
	return Paths{
		Agents:     ".", // AGENTS.md in project root
		Commands:   ".", // No separate commands
		Deprecated: []string{},
	}
}

func (g *Generic) Templates() Templates {
	return Templates{
		Install:  genericInstallTemplate,
		Commands: map[string]string{}, // No commands for generic
	}
}

// FormatAgent creates a simple markdown section for an expert.
// For generic, this is used as part of AGENTS.md generation.
func (g *Generic) FormatAgent(e *expert.Expert) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("### %s%s", e.Name, e.SourceMarker()))
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

// FormatCommand returns empty for generic - no commands supported.
func (g *Generic) FormatCommand(name, description, body string) string {
	return ""
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

	return strings.Join(parts, "\n")
}
