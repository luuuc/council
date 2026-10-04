package mcp

import (
	"fmt"
	"strings"

	"github.com/luuuc/council/internal/brief"
	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
)

// handleAssemble implements the council_assemble MCP tool.
func (s *Server) handleAssemble() toolCallResult {
	if !config.Exists() {
		return errorResult("no council in this directory: start the server with `council mcp --dir <project>` for a project that has run `council init`")
	}
	text, err := brief.Assemble()
	if err != nil {
		return errorResult(fmt.Sprintf("failed to build the brief: %v", err))
	}
	return textResult(text)
}

// handleAdd implements the council_add MCP tool: the same checks and naming
// rules as 'council add'.
func (s *Server) handleAdd(args map[string]any) toolCallResult {
	raw, ok := args["persona"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return errorResult("missing required field: persona")
	}
	if !config.Exists() {
		return errorResult("no council in this directory: start the server with `council mcp --dir <project>` for a project that has run `council init`")
	}

	e, err := expert.ParseLoose(raw)
	if err != nil {
		return errorResult(fmt.Sprintf("could not read the persona: %v", err))
	}
	if err := expert.Prepare(e); err != nil {
		return errorResult(err.Error())
	}
	if expert.Exists(e.ID) {
		return errorResult(fmt.Sprintf("%s (%s) is already on the council", e.Name, e.ID))
	}
	if err := e.Save(); err != nil {
		return errorResult(fmt.Sprintf("failed to save persona: %v", err))
	}

	return textResult(fmt.Sprintf("Added %s (%s) to the council. File: %s\n"+
		"Run `council sync` in the project to update Claude Code and OpenCode configs.", e.Name, e.ID, e.Path()))
}

func textResult(text string) toolCallResult {
	return toolCallResult{Content: []toolContent{{Type: "text", Text: text}}}
}
