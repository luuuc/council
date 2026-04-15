package pack

import (
	"cmp"
	"slices"
)

// ListAll returns all packs: built-in merged with custom, sorted by name.
// Custom packs override built-in packs with the same name.
func ListAll() ([]*Pack, error) {
	builtins := Builtins()

	custom, err := List()
	if err != nil {
		return nil, err
	}

	// Start with builtins
	merged := make(map[string]*Pack, len(builtins))
	for name, p := range builtins {
		merged[name] = p
	}

	// Custom overrides builtin
	for _, p := range custom {
		merged[p.Name] = p
	}

	result := make([]*Pack, 0, len(merged))
	for _, p := range merged {
		result = append(result, p)
	}

	// Sort for deterministic output
	slices.SortFunc(result, func(a, b *Pack) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return result, nil
}

// Get returns a pack by name, checking custom packs first, then builtins.
func Get(name string) (*Pack, error) {
	// Try custom first
	p, err := Load(name)
	if err == nil {
		return p, nil
	}

	// Try builtin
	if bp, ok := Builtins()[name]; ok {
		return bp, nil
	}

	return nil, err // return original Load error
}
