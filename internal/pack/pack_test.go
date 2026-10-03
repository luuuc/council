package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Pack
		wantErr bool
	}{
		{
			name: "full pack",
			input: `name: rails
description: Rails review council
members:
  - id: dhh
    blocking: true
  - id: kent-beck
`,
			want: Pack{
				Name:        "rails",
				Description: "Rails review council",
				Members: []Member{
					{ID: "dhh", Blocking: true},
					{ID: "kent-beck", Blocking: false},
				},
			},
		},
		{
			name: "minimal pack",
			input: `name: minimal
members:
  - id: alice
`,
			want: Pack{
				Name:    "minimal",
				Members: []Member{{ID: "alice"}},
			},
		},
		{
			name:  "empty members",
			input: `name: empty`,
			want: Pack{
				Name: "empty",
			},
		},
		{
			name:    "invalid yaml",
			input:   ":\ninvalid:\n  - [broken",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
			if len(got.Members) != len(tt.want.Members) {
				t.Fatalf("Members count = %d, want %d", len(got.Members), len(tt.want.Members))
			}
			for i, m := range got.Members {
				if m.ID != tt.want.Members[i].ID {
					t.Errorf("Members[%d].ID = %q, want %q", i, m.ID, tt.want.Members[i].ID)
				}
				if m.Blocking != tt.want.Members[i].Blocking {
					t.Errorf("Members[%d].Blocking = %v, want %v", i, m.Blocking, tt.want.Members[i].Blocking)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		pack    Pack
		wantErr bool
	}{
		{
			name: "valid",
			pack: Pack{Name: "rails", Members: []Member{{ID: "dhh"}}},
		},
		{
			name:    "empty name",
			pack:    Pack{},
			wantErr: true,
		},
		{
			name:    "name with spaces",
			pack:    Pack{Name: "my pack"},
			wantErr: true,
		},
		{
			name:    "name with slash",
			pack:    Pack{Name: "my/pack"},
			wantErr: true,
		},
		{
			name: "no members is valid",
			pack: Pack{Name: "empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pack.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHasMember(t *testing.T) {
	p := Pack{
		Name:    "test",
		Members: []Member{{ID: "alice"}, {ID: "bob"}},
	}

	if !p.HasMember("alice") {
		t.Error("HasMember(alice) = false, want true")
	}
	if p.HasMember("charlie") {
		t.Error("HasMember(charlie) = true, want false")
	}
}

func TestAddMember(t *testing.T) {
	p := Pack{Name: "test", Members: []Member{{ID: "alice"}}}

	if err := p.AddMember("bob", true); err != nil {
		t.Fatalf("AddMember(bob) error = %v", err)
	}
	if len(p.Members) != 2 {
		t.Fatalf("Members count = %d, want 2", len(p.Members))
	}
	if p.Members[1].ID != "bob" || !p.Members[1].Blocking {
		t.Errorf("Members[1] = %+v, want {ID:bob Blocking:true}", p.Members[1])
	}

	// Duplicate
	if err := p.AddMember("alice", false); err == nil {
		t.Error("AddMember(alice) should fail for duplicate")
	}
}

func TestRemoveMember(t *testing.T) {
	p := Pack{Name: "test", Members: []Member{{ID: "alice"}, {ID: "bob"}}}

	if err := p.RemoveMember("alice"); err != nil {
		t.Fatalf("RemoveMember(alice) error = %v", err)
	}
	if len(p.Members) != 1 || p.Members[0].ID != "bob" {
		t.Errorf("Members after remove = %+v, want [{ID:bob}]", p.Members)
	}

	// Not found
	if err := p.RemoveMember("charlie"); err == nil {
		t.Error("RemoveMember(charlie) should fail for missing member")
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Use temp dir as .council/packs
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	// Create .council/packs directory
	_ = os.MkdirAll(filepath.Join(tmp, ".council", "packs"), 0755)

	p := &Pack{
		Name:        "test-pack",
		Description: "A test pack",
		Members: []Member{
			{ID: "alice", Blocking: true},
			{ID: "bob", Blocking: false},
		},
	}

	if err := Save(p); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load("test-pack")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Name != p.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, p.Name)
	}
	if loaded.Description != p.Description {
		t.Errorf("Description = %q, want %q", loaded.Description, p.Description)
	}
	if len(loaded.Members) != len(p.Members) {
		t.Fatalf("Members count = %d, want %d", len(loaded.Members), len(p.Members))
	}
	for i, m := range loaded.Members {
		if m.ID != p.Members[i].ID || m.Blocking != p.Members[i].Blocking {
			t.Errorf("Members[%d] = %+v, want %+v", i, m, p.Members[i])
		}
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	packsDir := filepath.Join(tmp, ".council", "packs")
	_ = os.MkdirAll(packsDir, 0755)

	// Write two pack files
	for _, name := range []string{"alpha", "beta"} {
		p := &Pack{Name: name, Members: []Member{{ID: "expert-1"}}}
		if err := Save(p); err != nil {
			t.Fatalf("Save(%s) error = %v", name, err)
		}
	}

	packs, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(packs) != 2 {
		t.Fatalf("List() returned %d packs, want 2", len(packs))
	}
}

func TestLoadNotFound(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	_, err := Load("nonexistent")
	if err == nil {
		t.Error("Load(nonexistent) should fail")
	}
}

func TestDelete(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	_ = os.MkdirAll(filepath.Join(tmp, ".council", "packs"), 0755)

	p := &Pack{Name: "doomed", Members: []Member{{ID: "alice"}}}
	if err := Save(p); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := Delete("doomed"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := Load("doomed"); err == nil {
		t.Error("Load() should fail after Delete()")
	}

	// Delete non-existent
	if err := Delete("nonexistent"); err == nil {
		t.Error("Delete(nonexistent) should fail")
	}
}

func TestListWithWarnings(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	packsDir := filepath.Join(tmp, ".council", "packs")
	_ = os.MkdirAll(packsDir, 0755)

	// Write a valid pack
	valid := &Pack{Name: "good", Members: []Member{{ID: "alice"}}}
	if err := Save(valid); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Write a broken YAML file
	_ = os.WriteFile(filepath.Join(packsDir, "broken.yaml"), []byte(":\n  - [invalid"), 0644)

	result, err := ListWithWarnings()
	if err != nil {
		t.Fatalf("ListWithWarnings() error = %v", err)
	}

	if len(result.Packs) != 1 {
		t.Errorf("Packs count = %d, want 1", len(result.Packs))
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Warnings count = %d, want 1: %v", len(result.Warnings), result.Warnings)
	}
}
