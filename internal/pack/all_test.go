package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListAllMerge(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	packsDir := filepath.Join(tmp, ".council", "packs")
	_ = os.MkdirAll(packsDir, 0755)

	// Create a custom pack that shadows the builtin "go" pack
	custom := &Pack{
		Name:        "go",
		Description: "My custom Go council",
		Members:     []Member{{ID: "custom-expert"}},
	}
	if err := Save(custom); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	all, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	// Find the "go" pack in results
	var goPack *Pack
	for _, p := range all {
		if p.Name == "go" {
			goPack = p
			break
		}
	}

	if goPack == nil {
		t.Fatal("ListAll() missing 'go' pack")
	}

	// Custom should override builtin
	if goPack.Description != "My custom Go council" {
		t.Errorf("go pack Description = %q, want custom override", goPack.Description)
	}
	if len(goPack.Members) != 1 || goPack.Members[0].ID != "custom-expert" {
		t.Errorf("go pack Members = %+v, want custom members", goPack.Members)
	}
}

func TestListAllSorted(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	// No custom packs — just builtins
	all, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	// Verify sorted by name
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Errorf("ListAll() not sorted: %q before %q", all[i-1].Name, all[i].Name)
		}
	}
}
