package review

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"

	"github.com/luuuc/council/internal/expert"
)

// Moderator is the pseudo-expert that closes a sequential review. It has no
// opinion and no vote: it lists where members disagree and what the human
// has to decide.
var Moderator = &expert.Expert{ID: "moderator", Name: "Moderator", Focus: "neutral summary of the debate"}

var moderatorTemplate = template.Must(template.New("moderator").Parse(`You are the moderator of a council review. You have no opinion and no vote.
Your job is to help the author decide: show where the members disagree and what
the author has to decide. Do not recommend an outcome and do not pick a side.

## Submission

` + "```" + `
{{.Content}}
` + "```" + `
{{if .Context}}
## Context

{{.Context}}
{{end}}
## The Debate
{{range .Prior}}
### {{if .Name}}{{.Name}}{{else}}{{.Expert}}{{end}} ({{.Expert}}) — {{.Verdict}}{{if .ChangedFrom}} (changed from {{.ChangedFrom}}{{if .ChangeReason}}: {{.ChangeReason}}{{end}}){{end}}
{{range .Notes}}
- {{.}}{{end}}{{range .Replies}}
- [{{.Stance}} {{.To}}] {{.Note}}{{end}}{{range .FinalWord}}
- final word [{{.Stance}} {{.To}}] {{.Note}}{{end}}
{{end}}
## Response Format

Respond with ONLY a JSON object matching this schema. No markdown, no code fences.

{"agreements":["<a point every member who addressed it accepts>"],"disagreements":[{"topic":"<the open question>","sides":[{"experts":["<expert-id>"],"position":"<what they hold>"}]}],"decisions":["<a question the author must answer, naming the trade-off>"]}

Rules:
- disagreements: only where members actually took different positions. Each
  side lists the expert ids who hold it. Leave it empty if they all agree.
- decisions: questions for the author, not recommendations. Name what each
  choice costs. Leave out anything every member agrees on.
- agreements: points nobody disputed that the author should act on.
- Members whose id starts with "customer-" speak for the users. Where what
  they need conflicts with what the experts want, make that a disagreement.

Respond with ONLY the JSON object. Nothing else.`))

// BuildModeratorPrompt builds the moderator prompt for a finished debate.
// sub.Prior holds every member's final verdict.
func BuildModeratorPrompt(sub Submission) string {
	var buf bytes.Buffer
	if err := moderatorTemplate.Execute(&buf, sub); err != nil {
		return "Summarize where these reviewers disagree and what the author must decide."
	}
	return buf.String()
}

// Moderation is the moderator's account of the debate.
type Moderation struct {
	Agreements    []string       `json:"agreements"`
	Disagreements []Disagreement `json:"disagreements"`
	Decisions     []string       `json:"decisions"`
}

// ParseModeration extracts the moderator's JSON. ok is false when the
// response has no usable moderation.
func ParseModeration(raw string, validIDs map[string]bool) (Moderation, bool) {
	text := strings.TrimSpace(raw)
	candidates := []string{text, extractFromCodeFence(text)}
	if start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}'); start >= 0 && end > start {
		candidates = append(candidates, text[start:end+1])
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		var m Moderation
		if err := json.Unmarshal([]byte(c), &m); err != nil {
			continue
		}
		m.Disagreements = cleanDisagreements(m.Disagreements, validIDs)
		m.Decisions = nonEmpty(m.Decisions)
		m.Agreements = nonEmpty(m.Agreements)
		if len(m.Disagreements) == 0 && len(m.Decisions) == 0 && len(m.Agreements) == 0 {
			continue
		}
		return m, true
	}
	return Moderation{}, false
}

// cleanDisagreements keeps sides held by real members and drops
// disagreements left with fewer than two sides.
func cleanDisagreements(ds []Disagreement, validIDs map[string]bool) []Disagreement {
	var out []Disagreement
	for _, d := range ds {
		var sides []Side
		for _, s := range d.Sides {
			var ids []string
			for _, id := range s.Experts {
				if validIDs[id] {
					ids = append(ids, id)
				}
			}
			if len(ids) > 0 && strings.TrimSpace(s.Position) != "" {
				sides = append(sides, Side{Experts: ids, Position: strings.TrimSpace(s.Position)})
			}
		}
		if strings.TrimSpace(d.Topic) != "" && len(sides) >= 2 {
			out = append(out, Disagreement{Topic: strings.TrimSpace(d.Topic), Sides: sides})
		}
	}
	return out
}

func nonEmpty(items []string) []string {
	var out []string
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// decodeJSONObject decodes the JSON object in a model answer into v: the
// whole text, a code fence, or the outermost braces. It reports success.
func decodeJSONObject(raw string, v any) bool {
	text := strings.TrimSpace(raw)
	candidates := []string{text, extractFromCodeFence(text)}
	if start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}'); start >= 0 && end > start {
		candidates = append(candidates, text[start:end+1])
	}
	for _, c := range candidates {
		if c != "" && json.Unmarshal([]byte(c), v) == nil {
			return true
		}
	}
	return false
}
