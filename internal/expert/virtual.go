package expert

import "strings"

// VirtualPrefix names personas modeled on real people (e.g. "Virtual Kent Beck").
const VirtualPrefix = "Virtual "

// NormalizeVirtual enforces the naming rule for a persona built from a real
// person's public work: name "Virtual {Name}", ID from the real name, and
// tensions only with current council members. requested is the name the
// user asked for, used when the persona has no name.
func NormalizeVirtual(e *Expert, requested string) {
	base := strings.TrimSpace(strings.TrimPrefix(e.Name, VirtualPrefix))
	if base == "" {
		base = strings.TrimSpace(strings.TrimPrefix(requested, VirtualPrefix))
	}
	e.Name = VirtualPrefix + base
	e.ID = ToID(base)
	if e.Category == "" {
		e.Category = "custom"
	}

	members := map[string]bool{}
	if list, err := List(); err == nil {
		for _, m := range list {
			members[m.ID] = true
		}
	}
	var kept []Tension
	for _, t := range e.Tensions {
		if members[t.Expert] && t.Expert != e.ID {
			kept = append(kept, t)
		}
	}
	e.Tensions = kept
}
