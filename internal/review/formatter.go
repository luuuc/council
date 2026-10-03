package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatHuman renders a SynthesizedResult as human-readable text.
func FormatHuman(result *SynthesizedResult, packName string, expertCount int) string {
	var b strings.Builder
	b.WriteString(FormatHeader(packName, expertCount))
	for i := range result.Perspectives {
		b.WriteString(FormatPerspective(result.Perspectives[:i+1]))
	}
	for i, p := range result.Perspectives {
		if len(p.FinalWord) > 0 || p.ChangedFrom != "" {
			b.WriteString(FormatFinalWord(p.Expert, result.Perspectives[i:]))
		}
	}
	b.WriteString(FormatOutcome(result))
	return b.String()
}

// FormatHeader renders the title block of a review.
func FormatHeader(packName string, expertCount int) string {
	var b strings.Builder
	if packName != "" {
		fmt.Fprintf(&b, "Council Review — pack: %s (%d experts)\n", packName, expertCount)
	} else {
		fmt.Fprintf(&b, "Council Review — %d experts\n", expertCount)
	}
	b.WriteString(strings.Repeat("═", 50) + "\n\n")
	return b.String()
}

// FormatPerspective renders a member's review: the last verdict in
// verdicts. Earlier verdicts are used for the names of the members it
// replies to. The verdict shown is the one from their review, before any
// change in their final word.
func FormatPerspective(verdicts []ExpertVerdict) string {
	if len(verdicts) == 0 {
		return ""
	}
	p := verdicts[len(verdicts)-1]

	var b strings.Builder
	verdict := p.Verdict
	if p.ChangedFrom != "" {
		verdict = p.ChangedFrom
	}
	writeTitle(&b, displayNameOf(p), string(verdict))

	if p.Error != "" {
		fmt.Fprintf(&b, "  (error: %s)\n", p.Error)
	}
	for _, note := range p.Notes {
		fmt.Fprintf(&b, "  - %s\n", wrapNote(note, 46))
	}
	writeReplies(&b, p.Replies, verdicts)

	b.WriteByte('\n')
	return b.String()
}

// FormatFinalWord renders the final word of the member with the given ID.
// verdicts must include that member and everyone they answer.
func FormatFinalWord(id string, verdicts []ExpertVerdict) string {
	var p *ExpertVerdict
	for i := range verdicts {
		if verdicts[i].Expert == id {
			p = &verdicts[i]
		}
	}
	if p == nil || (len(p.FinalWord) == 0 && p.ChangedFrom == "") {
		return ""
	}

	var b strings.Builder
	status := "keeps " + string(p.Verdict)
	if p.ChangedFrom != "" {
		status = fmt.Sprintf("%s → %s", p.ChangedFrom, p.Verdict)
	}
	writeTitle(&b, displayNameOf(*p)+" — final word", status)
	if p.ChangedFrom != "" && p.ChangeReason != "" {
		fmt.Fprintf(&b, "  Changed verdict: %s\n", wrapNote(p.ChangeReason, 46))
	}
	writeReplies(&b, p.FinalWord, verdicts)

	b.WriteByte('\n')
	return b.String()
}

// FormatOutcome renders the end of a review: where the members disagree,
// what the author has to decide, what nobody disputed, and the vote count.
// Council doesn't recommend an outcome.
func FormatOutcome(result *SynthesizedResult) string {
	var b strings.Builder
	rule := strings.Repeat("─", 50) + "\n"

	if len(result.Errors) > 0 {
		b.WriteString(rule)
		for _, e := range result.Errors {
			fmt.Fprintf(&b, "Error: %s\n", e)
		}
		b.WriteByte('\n')
	}

	names := map[string]string{}
	for _, p := range result.Perspectives {
		names[p.Expert] = displayNameOf(p)
	}

	switch {
	case len(result.Disagreements) > 0:
		b.WriteString(rule)
		b.WriteString("Where they disagree\n")
		for i, d := range result.Disagreements {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, wrapIndent(d.Topic, 46, "     "))
			for _, s := range d.Sides {
				var who []string
				for _, id := range s.Experts {
					who = append(who, nameOr(names, id))
				}
				fmt.Fprintf(&b, "     - %s\n", wrapIndent(strings.Join(who, ", ")+": "+s.Position, 44, "       "))
			}
		}
		b.WriteByte('\n')
	case result.Tension != "":
		b.WriteString(rule)
		b.WriteString("Where they disagree\n")
		for _, l := range strings.Split(result.Tension, "\n") {
			fmt.Fprintf(&b, "  - %s\n", wrapNote(l, 46))
		}
		b.WriteByte('\n')
	}

	if len(result.Decisions) > 0 {
		b.WriteString("What you need to decide\n")
		for _, d := range result.Decisions {
			fmt.Fprintf(&b, "  - %s\n", wrapNote(d, 46))
		}
		b.WriteByte('\n')
	}

	if len(result.Agreements) > 0 {
		b.WriteString("Nobody disputed\n")
		for _, a := range result.Agreements {
			fmt.Fprintf(&b, "  - %s\n", wrapNote(a, 46))
		}
		b.WriteByte('\n')
	}

	fmt.Fprintf(&b, "Votes: %s\n", voteCount(result.Perspectives))
	if result.Blocking {
		b.WriteString("Blocked: a blocking member voted block or escalate.\n")
	}
	return b.String()
}

// FormatJSON marshals a SynthesizedResult as indented JSON.
func FormatJSON(result *SynthesizedResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// voteCount renders "2 block, 1 comment" in severity order.
func voteCount(perspectives []ExpertVerdict) string {
	counts := map[Verdict]int{}
	for _, p := range perspectives {
		if p.Error == "" {
			counts[p.Verdict]++
		}
	}
	var parts []string
	for _, v := range []Verdict{VerdictEscalate, VerdictBlock, VerdictComment, VerdictPass} {
		if n := counts[v]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, v))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func writeTitle(b *strings.Builder, name, right string) {
	padding := 50 - len(name) - len(right)
	if padding < 2 {
		padding = 2
	}
	fmt.Fprintf(b, "%s%s%s\n", name, strings.Repeat(" ", padding), right)
}

func writeReplies(b *strings.Builder, replies []Reply, verdicts []ExpertVerdict) {
	for _, r := range replies {
		fmt.Fprintf(b, "  → %s %s:\n    %s\n", replyVerb(r.Stance), replyTarget(r.To, verdicts), wrapNote(r.Note, 46))
	}
}

func displayNameOf(p ExpertVerdict) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Expert
}

func nameOr(names map[string]string, id string) string {
	if n, ok := names[id]; ok {
		return n
	}
	return id
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

// wrapNote wraps long notes for terminal display, indenting continuation
// lines under a "  - " bullet.
func wrapNote(note string, width int) string {
	return wrapIndent(note, width, "    ")
}

// wrapIndent wraps text at width, starting continuation lines with indent.
func wrapIndent(note string, width int, indent string) string {
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
	return strings.Join(lines, "\n"+indent)
}
