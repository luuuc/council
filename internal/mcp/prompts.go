package mcp

import (
	"encoding/json"
	"fmt"
)

// MCP prompts appear in the client's prompt menu (e.g. Claude Desktop),
// so users can convene the council without knowing the tool names.

type promptsCapability struct{}

type promptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

type promptDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []promptArgument `json:"arguments,omitempty"`
}

type promptMessage struct {
	Role    string      `json:"role"`
	Content toolContent `json:"content"`
}

type promptGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
}

var councilPrompt = promptDefinition{
	Name:        "council",
	Description: "Convene the council on code, a document, a plan, or a decision",
	Arguments: []promptArgument{
		{Name: "pack", Description: "Pack to convene (e.g. go, rails, writing)", Required: true},
		{Name: "topic", Description: "What the council should review"},
	},
}

var assemblePrompt = promptDefinition{
	Name:        "assemble",
	Description: "Assemble or extend the council: your AI proposes members for this project, you choose",
	Arguments: []promptArgument{
		{Name: "focus", Description: "Optional: what the council should be good at (e.g. product launch, security)"},
	},
}

func (s *Server) handlePromptsList(req *jsonrpcRequest) {
	s.sendResult(req.ID, map[string]any{"prompts": []promptDefinition{councilPrompt, assemblePrompt}})
}

func (s *Server) handlePromptsGet(req *jsonrpcRequest) {
	var params promptGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, errCodeInvalidParams, "invalid params", err.Error())
		return
	}
	if params.Name == assemblePrompt.Name {
		text := "Assemble a council for this project: call council_assemble and follow the brief it returns, step by step."
		if focus := params.Arguments["focus"]; focus != "" {
			text += " The council should be good at: " + focus + "."
		}
		s.sendResult(req.ID, map[string]any{
			"description": assemblePrompt.Description,
			"messages":    []promptMessage{{Role: "user", Content: toolContent{Type: "text", Text: text}}},
		})
		return
	}
	if params.Name != councilPrompt.Name {
		s.sendError(req.ID, errCodeInvalidParams, "unknown prompt", params.Name)
		return
	}

	s.sendResult(req.ID, map[string]any{
		"description": councilPrompt.Description,
		"messages": []promptMessage{{
			Role:    "user",
			Content: toolContent{Type: "text", Text: councilInstructions(params.Arguments["pack"], params.Arguments["topic"])},
		}},
	})
}

// councilInstructions tells the client's model how to run the council with
// the council tools.
func councilInstructions(pack, topic string) string {
	if topic == "" {
		topic = "what we are working on in this conversation"
	}
	return fmt.Sprintf(`Convene the %q council on: %s

1. Work out what the council reviews: code, a diff, a document, or for a
   question, plan, or decision, a short brief with the question, the relevant
   context from this conversation, and the options being considered.
2. Call council_review with pack %q and that content. If it reports that no AI
   backend is available, call council_convene instead and take each member's
   turn, passing each review to council_turn until the council finishes.
3. Present the debate: each member's verdict, notes, and replies in the order
   they spoke. Keep the disagreements visible; do not merge them into a
   consensus. End with the open trade-offs and what I have to decide. I make
   the call, not the council and not you.`, pack, topic, pack)
}
