package pack

import (
	"cmp"
	"slices"
)

// ListAll returns the project's packs, sorted by name.
func ListAll() ([]*Pack, error) {
	packs, err := List()
	if err != nil {
		return nil, err
	}
	slices.SortFunc(packs, func(a, b *Pack) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return packs, nil
}

// Get returns one of the project's packs by name.
func Get(name string) (*Pack, error) {
	return Load(name)
}
