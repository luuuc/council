package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/luuuc/council-cli/internal/expert"
	"github.com/luuuc/council-cli/internal/pack"
	"github.com/luuuc/council-cli/internal/review"
)

// handleReview implements the council_review MCP tool.
func (s *Server) handleReview(ctx context.Context, args map[string]any) toolCallResult {
	packName, ok := args["pack"].(string)
	if !ok || packName == "" {
		return errorResult("missing required field: pack")
	}
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return errorResult("missing required field: content")
	}

	// Resolve pack and experts
	p, err := pack.Get(packName)
	if err != nil {
		return errorResult(fmt.Sprintf("pack %q not found: %v", packName, err))
	}

	available, err := expert.List()
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list experts: %v", err))
	}

	resolved, _ := pack.Resolve(p, available)
	if len(resolved) == 0 {
		return errorResult(fmt.Sprintf("no experts resolved for pack %q", packName))
	}

	inputs := make([]review.ExpertInput, len(resolved))
	for i, rm := range resolved {
		inputs[i] = review.ExpertInput{
			Expert:   rm.Expert,
			Blocking: rm.Blocking,
		}
	}

	sub := review.Submission{Content: content}

	// Get backend (also caches config)
	backend, err := s.getBackend()
	if err != nil {
		return errorResult(fmt.Sprintf("backend error: %v", err))
	}

	runner := &review.Runner{
		Backend: backend,
		Options: review.ReviewOptions{
			Concurrency: s.config.AI.Concurrency,
			Timeout:     s.config.AI.Timeout,
		},
	}

	result := runner.Run(ctx, inputs, sub)

	data, err := review.FormatJSON(result)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal result: %v", err))
	}

	return toolCallResult{
		Content: []toolContent{{Type: "text", Text: string(data)}},
	}
}

// listExpertInfo is the JSON structure returned by council_list.
type listExpertInfo struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Focus    string           `json:"focus"`
	Blocking bool             `json:"blocking"`
	Tensions []expert.Tension `json:"tensions,omitempty"`
}

// handleList implements the council_list MCP tool.
func (s *Server) handleList(args map[string]any) toolCallResult {
	packName, ok := args["pack"].(string)
	if !ok || packName == "" {
		return errorResult("missing required field: pack")
	}

	p, err := pack.Get(packName)
	if err != nil {
		return errorResult(fmt.Sprintf("pack %q not found: %v", packName, err))
	}

	available, err := expert.List()
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list experts: %v", err))
	}

	resolved, _ := pack.Resolve(p, available)

	experts := make([]listExpertInfo, len(resolved))
	for i, rm := range resolved {
		experts[i] = listExpertInfo{
			ID:       rm.Expert.ID,
			Name:     rm.Expert.Name,
			Focus:    rm.Expert.Focus,
			Blocking: rm.Blocking,
			Tensions: rm.Expert.Tensions,
		}
	}

	data, err := json.MarshalIndent(map[string]any{
		"pack":    p.Name,
		"experts": experts,
	}, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal result: %v", err))
	}

	return toolCallResult{
		Content: []toolContent{{Type: "text", Text: string(data)}},
	}
}

// explainTemplate is the prompt for the council_explain tool.
var explainTemplate = template.Must(template.New("explain").Parse(`You are {{.Expert.Name}}, an expert in {{.Expert.Focus}}.

## Your Persona

{{.Expert.Body}}

## Task

A council review flagged the following note:

> {{.Note}}

Explain your reasoning in depth. Specifically:
1. Which of your principles or red flags triggered this observation?
2. What does your worldview say about this pattern — why does it matter?
3. What would you recommend as a concrete fix or improvement?

Be direct, specific, and grounded in your expertise. Speak in first person as {{.Expert.Name}}.`))

type explainData struct {
	Expert *expert.Expert
	Note   string
}

// handleExplain implements the council_explain MCP tool.
func (s *Server) handleExplain(ctx context.Context, args map[string]any) toolCallResult {
	expertID, ok := args["expert"].(string)
	if !ok || expertID == "" {
		return errorResult("missing required field: expert")
	}
	note, ok := args["note"].(string)
	if !ok || note == "" {
		return errorResult("missing required field: note")
	}

	e, err := expert.Load(expertID)
	if err != nil {
		return errorResult(fmt.Sprintf("expert %q not found: %v", expertID, err))
	}

	// Build the explain prompt — uses RawPrompt to bypass the review prompt
	// template and ParseVerdict in the backend. The LLM returns natural
	// language, which comes back in verdict.Notes[0].
	var buf bytes.Buffer
	if err := explainTemplate.Execute(&buf, explainData{Expert: e, Note: note}); err != nil {
		return errorResult(fmt.Sprintf("failed to build prompt: %v", err))
	}

	backend, err := s.getBackend()
	if err != nil {
		return errorResult(fmt.Sprintf("backend error: %v", err))
	}

	sub := review.Submission{RawPrompt: buf.String()}
	verdict, err := backend.Review(ctx, e, sub)
	if err != nil {
		return errorResult(fmt.Sprintf("explain failed: %v", err))
	}

	// The backend returns the raw LLM text in Notes[0] when RawPrompt is set.
	explanation := ""
	if len(verdict.Notes) > 0 {
		explanation = verdict.Notes[0]
	}

	return toolCallResult{
		Content: []toolContent{{Type: "text", Text: explanation}},
	}
}

func errorResult(msg string) toolCallResult {
	return toolCallResult{
		Content: []toolContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}
