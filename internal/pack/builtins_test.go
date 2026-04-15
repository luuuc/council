package pack

import "testing"

func TestBuiltins(t *testing.T) {
	packs := Builtins()

	expected := []string{"rails", "go", "writing"}
	for _, name := range expected {
		p, ok := packs[name]
		if !ok {
			t.Errorf("missing built-in pack %q", name)
			continue
		}
		if p.Source != "builtin" {
			t.Errorf("pack %q Source = %q, want %q", name, p.Source, "builtin")
		}
		if len(p.Members) == 0 {
			t.Errorf("pack %q has no members", name)
		}
		if p.Name != name {
			t.Errorf("pack %q Name = %q", name, p.Name)
		}
	}
}

func TestBuiltinsSpecificMembers(t *testing.T) {
	packs := Builtins()

	tests := []struct {
		pack    string
		members []string
	}{
		{"rails", []string{"kent-beck", "dhh", "bruce-schneier", "jason-fried", "matz", "luc-perussault-diallo"}},
		{"go", []string{"rob-pike", "dave-cheney", "kent-beck", "bruce-schneier", "antirez", "luc-perussault-diallo"}},
		{"writing", []string{"luc-perussault-diallo", "jason-fried", "dieter-rams", "william-zinsser"}},
	}

	for _, tt := range tests {
		t.Run(tt.pack, func(t *testing.T) {
			p := packs[tt.pack]
			if len(p.Members) != len(tt.members) {
				t.Fatalf("pack %q has %d members, want %d", tt.pack, len(p.Members), len(tt.members))
			}
			for i, want := range tt.members {
				if p.Members[i].ID != want {
					t.Errorf("Members[%d].ID = %q, want %q", i, p.Members[i].ID, want)
				}
			}
		})
	}
}
