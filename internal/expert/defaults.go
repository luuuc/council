package expert

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

// Council ships no people, with one exception: the author's own persona,
// included with his consent as a ready-to-use example of the format. It is
// added to every new council and can be removed like any other member.
//
//go:embed defaults/*.md
var defaultFiles embed.FS

// DefaultMembers returns the members every new council starts with.
func DefaultMembers() ([]*Expert, error) {
	names, err := fs.Glob(defaultFiles, "defaults/*.md")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)

	var members []*Expert
	for _, name := range names {
		data, err := defaultFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}
		e, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("default member %s: %w", name, err)
		}
		members = append(members, e)
	}
	return members, nil
}
