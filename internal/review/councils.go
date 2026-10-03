package review

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/luuuc/council/internal/expert"
)

// Council is one named group of members, such as the "product" pack.
type Council struct {
	Name   string
	Inputs []ExpertInput
}

// CouncilStatement is a council's spokesperson answering the other councils.
type CouncilStatement struct {
	Council    string  `json:"council"`
	Position   string  `json:"position"`
	Challenges []Reply `json:"challenges,omitempty"` // To is another council's name
}

// CouncilOutcome is one council's own review.
type CouncilOutcome struct {
	Name   string             `json:"name"`
	Result *SynthesizedResult `json:"result"`
}

// CouncilsResult is a review by several councils that then challenge each
// other's conclusions. Disagreements are between councils (Side.Experts
// holds council names); Decisions are for the human.
type CouncilsResult struct {
	Councils      []CouncilOutcome   `json:"councils"`
	Statements    []CouncilStatement `json:"statements"`
	Agreements    []string           `json:"agreements"`
	Disagreements []Disagreement     `json:"disagreements,omitempty"`
	Decisions     []string           `json:"decisions,omitempty"`
	Errors        []string           `json:"errors,omitempty"`
}

// CouncilHooks report progress while councils run. All are optional.
type CouncilHooks struct {
	OnCouncilStart func(name string, members int)
	OnCouncilDone  func(name string, result *SynthesizedResult)
	OnStatement    func(s CouncilStatement)
	OnCrossStart   func(label string) // before each spokesperson and the final moderator
}

// RunCouncils runs every council's debate at the same time (they are
// independent), then each council's spokesperson answers the other
// councils' conclusions, then a moderator lists where the councils
// disagree and what the human must decide.
//
// The Runner's OnStart hook fires for every turn, with Turn.Council set;
// OnVerdict doesn't fire, since councils finish out of order. OnCouncilDone
// fires in the given council order as each council finishes.
func (r *Runner) RunCouncils(ctx context.Context, councils []Council, sub Submission, hooks CouncilHooks) *CouncilsResult {
	out := &CouncilsResult{}

	results := make([]*SynthesizedResult, len(councils))
	done := make([]chan struct{}, len(councils))
	for i, c := range councils {
		if hooks.OnCouncilStart != nil {
			hooks.OnCouncilStart(c.Name, len(c.Inputs))
		}
		done[i] = make(chan struct{})
		cr := *r
		cr.OnVerdict = nil
		if r.OnStart != nil {
			cr.OnStart = func(t Turn) {
				t.Council = c.Name
				r.OnStart(t)
			}
		}
		go func(i int, c Council) {
			defer close(done[i])
			results[i] = cr.Run(ctx, c.Inputs, sub)
		}(i, c)
	}
	for i, c := range councils {
		<-done[i]
		out.Councils = append(out.Councils, CouncilOutcome{Name: c.Name, Result: results[i]})
		if hooks.OnCouncilDone != nil {
			hooks.OnCouncilDone(c.Name, results[i])
		}
	}
	if len(out.Councils) < 2 {
		return out
	}

	names := map[string]bool{}
	for _, c := range out.Councils {
		names[c.Name] = true
	}

	for _, c := range out.Councils {
		if hooks.OnCrossStart != nil {
			hooks.OnCrossStart("Spokesperson for the " + c.Name + " council")
		}
		raw, err := r.rawCall(ctx, &expert.Expert{ID: c.Name + "-council", Name: "The " + c.Name + " council"},
			buildSpokespersonPrompt(c.Name, out.Councils, sub))
		if err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%s spokesperson: %s", c.Name, err))
			continue
		}
		stmt, ok := parseStatement(raw, c.Name, names)
		if !ok {
			out.Errors = append(out.Errors, fmt.Sprintf("%s spokesperson: response could not be read", c.Name))
			continue
		}
		out.Statements = append(out.Statements, stmt)
		if hooks.OnStatement != nil {
			hooks.OnStatement(stmt)
		}
	}

	if hooks.OnCrossStart != nil {
		hooks.OnCrossStart("Moderator across councils")
	}
	raw, err := r.rawCall(ctx, Moderator, buildCrossModeratorPrompt(out, sub))
	if err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("moderator across councils: %s", err))
		return out
	}
	if m, ok := ParseModeration(raw, names); ok {
		out.Agreements = m.Agreements
		out.Disagreements = m.Disagreements
		out.Decisions = m.Decisions
	} else {
		log.Println("cross-council moderator response could not be read")
	}
	return out
}

// rawCall sends a prompt through the backend and returns the raw answer.
func (r *Runner) rawCall(ctx context.Context, e *expert.Expert, prompt string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()
	v, err := r.Backend.Review(callCtx, e, Submission{RawPrompt: prompt})
	if err != nil {
		return "", err
	}
	if len(v.Notes) == 0 {
		return "", nil
	}
	return v.Notes[0], nil
}

// parseStatement reads a spokesperson's JSON. Challenges must name another
// council.
func parseStatement(raw, council string, names map[string]bool) (CouncilStatement, bool) {
	var parsed struct {
		Position   string  `json:"position"`
		Challenges []Reply `json:"challenges"`
	}
	if !decodeJSONObject(raw, &parsed) || strings.TrimSpace(parsed.Position) == "" {
		return CouncilStatement{}, false
	}
	var challenges []Reply
	for _, c := range normalizeReplies(parsed.Challenges) {
		if names[c.To] && c.To != council {
			challenges = append(challenges, c)
		}
	}
	return CouncilStatement{Council: council, Position: strings.TrimSpace(parsed.Position), Challenges: challenges}, true
}

var digestTemplate = template.Must(template.New("digest").Parse(`### The {{.Name}} council
Members: {{range $i, $p := .Result.Perspectives}}{{if $i}}, {{end}}{{$p.Name}} ({{$p.Verdict}}){{end}}
{{- if .Result.Agreements}}
Nobody disputed:{{range .Result.Agreements}}
- {{.}}{{end}}{{end}}
{{- if .Result.Disagreements}}
Disagreed on:{{range .Result.Disagreements}}
- {{.Topic}}{{range .Sides}}
  - {{range $i, $e := .Experts}}{{if $i}}, {{end}}{{$e}}{{end}}: {{.Position}}{{end}}{{end}}
{{- else if .Result.Tension}}
Disagreed on:
{{.Result.Tension}}{{end}}
{{- if .Result.Decisions}}
Left for the author to decide:{{range .Result.Decisions}}
- {{.}}{{end}}{{end}}
`))

// councilDigest summarizes a council's review for the cross-council turns.
func councilDigest(c CouncilOutcome) string {
	var buf bytes.Buffer
	if err := digestTemplate.Execute(&buf, c); err != nil {
		return "### The " + c.Name + " council\n(summary unavailable)\n"
	}
	return buf.String()
}

func submissionBlock(sub Submission) string {
	var b strings.Builder
	b.WriteString("## Submission\n\n```\n" + sub.Content + "\n```\n")
	if sub.Context != "" {
		b.WriteString("\n## Context\n\n" + sub.Context + "\n")
	}
	return b.String()
}

func buildSpokespersonPrompt(name string, councils []CouncilOutcome, sub Submission) string {
	var own, others strings.Builder
	var otherNames []string
	for _, c := range councils {
		if c.Name == name {
			own.WriteString(councilDigest(c))
			continue
		}
		others.WriteString(councilDigest(c) + "\n")
		otherNames = append(otherNames, c.Name)
	}

	return fmt.Sprintf(`You speak for the %[1]s council. Several councils reviewed the same
submission separately. Now each council answers the others.

Represent your council faithfully: its conclusions, and its internal
disagreements where they matter. Do not invent positions its members didn't
take. Then challenge the other councils: where their conclusions conflict with
yours, what they missed from your council's angle, and where you back them.
Don't soften toward agreement; the conflicts between councils are the point.

%[2]s
## Your Council's Review

%[3]s
## The Other Councils

%[4]s
## Response Format

Respond with ONLY a JSON object. No markdown, no code fences.

{"position":"<your council's position in 2-3 sentences>","challenges":[{"to":"<one of: %[5]s>","stance":"<agree|disagree|adds>","note":"<your challenge or support>"}]}`,
		name, submissionBlock(sub), own.String(), others.String(), strings.Join(otherNames, ", "))
}

func buildCrossModeratorPrompt(r *CouncilsResult, sub Submission) string {
	var b strings.Builder
	for _, c := range r.Councils {
		b.WriteString(councilDigest(c) + "\n")
	}
	var stmts strings.Builder
	for _, s := range r.Statements {
		fmt.Fprintf(&stmts, "### The %s council's spokesperson\n%s\n", s.Council, s.Position)
		for _, c := range s.Challenges {
			fmt.Fprintf(&stmts, "- [%s %s] %s\n", c.Stance, c.To, c.Note)
		}
		stmts.WriteString("\n")
	}
	var names []string
	for _, c := range r.Councils {
		names = append(names, c.Name)
	}

	return fmt.Sprintf(`You are the moderator across several councils that reviewed the same
submission. You have no opinion and no vote. Help the author decide: show
where the councils disagree and what the author has to decide. Do not
recommend an outcome and do not pick a side.

%s
## Each Council's Review

%s
## The Councils Answer Each Other

%s
## Response Format

Respond with ONLY a JSON object matching this schema. No markdown, no code fences.

{"agreements":["<a point no council disputed>"],"disagreements":[{"topic":"<the open question>","sides":[{"experts":["<council name: one of %s>"],"position":"<what that council holds>"}]}],"decisions":["<a question the author must answer, naming the trade-off>"]}

Rules:
- disagreements: between councils, not within one. Each side lists council names.
- decisions: questions for the author, not recommendations. Name what each choice costs.

Respond with ONLY the JSON object. Nothing else.`,
		submissionBlock(sub), b.String(), stmts.String(), strings.Join(names, ", "))
}
