package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/pack"
	"github.com/luuuc/council/internal/review"
)

// handleRoom implements the council_room MCP tool: the room prompt the
// client's own model answers in one pass.
func (s *Server) handleRoom(args map[string]any) toolCallResult {
	content, _ := args["content"].(string)
	if strings.TrimSpace(content) == "" {
		return errorResult("missing required field: content")
	}
	councils, err := roomCouncils(args)
	if err != nil {
		return errorResult(err.Error())
	}
	background, _ := args["context"].(string)

	councils, seatNotes := review.SeatCouncils(councils)
	var notes string
	for _, n := range seatNotes {
		notes += "Note for the user: " + n + "\n"
	}
	if notes != "" {
		notes += "\n"
	}
	prompt := review.BuildRoomPrompt(councils, review.Submission{Content: content, Context: background})
	return textResult(notes + prompt + "\n\n----- after you answer -----\n\n" +
		"Call council_record with " + sameCouncils(args) + " and answer set to the JSON object. " +
		"Council checks it and returns the review to show the user.")
}

// handleRecord implements the council_record MCP tool: it checks the
// answer to the room prompt, saves the review, and returns it rendered.
func (s *Server) handleRecord(args map[string]any) toolCallResult {
	answer, _ := args["answer"].(string)
	if strings.TrimSpace(answer) == "" {
		return errorResult("missing required field: answer")
	}
	councils, err := roomCouncils(args)
	if err != nil {
		return errorResult(err.Error())
	}
	councils, _ = review.SeatCouncils(councils)
	result, err := review.ParseRoom(answer, councils)
	if err != nil {
		return errorResult(err.Error() + "\n\nCall council_record again with " + sameCouncils(args) + " and the fixed answer.")
	}

	text := review.FormatRoom(result)
	var names []string
	for _, c := range councils {
		names = append(names, c.Name)
	}
	if path, err := review.Save(text, strings.Join(names, "-"), time.Now()); err == nil {
		text += "\nSaved to " + path + "\n"
	}
	return textResult(text)
}

// roomCouncils resolves the councils in the room: several ("councils"),
// one pack ("pack"), or every member.
func roomCouncils(args map[string]any) ([]review.Council, error) {
	if list, _ := args["councils"].(string); list != "" {
		return resolveCouncils(list)
	}
	if name, _ := args["pack"].(string); name != "" {
		inputs, err := resolvePackInputs(name)
		if err != nil {
			return nil, err
		}
		return []review.Council{{Name: name, Inputs: inputs}}, nil
	}
	experts, err := expert.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list experts: %v", err)
	}
	if len(experts) == 0 {
		return nil, fmt.Errorf("the council has no members yet: assemble one first (council_assemble)")
	}
	inputs := make([]review.ExpertInput, len(experts))
	for i, e := range experts {
		inputs[i] = review.ExpertInput{Expert: e}
	}
	return []review.Council{{Inputs: inputs}}, nil
}

func sameCouncils(args map[string]any) string {
	if list, _ := args["councils"].(string); list != "" {
		return fmt.Sprintf("councils %q", list)
	}
	if name, _ := args["pack"].(string); name != "" {
		return fmt.Sprintf("pack %q", name)
	}
	return "no pack"
}

// resolvePackInputs resolves a pack's members into review inputs, in pack order.
func resolvePackInputs(packName string) ([]review.ExpertInput, error) {
	p, err := pack.Get(packName)
	if err != nil {
		return nil, fmt.Errorf("pack %q not found: %v", packName, err)
	}

	available, err := expert.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list experts: %v", err)
	}

	resolved, _ := pack.Resolve(p, available)
	if len(resolved) == 0 {
		return nil, fmt.Errorf("no experts resolved for pack %q", packName)
	}

	inputs := make([]review.ExpertInput, len(resolved))
	for i, rm := range resolved {
		inputs[i] = review.ExpertInput{
			Expert:   rm.Expert,
			Blocking: rm.Blocking,
		}
	}
	return inputs, nil
}

// resolveCouncils resolves a comma-separated list of packs into councils.
func resolveCouncils(list string) ([]review.Council, error) {
	var councils []review.Council
	seen := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		inputs, err := resolvePackInputs(name)
		if err != nil {
			return nil, err
		}
		councils = append(councils, review.Council{Name: name, Inputs: inputs})
	}
	if len(councils) < 2 {
		return nil, fmt.Errorf("councils needs at least two packs, e.g. \"product,security\"")
	}
	return councils, nil
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

	return textResult(string(data))
}

func errorResult(msg string) toolCallResult {
	return toolCallResult{
		Content: []toolContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}
