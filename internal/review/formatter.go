package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatHuman renders a SynthesizedResult as human-readable text.
func FormatHuman(result *SynthesizedResult, packName string, expertCount int) string {
	var b strings.Builder

	// Header
	if packName != "" {
		fmt.Fprintf(&b, "Council Review — pack: %s (%d experts)\n", packName, expertCount)
	} else {
		fmt.Fprintf(&b, "Council Review — %d experts\n", expertCount)
	}
	b.WriteString(strings.Repeat("═", 50) + "\n\n")

	// Perspectives
	for _, p := range result.Perspectives {
		name := p.Expert
		if p.Name != "" {
			name = p.Name
		}
		verdict := string(p.Verdict)

		// Right-align verdict
		padding := 50 - len(name) - len(verdict)
		if padding < 2 {
			padding = 2
		}
		fmt.Fprintf(&b, "%s%s%s\n", name, strings.Repeat(" ", padding), verdict)

		if p.Error != "" {
			fmt.Fprintf(&b, "  (error: %s)\n", p.Error)
		}

		for _, note := range p.Notes {
			fmt.Fprintf(&b, "  - %s\n", wrapNote(note, 46))
		}

		for _, r := range p.Replies {
			fmt.Fprintf(&b, "  → %s %s:\n    %s\n", replyVerb(r.Stance), replyTarget(r.To, result.Perspectives), wrapNote(r.Note, 46))
		}

		b.WriteByte('\n')
	}

	// Errors
	if len(result.Errors) > 0 {
		b.WriteString(strings.Repeat("─", 50) + "\n")
		for _, e := range result.Errors {
			fmt.Fprintf(&b, "Error: %s\n", e)
		}
		b.WriteByte('\n')
	}

	// Tension
	if result.Tension != "" {
		b.WriteString(strings.Repeat("─", 50) + "\n")
		lines := strings.Split(result.Tension, "\n")
		if len(lines) == 1 {
			fmt.Fprintf(&b, "Tension: %s\n\n", result.Tension)
		} else {
			b.WriteString("Tension:\n")
			for _, l := range lines {
				fmt.Fprintf(&b, "  - %s\n", l)
			}
			b.WriteByte('\n')
		}
	}

	// Agreements
	if len(result.Agreements) > 0 {
		for _, a := range result.Agreements {
			fmt.Fprintf(&b, "Agreement: %s\n", a)
		}
		b.WriteByte('\n')
	}

	// Verdict line
	verdictLabel := verdictDisplayLabel(result.Verdict, result.Blocking)
	fmt.Fprintf(&b, "Verdict: %s\n", verdictLabel)

	return b.String()
}

// replyVerb renders a stance as the verb shown in human output.
func replyVerb(s Stance) string {
	switch s {
	case StanceAgree:
		return "agrees with"
	case StanceDisagree:
		return "disagrees with"
	default:
		return "adds to"
	}
}

// replyTarget returns the display name of the expert a reply is addressed to.
func replyTarget(id string, perspectives []ExpertVerdict) string {
	for _, p := range perspectives {
		if p.Expert == id && p.Name != "" {
			return p.Name
		}
	}
	return id
}

// FormatJSON marshals a SynthesizedResult as indented JSON.
func FormatJSON(result *SynthesizedResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// verdictDisplayLabel returns a human-friendly label for the overall verdict.
func verdictDisplayLabel(v Verdict, blocking bool) string {
	if blocking {
		return "blocked"
	}
	switch v {
	case VerdictPass:
		return "ship it"
	case VerdictComment:
		return "ship with comments"
	case VerdictBlock:
		return "fix before shipping"
	case VerdictEscalate:
		return "needs escalation"
	default:
		return string(v)
	}
}

// wrapNote wraps a note at the given width for indented display.
func wrapNote(note string, width int) string {
	if len(note) <= width {
		return note
	}

	var lines []string
	for len(note) > 0 {
		if len(note) <= width {
			lines = append(lines, note)
			break
		}
		// Find last space within width
		cut := strings.LastIndex(note[:width], " ")
		if cut <= 0 {
			cut = width // no space found, hard cut
		}
		lines = append(lines, note[:cut])
		note = strings.TrimSpace(note[cut:])
	}
	return strings.Join(lines, "\n    ")
}
