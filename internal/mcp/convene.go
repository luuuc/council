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
// turn. Council keeps the order, builds each prompt with the earlier
// reviews, and synthesizes the result. This works in clients with no
// headless mode and no API key, such as Claude Desktop.
type session struct {
	debate   steps
	councils *review.CouncilsDebate // set when several councils debate
	turn     review.Turn            // the turn whose answer is expected next
}

// steps is a debate run one turn at a time: one council (review.Debate)
// or several (review.CouncilsDebate).
type steps interface {
	Next() (review.Turn, bool)
	RecordVerdict(review.ExpertVerdict)
	RecordRaw(review.TurnKind, string) bool
}

// handleConvene implements the council_convene MCP tool.
func (s *Server) handleConvene(args map[string]any) toolCallResult {
	packName, _ := args["pack"].(string)
	if list, _ := args["councils"].(string); packName == "" && list == "" {
		return errorResult("missing required field: pack (or councils)")
	}
	content, ok := args["content"].(string)
	if !ok || content == "" {
		return errorResult("missing required field: content")
	}
	background, _ := args["context"].(string)

	sub := review.Submission{Content: content, Context: background}
	var sess *session
	var intro string
	if list, _ := args["councils"].(string); list != "" {
		councils, err := resolveCouncils(list)
		if err != nil {
			return errorResult(err.Error())
		}
		cd := review.NewCouncilsDebate(councils, sub, true, true)
		sess = &session{debate: cd, councils: cd}
		var names []string
		for _, c := range councils {
			names = append(names, c.Name)
		}
		intro = fmt.Sprintf("%d councils will each debate in turn (%s); then each council's spokesperson answers the others, "+
			"and a moderator lists where the councils disagree and what to decide.", len(councils), strings.Join(names, ", "))
	} else {
		inputs, err := resolvePackInputs(packName)
		if err != nil {
			return errorResult(err.Error())
		}
		names := make([]string, len(inputs))
		for i, inp := range inputs {
			names[i] = inp.Expert.Name
		}
		sess = &session{debate: review.NewDebate(inputs, sub, true, true)}
		intro = fmt.Sprintf("%d members will speak in this order: %s. "+
			"Then earlier members get a final word, and a moderator lists disagreements and decisions.", len(inputs), strings.Join(names, ", "))
	}

	id, err := newSessionID()
	if err != nil {
		return errorResult(fmt.Sprintf("failed to start session: %v", err))
	}
	if s.sessions == nil {
		s.sessions = map[string]*session{}
	}
	turn, _ := sess.debate.Next()
	sess.turn = turn
	s.sessions[id] = sess

	var b strings.Builder
	fmt.Fprintf(&b, "Council session %s: %s\n\n", id, intro)
	b.WriteString(turnPrompt(id, sess.turn))
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

	retry := fmt.Sprintf("could not read the answer for %s: it must be a single JSON object in the format the prompt asks for. "+
		"Call council_turn again with session %q and the corrected JSON.", sess.turn.Label(), id)
	if k := sess.turn.Kind; k == review.TurnModerator || k == review.TurnSpokesperson || k == review.TurnCrossModerator {
		// Raw turns are consumed even when unreadable, so a summary the
		// model can't produce doesn't loop forever.
		sess.debate.RecordRaw(k, raw)
	} else {
		verdict := review.ParseVerdict(sess.turn.Expert.ID, []byte(raw))
		if verdict.Error != "" {
			return errorResult(retry)
		}
		sess.debate.RecordVerdict(verdict)
	}

	next, more := sess.debate.Next()
	if !more {
		return s.finish(id, sess)
	}
	sess.turn = next
	return textResult(turnPrompt(id, next))
}

// finish ends a session and returns the debate for the client to present.
func (s *Server) finish(id string, sess *session) toolCallResult {
	delete(s.sessions, id)

	if sess.councils != nil {
		var b strings.Builder
		b.WriteString("The councils have finished. Present each council's debate briefly, then how the councils answered each other, " +
			"then where the councils disagree and what the user has to decide. Do not recommend an outcome.\n\n")
		b.WriteString(review.FormatHumanCouncils(sess.councils.Result()))
		return textResult(b.String())
	}
	result := sess.debate.(*review.Debate).Result()

	var b strings.Builder
	b.WriteString("The council has finished. Present the debate to the user:\n" +
		"- Each member's verdict, notes, and replies, in the order they spoke, then the final words.\n" +
		"- Keep the disagreements visible. Do not merge them into a consensus or recommend an outcome.\n" +
		"- End with where they disagree and what the user has to decide. The user makes the call.\n\n")
	b.WriteString(review.FormatHuman(result, "", len(result.Perspectives)))
	return textResult(b.String())
}

// turnPrompt tells the client's model whose turn it is and gives it that
// turn's prompt.
func turnPrompt(id string, t review.Turn) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Turn %d of up to %d: %s.\n\n", t.Number, t.Total, t.Label())
	switch t.Kind {
	case review.TurnSpokesperson:
		fmt.Fprintf(&b, "Speak for the %s council by following the prompt below: represent its conclusions faithfully and challenge the other councils. ", t.Council)
	case review.TurnCrossModerator:
		b.WriteString("Write the moderator's summary across councils by following the prompt below. Stay neutral: no opinion, no vote, no recommendation. ")
	case review.TurnModerator:
		b.WriteString("Write the moderator's summary by following the prompt below. Stay neutral: no opinion, no vote, no recommendation. ")
	case review.TurnFinalWord:
		fmt.Fprintf(&b, "Write %s's final word by following the prompt below. Hold their positions; concede only where the later members really convinced them. ", t.Expert.Name)
	default:
		fmt.Fprintf(&b, "Write this member's review by following the prompt below as %s. "+
			"Speak only for them: hold their positions even where they clash with earlier members, "+
			"and do not soften the review toward a consensus. ", t.Expert.Name)
	}
	fmt.Fprintf(&b, "Then call council_turn with session %q and review set to the JSON object only.\n\n", id)
	b.WriteString("----- prompt: " + t.Label() + " -----\n\n")
	b.WriteString(t.Prompt())
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

	kind, _ := args["kind"].(string)
	switch kind {
	case "", "person":
		expert.NormalizeVirtual(e, e.Name)
	case expert.KindRole:
		expert.NormalizeRole(e, e.Name)
	case expert.KindCustomer:
		expert.NormalizeCustomer(e)
	default:
		return errorResult(fmt.Sprintf("unknown kind %q: use person, role, or customer", kind))
	}
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
		return nil, fmt.Errorf("councils needs at least two packs, e.g. \"product,security,code\"")
	}
	return councils, nil
}
