package cmd

import "testing"

func TestSuggestionsSchema(t *testing.T) {
	bank := loadSuggestionBank()

	if len(bank) == 0 {
		t.Fatal("suggestionBank is empty - suggestions.yaml failed to load")
	}

	seenIDs := make(map[string]string) // id -> category for duplicate detection
	totalExperts := 0

	for category, experts := range bank {
		if len(experts) == 0 {
			t.Errorf("category %q has no experts", category)
			continue
		}

		for i, e := range experts {
			totalExperts++
			prefix := func(msg string) string {
				return category + "[" + string(rune('0'+i)) + "] " + e.Name + ": " + msg
			}

			// Required fields
			if e.ID == "" {
				t.Error(prefix("missing id"))
			}
			if e.Name == "" {
				t.Error(prefix("missing name"))
			}
			if e.Focus == "" {
				t.Error(prefix("missing focus"))
			}
			if e.Philosophy == "" {
				t.Error(prefix("missing philosophy"))
			}
			if len(e.Principles) == 0 {
				t.Error(prefix("missing principles"))
			}
			if len(e.RedFlags) == 0 {
				t.Error(prefix("missing red_flags"))
			}

			// Quality checks
			if len(e.Principles) < 3 {
				t.Errorf(prefix("too few principles (%d, want at least 3)"), len(e.Principles))
			}
			if len(e.RedFlags) < 2 {
				t.Errorf(prefix("too few red_flags (%d, want at least 2)"), len(e.RedFlags))
			}

			// PIR-001: composite persona fields (enforced after PIR-004 gallery replacement)
			if len(e.Influences) > 0 && e.Backstory == "" {
				t.Error(prefix("has influences but missing backstory"))
			}
			if e.Backstory != "" && len(e.Influences) == 0 {
				t.Error(prefix("has backstory but missing influences"))
			}

			// ID format: must be kebab-case (lowercase, hyphens only)
			// Note: IDs can differ from ToID(name) for aliases (tenderlove),
			// disambiguation (jose-valim vs jose-valim-phoenix), or accented names
			for _, c := range e.ID {
				if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
					t.Errorf(prefix("id %q contains invalid character %q"), e.ID, string(c))
					break
				}
			}

			// Duplicate ID check
			if prevCategory, exists := seenIDs[e.ID]; exists {
				t.Errorf(prefix("duplicate id %q (also in %s)"), e.ID, prevCategory)
			}
			seenIDs[e.ID] = category
		}
	}

	t.Logf("Validated %d experts across %d categories", totalExperts, len(bank))
}

func TestSuggestionsTriggers(t *testing.T) {
	// Check that general experts have either Core=true or Triggers defined
	generalExperts := loadSuggestionBank()["general"]
	if len(generalExperts) == 0 {
		t.Skip("no general category found")
	}

	for _, e := range generalExperts {
		if !e.Core && len(e.Triggers) == 0 {
			t.Errorf("general expert %q has neither core=true nor triggers defined", e.Name)
		}
	}
}

func TestSuggestSimilar(t *testing.T) {
	tests := []struct {
		input    string
		wantName string // empty means expect nil
	}{
		// Single character typos (matches against IDs)
		{"Luc Perussault-Diall", "Luc Perussault-Diallo"},
		{"ada-redgrav", "The TDD Advocate"},
		{"Sable Okor", "The Go Purist"},

		// Case insensitive - exact matches should return nil (use LookupPersona)
		{"THE GO PURIST", ""},
		{"the go purist", ""},

		// First-name found by LookupPersona - should return nil
		{"Luc", ""},
		{"luc", ""},

		// Legacy alias resolution - returns nil because LookupPersona resolves them
		{"kent-beck", ""},
		{"rob-pike", ""},

		// Prefix matching for short inputs (2-3 chars) - unique prefix
		{"Lu", "Luc Perussault-Diallo"},

		// No close match
		{"xyz", ""},
		{"completely unknown person", ""},

		// Too far (distance > 3)
		{"abcdefgh", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, _ := SuggestSimilar(tt.input)
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

func TestLookupPersona(t *testing.T) {
	tests := []struct {
		input   string
		wantID  string
		wantNil bool
	}{
		// Exact matches by name
		{"The Go Purist", "sable-okoro", false},
		{"THE GO PURIST", "sable-okoro", false},
		{"  The Go Purist  ", "sable-okoro", false},
		{"The TDD Advocate", "ada-redgrave", false},

		// Exact match by ID
		{"sable-okoro", "sable-okoro", false},
		{"ada-redgrave", "ada-redgrave", false},

		// First-name matching (only Luc has a non-"The" first name)
		{"Luc", "luc-perussault-diallo", false},
		{"luc", "luc-perussault-diallo", false},

		// Legacy alias resolution
		{"kent-beck", "ada-redgrave", false},
		{"rob-pike", "sable-okoro", false},

		// Unknown
		{"Unknown Person", "", true},
		{"Brad Pitt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := LookupPersona(tt.input)
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

func TestFilterPersonasByCategory(t *testing.T) {
	personas := []PersonaJSON{
		{ID: "sable-okoro", Name: "The Go Purist", Category: "go", Focus: "Go systems programming"},
		{ID: "nolan-chambers", Name: "The Pythonista", Category: "python", Focus: "Python idioms"},
		{ID: "ada-redgrave", Name: "The TDD Advocate", Category: "general", Focus: "TDD"},
		{ID: "diego-valdez", Name: "The Rails Monolith", Category: "rails", Focus: "Rails conventions"},
	}

	tests := []struct {
		category string
		wantLen  int
	}{
		{"go", 1},
		{"GO", 1},       // case-insensitive
		{"general", 1},
		{"rails", 1},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			filtered := filterPersonasByCategory(personas, tt.category)
			if len(filtered) != tt.wantLen {
				t.Errorf("filterPersonasByCategory(%q) returned %d personas, want %d", tt.category, len(filtered), tt.wantLen)
			}
		})
	}
}

func TestFilterPersonasBySearch(t *testing.T) {
	personas := []PersonaJSON{
		{ID: "sable-okoro", Name: "The Go Purist", Category: "go", Focus: "Go systems programming"},
		{ID: "ada-redgrave", Name: "The TDD Advocate", Category: "general", Focus: "Test-driven development"},
		{ID: "nadia-kowalski", Name: "The Threat Modeler", Category: "security", Focus: "Application security"},
	}

	tests := []struct {
		search  string
		wantLen int
	}{
		{"Purist", 1},
		{"purist", 1},         // case-insensitive
		{"security", 1},       // matches name and focus (same persona)
		{"test", 1},           // matches focus
		{"xyz", 0},            // no match
		{"Go", 1},             // matches focus
	}

	for _, tt := range tests {
		t.Run(tt.search, func(t *testing.T) {
			filtered := filterPersonasBySearch(personas, tt.search)
			if len(filtered) != tt.wantLen {
				t.Errorf("filterPersonasBySearch(%q) returned %d personas, want %d", tt.search, len(filtered), tt.wantLen)
			}
		})
	}
}
