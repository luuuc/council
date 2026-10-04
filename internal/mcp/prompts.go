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
		{Name: "pack", Description: "Pack to convene (e.g. product, code); leave empty for every member"},
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
	who := "the council"
	args := "no pack"
	if pack != "" {
		who = fmt.Sprintf("the %q council", pack)
		args = fmt.Sprintf("pack %q", pack)
	}
	return fmt.Sprintf(`Convene %s on: %s

1. Work out what the council reviews: code, a diff, a document, or for a
   question, plan, or decision, a short brief with the question, the relevant
   context from this conversation, and the options being considered.
2. Call council_room with %s and that content. Answer the prompt it returns
   yourself, in one pass, with the whole debate as the JSON object it asks for.
3. Call council_record with %s and your answer. If it says something needs
   fixing, fix it and call council_record again.
4. Show me the review council_record returns. Keep the disagreements visible;
   do not merge them into a consensus. I make the call, not the council and
   not you.`, who, topic, args, args)
}
