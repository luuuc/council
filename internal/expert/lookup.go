package expert

import (
	"strings"
)

// SuggestionBank is a map of category to list of experts used for persona lookup.
type SuggestionBank map[string][]Expert

// LookupPersona finds a curated persona by name or ID (case-insensitive).
// Returns nil if not found.
func LookupPersona(bank SuggestionBank, nameOrID string) *Expert {
	normalized := strings.ToLower(strings.TrimSpace(nameOrID))

	// Try legacy alias resolution first
	if newID, ok := LegacyAlias(normalized); ok {
		normalized = newID
	}

	// First pass: exact matches
	for _, experts := range bank {
		for _, e := range experts {
			// Match by ID
			if strings.ToLower(e.ID) == normalized {
				copy := e
				return &copy
			}
			// Match by name (case-insensitive)
			if strings.ToLower(e.Name) == normalized {
				copy := e
				return &copy
			}
			// Match by name converted to ID format (spaces → dashes)
			if strings.ToLower(strings.ReplaceAll(e.Name, " ", "-")) == normalized {
				copy := e
				return &copy
			}
		}
	}

	// Second pass: first-name matching (for inputs like "Luc" → "Luc Perussault-Diallo")
	// Only if input looks like a single word (no spaces, no dashes)
	if !strings.Contains(normalized, " ") && !strings.Contains(normalized, "-") {
		var firstNameMatch *Expert
		matchCount := 0
		for _, experts := range bank {
			for _, e := range experts {
				nameParts := strings.Split(e.Name, " ")
				if len(nameParts) > 0 && strings.ToLower(nameParts[0]) == normalized {
					matchCount++
					if matchCount == 1 {
						copy := e
						firstNameMatch = &copy
					}
				}
			}
		}
		// Only return if exactly one match (avoid ambiguity)
		if matchCount == 1 {
			return firstNameMatch
		}
	}

	return nil
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}
	return d[len(a)][len(b)]
}

// SuggestSimilar finds the closest persona match using edit distance.
// Returns nil if no close match (distance > 3), if exact match exists,
// or if the input is too short to match reliably.
// The second return value is the edit distance of the match.
func SuggestSimilar(bank SuggestionBank, input string) (*Expert, int) {
	// If LookupPersona would find this, don't suggest
	if LookupPersona(bank, input) != nil {
		return nil, 0
	}

	normalized := strings.ToLower(strings.TrimSpace(input))

	// For short inputs (< 4 chars), try prefix matching on first names
	// This handles cases like "Sable" → "Sable Okoro", "Iris" → "Iris Vance"
	if len(normalized) < 4 && len(normalized) >= 2 {
		var prefixMatches []*Expert
		for _, experts := range bank {
			for _, e := range experts {
				nameParts := strings.Split(e.Name, " ")
				if len(nameParts) > 0 {
					firstName := strings.ToLower(nameParts[0])
					if strings.HasPrefix(firstName, normalized) {
						copy := e
						prefixMatches = append(prefixMatches, &copy)
					}
				}
			}
		}
		// Return first match if only one, or nil if ambiguous
		if len(prefixMatches) == 1 {
			return prefixMatches[0], 1 // Distance 1 for prefix match
		}
		// Multiple matches or none - fall through to return nil for short inputs
		return nil, 0
	}

	var bestMatch *Expert
	bestDistance := 4 // Threshold: only consider distance <= 3

	for _, experts := range bank {
		for _, e := range experts {
			// Check distance against name
			if d := levenshtein(normalized, strings.ToLower(e.Name)); d < bestDistance && d > 0 {
				bestDistance = d
				copy := e
				bestMatch = &copy
			}
			// Check distance against ID
			if d := levenshtein(normalized, strings.ToLower(e.ID)); d < bestDistance && d > 0 {
				bestDistance = d
				copy := e
				bestMatch = &copy
			}
		}
	}
	return bestMatch, bestDistance
}
