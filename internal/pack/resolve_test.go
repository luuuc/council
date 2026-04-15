package pack

import (
	"testing"

	"github.com/luuuc/council-cli/internal/expert"
)

func TestResolve(t *testing.T) {
	alice := &expert.Expert{ID: "alice", Name: "Alice", Priority: "normal"}
	bob := &expert.Expert{ID: "bob", Name: "Bob", Priority: "normal"}
	charlie := &expert.Expert{ID: "charlie", Name: "Charlie", Priority: "always"}

	available := []*expert.Expert{alice, bob, charlie}

	tests := []struct {
		name         string
		pack         *Pack
		wantIDs      []string
		wantBlocking map[string]bool
		wantWarnings int
	}{
		{
			name: "basic resolution",
			pack: &Pack{
				Name: "test",
				Members: []Member{
					{ID: "alice", Blocking: true},
					{ID: "bob"},
				},
			},
			wantIDs:      []string{"alice", "bob", "charlie"}, // charlie injected via always
			wantBlocking: map[string]bool{"alice": true, "bob": false, "charlie": false},
		},
		{
			name: "unknown member warns",
			pack: &Pack{
				Name:    "test",
				Members: []Member{{ID: "alice"}, {ID: "unknown"}},
			},
			wantIDs:      []string{"alice", "charlie"},
			wantWarnings: 1,
		},
		{
			name: "always expert already in pack keeps blocking flag",
			pack: &Pack{
				Name: "test",
				Members: []Member{
					{ID: "charlie", Blocking: true},
				},
			},
			wantIDs:      []string{"charlie"},
			wantBlocking: map[string]bool{"charlie": true}, // keeps pack's blocking flag
		},
		{
			name: "empty pack gets only always experts",
			pack: &Pack{
				Name:    "empty",
				Members: nil,
			},
			wantIDs: []string{"charlie"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, warnings := Resolve(tt.pack, available)

			if len(warnings) != tt.wantWarnings {
				t.Errorf("warnings count = %d, want %d: %v", len(warnings), tt.wantWarnings, warnings)
			}

			if len(resolved) != len(tt.wantIDs) {
				t.Fatalf("resolved count = %d, want %d", len(resolved), len(tt.wantIDs))
			}

			for i, want := range tt.wantIDs {
				if resolved[i].Expert.ID != want {
					t.Errorf("resolved[%d].ID = %q, want %q", i, resolved[i].Expert.ID, want)
				}
			}

			if tt.wantBlocking != nil {
				for _, rm := range resolved {
					want, ok := tt.wantBlocking[rm.Expert.ID]
					if ok && rm.Blocking != want {
						t.Errorf("%s Blocking = %v, want %v", rm.Expert.ID, rm.Blocking, want)
					}
				}
			}
		})
	}
}

func TestResolveNoPriorityAlways(t *testing.T) {
	experts := []*expert.Expert{
		{ID: "alice", Priority: "normal"},
		{ID: "bob", Priority: "high"},
	}

	p := &Pack{
		Name:    "test",
		Members: []Member{{ID: "alice"}},
	}

	resolved, warnings := Resolve(p, experts)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(resolved) != 1 {
		t.Fatalf("resolved count = %d, want 1 (no always-priority injection)", len(resolved))
	}
	if resolved[0].Expert.ID != "alice" {
		t.Errorf("resolved[0].ID = %q, want alice", resolved[0].Expert.ID)
	}
}
