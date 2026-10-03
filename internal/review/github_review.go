package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GitHubReviewEvent is the action type for a PR review.
type GitHubReviewEvent string

const (
	GitHubApprove        GitHubReviewEvent = "APPROVE"
	GitHubRequestChanges GitHubReviewEvent = "REQUEST_CHANGES"
	GitHubComment        GitHubReviewEvent = "COMMENT"
)

// GitHubReview is the payload for the GitHub PR Reviews API (POST /repos/{owner}/{repo}/pulls/{number}/reviews).
type GitHubReview struct {
	Event    GitHubReviewEvent     `json:"event"`
	Body     string                `json:"body"`
	Comments []GitHubReviewComment `json:"comments,omitempty"`
}

// GitHubReviewComment is an inline comment in a PR review.
type GitHubReviewComment struct {
	Path     string `json:"path"`
	Position int    `json:"position"`
	Body     string `json:"body"`
}

// GitHubCheckRun is the payload for a GitHub Check Run annotation.
type GitHubCheckRun struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Conclusion string            `json:"conclusion"`
	Output     GitHubCheckOutput `json:"output"`
}

// GitHubCheckOutput is the output section of a Check Run.
type GitHubCheckOutput struct {
	Title       string                  `json:"title"`
	Summary     string                  `json:"summary"`
	Annotations []GitHubCheckAnnotation `json:"annotations,omitempty"`
}

// GitHubCheckAnnotation is an inline annotation in a Check Run.
type GitHubCheckAnnotation struct {
	Path            string `json:"path"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	AnnotationLevel string `json:"annotation_level"`
	Message         string `json:"message"`
	Title           string `json:"title,omitempty"`
}

// GitHubOutput bundles both the PR Review and Check Run payloads.
type GitHubOutput struct {
	Review   GitHubReview   `json:"review"`
	CheckRun GitHubCheckRun `json:"check_run"`
}

// MapVerdictToEvent converts a Council verdict to a GitHub review event.
func MapVerdictToEvent(v Verdict, blocking bool) GitHubReviewEvent {
	if blocking || v == VerdictBlock || v == VerdictEscalate {
		return GitHubRequestChanges
	}
	if v == VerdictPass {
		return GitHubApprove
	}
	return GitHubComment
}

// FormatGitHubReview builds the full GitHub PR Review payload from a SynthesizedResult.
// The diffPositions map is used to convert file/line references to diff-relative positions.
// If diffPositions is nil, no inline comments are generated.
func FormatGitHubReview(result *SynthesizedResult, packName string, expertCount int, dp *DiffPosition) GitHubOutput {
	event := MapVerdictToEvent(result.Verdict, result.Blocking)

	body := formatReviewBody(result, packName, expertCount)

	var comments []GitHubReviewComment
	var annotations []GitHubCheckAnnotation

	if dp != nil {
		comments, annotations = extractInlineComments(result, dp)
	}

	// Fallback comments (lines not in diff) go into the body
	var fallbacks []string
	var mappedComments []GitHubReviewComment
	for _, c := range comments {
		if c.Position == 0 {
			fallbacks = append(fallbacks, c.Body)
		} else {
			mappedComments = append(mappedComments, c)
		}
	}
	if len(fallbacks) > 0 {
		body += "\n\n### Additional Comments\n\n" + strings.Join(fallbacks, "\n\n")
	}

	// Check Run
	conclusion := "success"
	if event == GitHubRequestChanges {
		conclusion = "action_required"
	}

	checkTitle := fmt.Sprintf("%d experts reviewed, verdict: %s", expertCount, result.Verdict)
	checkSummary := body

	return GitHubOutput{
		Review: GitHubReview{
			Event:    event,
			Body:     body,
			Comments: mappedComments,
		},
		CheckRun: GitHubCheckRun{
			Name:       "Council Review",
			Status:     "completed",
			Conclusion: conclusion,
			Output: GitHubCheckOutput{
				Title:       checkTitle,
				Summary:     checkSummary,
				Annotations: annotations,
			},
		},
	}
}

// FormatGitHubJSON marshals the GitHub output as indented JSON.
func FormatGitHubJSON(output GitHubOutput) ([]byte, error) {
	return json.MarshalIndent(output, "", "  ")
}

func formatReviewBody(result *SynthesizedResult, packName string, expertCount int) string {
	var b strings.Builder

	b.WriteString("## Council Review\n\n")

	fmt.Fprintf(&b, "**Votes:** %s.", voteCount(result.Perspectives))
	if result.Blocking {
		b.WriteString(" Blocked: a blocking member voted block or escalate.")
	}
	b.WriteString("\n\n")

	names := map[string]string{}
	for _, p := range result.Perspectives {
		names[p.Expert] = perspectiveName(p)
	}

	switch {
	case len(result.Disagreements) > 0:
		b.WriteString("### Where they disagree\n")
		for _, d := range result.Disagreements {
			fmt.Fprintf(&b, "- **%s**\n", d.Topic)
			for _, side := range d.Sides {
				var who []string
				for _, id := range side.Experts {
					who = append(who, nameOr(names, id))
				}
				fmt.Fprintf(&b, "  - %s: %s\n", strings.Join(who, ", "), side.Position)
			}
		}
		b.WriteByte('\n')
	case result.Tension != "":
		b.WriteString("### Where they disagree\n")
		for _, l := range strings.Split(result.Tension, "\n") {
			fmt.Fprintf(&b, "- %s\n", l)
		}
		b.WriteByte('\n')
	}

	if len(result.Decisions) > 0 {
		b.WriteString("### What you need to decide\n")
		for _, d := range result.Decisions {
			fmt.Fprintf(&b, "- %s\n", d)
		}
		b.WriteByte('\n')
	}

	if len(result.Agreements) > 0 {
		b.WriteString("### Nobody disputed\n")
		for _, a := range result.Agreements {
			fmt.Fprintf(&b, "- %s\n", a)
		}
		b.WriteByte('\n')
	}

	if len(result.Perspectives) > 0 {
		b.WriteString("### Individual Perspectives\n\n")
		b.WriteString("| Expert | Verdict | Key Concern |\n")
		b.WriteString("|---|---|---|\n")
		for _, p := range result.Perspectives {
			concern := "—"
			if len(p.Notes) > 0 {
				concern = truncateString(p.Notes[0], 80)
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", perspectiveName(p), p.Verdict, concern)
		}
		b.WriteByte('\n')
	}

	if len(result.Errors) > 0 {
		b.WriteString("### Errors\n")
		for _, e := range result.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		b.WriteByte('\n')
	}

	if result.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", result.Summary)
	}

	packLabel := "default"
	if packName != "" {
		packLabel = packName
	}
	fmt.Fprintf(&b, "<sub>Reviewed by [Council](https://github.com/luuuc/council) · pack: %s · %d experts</sub>", packLabel, expertCount)

	return b.String()
}

func extractInlineComments(result *SynthesizedResult, dp *DiffPosition) ([]GitHubReviewComment, []GitHubCheckAnnotation) {
	var comments []GitHubReviewComment
	var annotations []GitHubCheckAnnotation

	for _, p := range result.Perspectives {
		for _, note := range p.Notes {
			file, line, text := parseNoteFileRef(note)
			if file == "" {
				continue
			}

			body := fmt.Sprintf("**%s** (%s):\n%s", perspectiveName(p), p.Verdict, text)

			pos, ok := dp.Position(file, line)
			if ok {
				comments = append(comments, GitHubReviewComment{
					Path:     file,
					Position: pos,
					Body:     body,
				})
			} else {
				// Can't map to diff position — add as fallback (position=0)
				comments = append(comments, GitHubReviewComment{
					Path: file,
					Body: fmt.Sprintf("**%s** (%s) on `%s:%d`:\n%s", p.Expert, p.Verdict, file, line, text),
				})
			}

			var level string
			switch p.Verdict {
			case VerdictBlock, VerdictEscalate:
				level = "failure"
			case VerdictComment:
				level = "warning"
			default:
				level = "notice"
			}

			annotations = append(annotations, GitHubCheckAnnotation{
				Path:            file,
				StartLine:       line,
				EndLine:         line,
				AnnotationLevel: level,
				Message:         text,
				Title:           fmt.Sprintf("%s (%s)", p.Expert, p.Verdict),
			})
		}
	}

	return comments, annotations
}

// parseNoteFileRef extracts a file:line reference from the beginning of a note.
// Expected formats: "path/to/file.go:42: message" or "path/to/file.go:42 message"
// Returns ("", 0, note) if no file reference is found.
func parseNoteFileRef(note string) (string, int, string) {
	// Look for file:line: or file:line pattern
	colonIdx := strings.Index(note, ":")
	if colonIdx < 1 {
		return "", 0, note
	}

	file := note[:colonIdx]
	// Validate it looks like a file path (contains a dot or slash)
	if !strings.Contains(file, ".") && !strings.Contains(file, "/") {
		return "", 0, note
	}

	rest := note[colonIdx+1:]
	// Extract line number
	lineEnd := 0
	for lineEnd < len(rest) && rest[lineEnd] >= '0' && rest[lineEnd] <= '9' {
		lineEnd++
	}
	if lineEnd == 0 {
		return "", 0, note
	}

	line := 0
	for _, c := range rest[:lineEnd] {
		line = line*10 + int(c-'0')
	}

	text := strings.TrimLeft(rest[lineEnd:], ": ")
	if text == "" {
		text = note
	}

	return file, line, text
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// perspectiveName returns the expert's display name, falling back to the ID.
func perspectiveName(p ExpertVerdict) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Expert
}
