package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/review"
)

// session is a council run where the MCP client's own model takes each
// member's turn. Council keeps the order, builds each member's prompt with
// the earlier reviews, and synthesizes the result. This works in clients
// with no headless mode and no API key, such as Claude Desktop.
type session struct {
	inputs   []review.ExpertInput
	sub      review.Submission
	verdicts []review.ExpertVerdict
	turn     int // index of the member whose review is expected next
}

// handleConvene implements the council_convene MCP tool.
func (s *Server) handleConvene(args map[string]any) toolCallResult {
	packName, ok := args["pack"].(string)
	if !ok || packName == "" {
		return errorResult("missing required field: pack")
	}
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return errorResult("missing required field: content")
	}
	background, _ := args["context"].(string)

	inputs, err := resolvePackInputs(packName)
	if err != nil {
		return errorResult(err.Error())
	}

	id, err := newSessionID()
	if err != nil {
		return errorResult(fmt.Sprintf("failed to start session: %v", err))
	}
	if s.sessions == nil {
		s.sessions = map[string]*session{}
	}
	sess := &session{inputs: inputs, sub: review.Submission{Content: content, Context: background}}
	s.sessions[id] = sess

	names := make([]string, len(inputs))
	for i, inp := range inputs {
		names[i] = inp.Expert.Name
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Council session %s: %d members will speak in this order: %s.\n\n",
		id, len(inputs), strings.Join(names, ", "))
	b.WriteString(turnPrompt(id, sess))
	return textResult(b.String())
}

// handleTurn implements the council_turn MCP tool.
func (s *Server) handleTurn(args map[string]any) toolCallResult {
	id, ok := args["session"].(string)
	if !ok || id == "" {
		return errorResult("missing required field: session")
	}
	raw, ok := args["review"].(string)
	if !ok || raw == "" {
		return errorResult("missing required field: review")
	}

	sess, ok := s.sessions[id]
	if !ok {
		return errorResult(fmt.Sprintf("unknown or finished session %q: start a new one with council_convene", id))
	}

	inp := sess.inputs[sess.turn]
	verdict := review.ParseVerdict(inp.Expert.ID, []byte(raw))
	if verdict.Error != "" {
		return errorResult(fmt.Sprintf("could not read %s's review: it must be a single JSON object in the format the prompt asks for. "+
			"Call council_turn again with session %q and the corrected JSON.", inp.Expert.Name, id))
	}
	verdict.Name = inp.Expert.Name
	verdict.Blocking = inp.Blocking
	sess.verdicts = append(sess.verdicts, verdict)
	sess.turn++

	if sess.turn < len(sess.inputs) {
		return textResult(turnPrompt(id, sess))
	}

	delete(s.sessions, id)
	experts := make([]*expert.Expert, len(sess.inputs))
	for i, in := range sess.inputs {
		experts[i] = in.Expert
	}
	result := review.Synthesize(sess.verdicts, experts, nil)

	var b strings.Builder
	b.WriteString("The council has finished. Present the debate to the user:\n" +
		"- Each member's verdict, notes, and replies, in the order they spoke.\n" +
		"- Keep the disagreements visible. Do not merge them into a consensus.\n" +
		"- End with the open trade-offs: where members disagree and what the user has to decide.\n\n")
	b.WriteString(review.FormatHuman(result, "", len(experts)))
	return textResult(b.String())
}

// turnPrompt tells the client's model whose turn it is and gives it that
// member's review prompt, including the earlier reviews.
func turnPrompt(id string, sess *session) string {
	inp := sess.inputs[sess.turn]
	turn := sess.sub
	turn.Prior = sess.verdicts

	var b strings.Builder
	fmt.Fprintf(&b, "Turn %d of %d: %s.\n\n", sess.turn+1, len(sess.inputs), inp.Expert.Name)
	fmt.Fprintf(&b, "Write this member's review by following the prompt below as %s. "+
		"Speak only for them: hold their positions even where they clash with earlier members, "+
		"and do not soften the review toward a consensus. "+
		"Then call council_turn with session %q and review set to the JSON object only.\n\n", inp.Expert.Name, id)
	b.WriteString("----- prompt for " + inp.Expert.Name + " -----\n\n")
	b.WriteString(review.BuildPrompt(inp.Expert, turn))
	return b.String()
}

// handleAddPersona implements the council_add_persona MCP tool.
func (s *Server) handleAddPersona(args map[string]any) toolCallResult {
	raw, ok := args["persona"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return errorResult("missing required field: persona")
	}
	if !config.Exists() {
		return errorResult("no council in this directory: start the server with `council mcp --dir <project>` for a project that has run `council start`")
	}

	text := strings.TrimSpace(raw)
	if !strings.HasPrefix(text, "---") {
		text = "---\n" + text + "\n---"
	}
	e, err := expert.Parse([]byte(text))
	if err != nil {
		return errorResult(fmt.Sprintf("could not read persona: %v", err))
	}
	if strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.Focus) == "" {
		return errorResult("persona needs at least name and focus")
	}

	expert.NormalizeVirtual(e, e.Name)
	if expert.Exists(e.ID) {
		return errorResult(fmt.Sprintf("expert %q already exists", e.ID))
	}
	if err := e.Save(); err != nil {
		return errorResult(fmt.Sprintf("failed to save persona: %v", err))
	}

	return textResult(fmt.Sprintf("Added %s (%s) to the council. File: %s\n"+
		"Run `council sync` in the project to update Claude Code and OpenCode configs.", e.Name, e.ID, e.Path()))
}

// addPersonaFormat documents the persona YAML for the council_add_persona tool.
const addPersonaFormat = `name: Their Full Name
focus: What they are known for (max 60 chars)
influences:
  - "A real talk, book, essay, or project — what it shows about their views"
philosophy: |
  2-4 sentences in first person, faithful to their documented views.
principles:
  - A position they have argued for publicly
red_flags:
  - A pattern they would push back on in a review
tensions:
  - expert: existing-member-id
    topic: what they disagree about
    position: this person's documented position
    counterpoint: the other member's position`

func newSessionID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func textResult(text string) toolCallResult {
	return toolCallResult{Content: []toolContent{{Type: "text", Text: text}}}
}
