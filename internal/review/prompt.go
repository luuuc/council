package review

import (
	"bytes"
	"text/template"

	"github.com/luuuc/council/internal/expert"
)

var promptTemplate = template.Must(template.New("review-prompt").Parse(`You are {{.Expert.Name}}, reviewing a submission as part of a council review. It may be code, a plan, a piece of writing, or a decision.

## Your Persona

{{.Expert.Body}}
{{if .Submission.Prior}}
## The Council So Far

You speak after these members. Read their reviews. Do not repeat what they said.
Stay true to your own views: push back where you disagree, back them up where
you agree for your own reasons, and add what they missed. Do not agree just to
be agreeable — the disagreements are the most useful part of the review.
{{range .Submission.Prior}}
### {{if .Name}}{{.Name}}{{else}}{{.Expert}}{{end}} ({{.Expert}}) — {{.Verdict}}
{{range .Notes}}
- {{.}}{{end}}{{range .Replies}}
- [{{.Stance}} {{.To}}] {{.Note}}{{end}}
{{end}}{{end}}
## Submission

` + "```" + `
{{.Submission.Content}}
` + "```" + `
{{if .Submission.Context}}
## Context

{{.Submission.Context}}
{{end}}
## Response Format

You MUST respond with ONLY a JSON object matching this exact schema. No markdown, no code fences, no explanation before or after.

{"expert":"{{.Expert.ID}}","verdict":"<pass|comment|block|escalate>","confidence":<0.0-1.0>,"notes":["<observation 1>","<observation 2>"],{{if .Submission.Prior}}"replies":[{"to":"<expert-id>","stance":"<agree|disagree|adds>","note":"<your reaction to their point>"}],{{end}}"blocking":false}

Field definitions:
- verdict: "pass" (no issues), "comment" (suggestions worth considering), "block" (must fix before shipping), "escalate" (beyond your expertise to judge)
- confidence: how confident you are in your assessment, from 0.0 to 1.0
- notes: specific observations from your area of expertise — be direct and concrete
- blocking: true only if this is a blocking issue that must be resolved
{{- if .Submission.Prior}}
- replies: your reactions to earlier members, by their expert id — "disagree" (you think they are wrong), "agree" (you back them, with your own reason), "adds" (you build on their point). Use an empty list if you have nothing to say to them.
{{- end}}

Respond with ONLY the JSON object. Nothing else.`))

type promptData struct {
	Expert     *expert.Expert
	Submission Submission
}

// BuildPrompt constructs the review prompt for an expert and submission.
func BuildPrompt(e *expert.Expert, sub Submission) string {
	var buf bytes.Buffer
	if err := promptTemplate.Execute(&buf, promptData{Expert: e, Submission: sub}); err != nil {
		// Fallback to a minimal prompt if template fails
		return "Review this code as " + e.Name + ":\n\n" + sub.Content
	}
	return buf.String()
}

var collectiveTemplate = template.Must(template.New("collective-prompt").Parse(`You are a council of expert reviewers. Review the submission below from each expert's perspective independently, then synthesize.

## Experts
{{range .Experts}}
### {{.Name}} — {{.Focus}}

{{.Body}}
{{end}}
## Submission

` + "```" + `
{{.Submission.Content}}
` + "```" + `
{{if .Submission.Context}}
## Context

{{.Submission.Context}}
{{end}}
## Instructions

Review the submission from each expert's perspective. Experts should react to each other — if one expert raises a concern that another would challenge, say so. The tension between perspectives is the most valuable part.

Respond with ONLY a JSON object matching this exact schema. No markdown, no code fences, no explanation before or after.

{"verdict":"<pass|comment|block|escalate>","blocking":false,"perspectives":[{"expert":"<expert-id>","verdict":"<pass|comment|block|escalate>","confidence":<0.0-1.0>,"notes":["<observation>"],"blocking":false}],"agreements":["<things all experts agree on>"],"tension":"<where experts disagree and why>","summary":"<one-line recommendation>"}

Field definitions:
- verdict: overall recommendation — "pass" (no issues), "comment" (suggestions), "block" (must fix), "escalate" (beyond expertise)
- perspectives: one entry per expert with their individual assessment
- agreements: observations that all experts share
- tension: where experts disagree — articulate both sides
- summary: one-line recommendation for the author

Each perspective must be substantive — skip an expert rather than produce a generic observation. Respond with ONLY the JSON object. Nothing else.`))

type collectivePromptData struct {
	Experts    []*expert.Expert
	Submission Submission
}

// BuildCollectivePrompt constructs a single prompt for all experts to review together.
func BuildCollectivePrompt(experts []*expert.Expert, sub Submission) string {
	var buf bytes.Buffer
	if err := collectiveTemplate.Execute(&buf, collectivePromptData{Experts: experts, Submission: sub}); err != nil {
		return "Review this code as a council of experts:\n\n" + sub.Content
	}
	return buf.String()
}
