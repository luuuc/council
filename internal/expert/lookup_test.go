package expert

import "testing"

// testBank creates a small suggestion bank for testing
func testBank() SuggestionBank {
	return SuggestionBank{
		"general": {
			{ID: "sable-okoro", Name: "Sable Okoro", Focus: "Go"},
			{ID: "ada-redgrave", Name: "Ada Redgrave", Focus: "Testing"},
			{ID: "elara-nygaard", Name: "Elara Nygaard", Focus: "Design"},
			{ID: "iris-vance", Name: "Iris Vance", Focus: "Deep Work"},
		},
		"custom": {
			{ID: "luc-perussault-diallo", Name: "Luc Perussault-Diallo", Focus: "Simplicity"},
			{ID: "renzo-cardenas", Name: "Renzo Cardenas", Focus: "SaaS"},
			{ID: "sable-variant", Name: "Sable Tanaka", Focus: "UX"},
		},
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"ab", "abc", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"rob pike", "rob pik", 1},
	}

	for _, tt := range tests {
		got := levenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestLookupPersona(t *testing.T) {
	bank := testBank()

	tests := []struct {
		input   string
		wantID  string
		wantNil bool
	}{
		// Exact matches
		{"Sable Okoro", "sable-okoro", false},
		{"sable-okoro", "sable-okoro", false},
		{"SABLE OKORO", "sable-okoro", false},
		{"  Sable Okoro  ", "sable-okoro", false},
		{"Ada Redgrave", "ada-redgrave", false},

		// First-name matching (unique first names)
		{"Luc", "luc-perussault-diallo", false},
		{"luc", "luc-perussault-diallo", false},
		{"Elara", "elara-nygaard", false},

		// First-name matching should NOT work for ambiguous names
		// "Sable" matches both "Sable Okoro" and "Sable Tanaka"
		{"Sable", "", true},

		// Unknown
		{"Unknown Person", "", true},
		{"Brad Pitt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := LookupPersona(bank, tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("LookupPersona(%q) = %v, want nil", tt.input, result)
				}
			} else {
				if result == nil {
					t.Errorf("LookupPersona(%q) = nil, want ID %q", tt.input, tt.wantID)
				} else if result.ID != tt.wantID {
					t.Errorf("LookupPersona(%q).ID = %q, want %q", tt.input, result.ID, tt.wantID)
				}
			}
		})
	}
}

func TestSuggestSimilar(t *testing.T) {
	bank := testBank()

	tests := []struct {
		input    string
		wantName string // empty means expect nil
	}{
		// Single character typos
		{"ada-redgrav", "Ada Redgrave"},
		{"Sable Okor", "Sable Okoro"},

		// Case insensitive - exact matches should return nil (use LookupPersona)
		{"SABLE OKORO", ""},
		{"sable okoro", ""},

		// First-name found by LookupPersona - should return nil
		{"Luc", ""},
		{"luc", ""},
		{"Elara", ""},
		{"Iris", ""},

		// Prefix matching for short inputs (2-3 chars) - unique prefix
		{"El", "Elara Nygaard"},

		// No close match
		{"xyz", ""},
		{"completely unknown person", ""},

		// Too far (distance > 3)
		{"abcdefgh", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, _ := SuggestSimilar(bank, tt.input)
			if tt.wantName == "" {
				if got != nil {
					t.Errorf("SuggestSimilar(%q) = %q, want nil", tt.input, got.Name)
				}
			} else {
				if got == nil {
					t.Errorf("SuggestSimilar(%q) = nil, want %q", tt.input, tt.wantName)
				} else if got.Name != tt.wantName {
					t.Errorf("SuggestSimilar(%q) = %q, want %q", tt.input, got.Name, tt.wantName)
				}
			}
		})
	}
}

func TestSuggestSimilar_DistanceBoundaries(t *testing.T) {
	bank := testBank()

	tests := []struct {
		input            string
		wantDistance     int
		wantNonNilResult bool
	}{
		// Distance 1 - high confidence
		{"Sable Okor", 1, true},
		{"ada-redgrav", 1, true},

		// Distance 2 - still prompts
		{"Sable Oko", 2, true},

		// Distance 3 - still matches
		{"Sable Ok", 3, true},

		// Exact match - returns nil (use LookupPersona instead)
		{"Sable Okoro", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, distance := SuggestSimilar(bank, tt.input)
			if tt.wantNonNilResult {
				if got == nil {
					t.Errorf("SuggestSimilar(%q) = nil, want non-nil result", tt.input)
				} else if distance != tt.wantDistance {
					t.Errorf("SuggestSimilar(%q) distance = %d, want %d", tt.input, distance, tt.wantDistance)
				}
			} else {
				if got != nil {
					t.Errorf("SuggestSimilar(%q) = %v, want nil", tt.input, got)
				}
			}
		})
	}
}
