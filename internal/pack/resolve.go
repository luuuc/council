package pack

import "github.com/luuuc/council-cli/internal/expert"

// ResolvedMember pairs an expert with their blocking status in a pack.
type ResolvedMember struct {
	Expert   *expert.Expert
	Blocking bool
}

// Resolve matches pack members to available experts and enforces priority: always.
// Returns resolved members and warnings for any member IDs not found in available experts.
func Resolve(p *Pack, available []*expert.Expert) ([]ResolvedMember, []string) {
	// Index available experts by ID
	byID := make(map[string]*expert.Expert, len(available))
	for _, e := range available {
		byID[e.ID] = e
	}

	// Track which expert IDs are already included
	included := make(map[string]bool)

	var resolved []ResolvedMember
	var warnings []string

	// Match pack members to available experts
	for _, m := range p.Members {
		e, ok := byID[m.ID]
		if !ok {
			warnings = append(warnings, "expert '"+m.ID+"' not found")
			continue
		}
		resolved = append(resolved, ResolvedMember{Expert: e, Blocking: m.Blocking})
		included[m.ID] = true
	}

	// Inject priority: always experts not already in the pack
	for _, e := range available {
		if e.Priority == "always" && !included[e.ID] {
			resolved = append(resolved, ResolvedMember{Expert: e, Blocking: false})
			included[e.ID] = true
		}
	}

	return resolved, warnings
}
