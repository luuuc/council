package review

import (
	"fmt"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

// Synthesize aggregates individual expert verdicts into a single result.
// It resolves conflicts using the decision hierarchy, detects tensions
// from expert metadata, and generates a summary.
func Synthesize(verdicts []ExpertVerdict, experts []*expert.Expert, errors []string) *SynthesizedResult {
	result := &SynthesizedResult{
		Verdict:      VerdictPass,
		Perspectives: verdicts,
		Errors:       errors,
	}

	if len(verdicts) == 0 {
		result.Summary = "No expert verdicts received."
		if len(errors) > 0 {
			result.Summary = fmt.Sprintf("All %d experts failed.", len(errors))
		}
		return result
	}

	// Index experts by ID for lookups
	byID := make(map[string]*expert.Expert, len(experts))
	for _, e := range experts {
		byID[e.ID] = e
	}

	// Determine overall verdict: highest severity wins, weighted by hierarchy
	result.Verdict = ResolveOverallVerdict(verdicts, byID)

	// Determine blocking status
	result.Blocking = ResolveBlocking(verdicts)

	// Find agreements
	result.Agreements = findAgreements(verdicts)

	// Find tensions
	result.Tension = findTension(verdicts, byID)

	// Build summary
	result.Summary = buildSummary(verdicts, errors, result.Verdict, result.Blocking)

	return result
}

// ResolveOverallVerdict determines the aggregate verdict.
// A block from a higher-priority domain outweighs a pass from lower ones.
func ResolveOverallVerdict(verdicts []ExpertVerdict, experts map[string]*expert.Expert) Verdict {
	highest := VerdictPass
	highestDomain := DomainQuality

	for _, v := range verdicts {
		if v.Error != "" {
			continue // skip failed experts
		}

		domain := DomainQuality
		if e, ok := experts[v.Expert]; ok {
			domain = ExpertDomain(e)
		}

		// Higher severity always wins
		if v.Verdict.Severity() > highest.Severity() {
			highest = v.Verdict
			highestDomain = domain
			continue
		}

		// Same severity: higher domain takes precedence (for tie-breaking context)
		if v.Verdict.Severity() == highest.Severity() && domain > highestDomain {
			highestDomain = domain
		}
	}

	return highest
}

// ResolveBlocking checks if any blocking expert issued a block or escalate.
func ResolveBlocking(verdicts []ExpertVerdict) bool {
	for _, v := range verdicts {
		if v.Blocking && (v.Verdict == VerdictBlock || v.Verdict == VerdictEscalate) {
			return true
		}
	}
	return false
}

// findAgreements returns notes that appear across multiple experts with the same verdict.
func findAgreements(verdicts []ExpertVerdict) []string {
	// Group verdicts by outcome (ignoring errors)
	byVerdict := make(map[Verdict][]ExpertVerdict)
	for _, v := range verdicts {
		if v.Error == "" {
			byVerdict[v.Verdict] = append(byVerdict[v.Verdict], v)
		}
	}

	// If all valid experts agree on verdict, that's an agreement
	var agreements []string
	validCount := 0
	for _, vs := range byVerdict {
		validCount += len(vs)
	}

	for verdict, vs := range byVerdict {
		if len(vs) == validCount && validCount > 1 {
			agreements = append(agreements, fmt.Sprintf("All %d experts agree: %s.", validCount, verdict))
		}
	}

	return agreements
}

// findTension reports where experts disagree. Disagreements the experts
// voiced in their replies come first; otherwise it falls back to known
// tension pairs among experts with different verdicts.
func findTension(verdicts []ExpertVerdict, experts map[string]*expert.Expert) string {
	if t := replyDisagreements(verdicts, experts); t != "" {
		return t
	}

	// Find pairs of experts who disagree
	for i := 0; i < len(verdicts); i++ {
		for j := i + 1; j < len(verdicts); j++ {
			vi, vj := verdicts[i], verdicts[j]
			if vi.Error != "" || vj.Error != "" {
				continue
			}
			if vi.Verdict == vj.Verdict {
				continue
			}

			// Check if there's a defined tension between these experts
			if t := lookupTension(vi.Expert, vj.Expert, experts); t != "" {
				return t
			}
		}
	}
	return ""
}

// replyDisagreements lists every "disagree" reply, one per line.
func replyDisagreements(verdicts []ExpertVerdict, experts map[string]*expert.Expert) string {
	var lines []string
	for _, v := range verdicts {
		for _, r := range v.Replies {
			if r.Stance != StanceDisagree {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s disagrees with %s: %s",
				displayName(v.Expert, v.Name, experts), displayName(r.To, "", experts), r.Note))
		}
	}
	return strings.Join(lines, "\n")
}

// displayName prefers the verdict's name, then the expert's name, then the ID.
func displayName(id, name string, experts map[string]*expert.Expert) string {
	if name != "" {
		return name
	}
	if e, ok := experts[id]; ok && e.Name != "" {
		return e.Name
	}
	return id
}

// lookupTension checks both directions for a defined tension between two experts.
func lookupTension(idA, idB string, experts map[string]*expert.Expert) string {
	if e, ok := experts[idA]; ok {
		for _, t := range e.Tensions {
			if t.Expert == idB {
				return formatTension(e.Name, t, experts)
			}
		}
	}
	if e, ok := experts[idB]; ok {
		for _, t := range e.Tensions {
			if t.Expert == idA {
				return formatTension(e.Name, t, experts)
			}
		}
	}
	return ""
}

// formatTension renders a tension as a readable string.
func formatTension(ownerName string, t expert.Tension, experts map[string]*expert.Expert) string {
	otherName := t.Expert
	if e, ok := experts[t.Expert]; ok {
		otherName = e.Name
	}
	return fmt.Sprintf("%s vs %s on %s: %s (counterpoint: %s)",
		ownerName, otherName, t.Topic, t.Position, t.Counterpoint)
}

// buildSummary counts the votes. It doesn't recommend an outcome: the
// author decides from the disagreements and decisions.
func buildSummary(verdicts []ExpertVerdict, errors []string, _ Verdict, blocking bool) string {
	summary := fmt.Sprintf("%d experts reviewed: %s", len(verdicts)+len(errors), voteCount(verdicts))
	if len(errors) > 0 {
		summary += fmt.Sprintf(", %d failed", len(errors))
	}
	summary += "."
	if blocking {
		summary += " Blocked: a blocking member voted block or escalate."
	}
	return summary
}
