package expert

import "strings"

// VirtualPrefix names personas modeled on real people (e.g. "Virtual Jane Doe").
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

	e.Kind = KindPerson
	keepCouncilTensions(e)
}

// CustomerPrefix names customer personas so they read as users, not experts.
const CustomerPrefix = "Customer: "

// NormalizeCustomer enforces the naming rule for a customer persona:
// "Customer: {label}", ID "customer-{label}".
func NormalizeCustomer(e *Expert) {
	label := strings.TrimSpace(strings.TrimPrefix(e.Name, CustomerPrefix))
	if label == "" {
		label = "User"
	}
	e.Name = CustomerPrefix + label
	e.ID = "customer-" + ToID(label)
	e.Kind = KindCustomer
	e.Category = "custom"
	e.Tensions = nil // customers react to the work, not to experts' positions
}

// NormalizeRole enforces the naming rule for a role persona: a plain role
// title (never "Virtual"), ID from the title.
func NormalizeRole(e *Expert, requested string) {
	title := strings.TrimSpace(strings.TrimPrefix(e.Name, VirtualPrefix))
	if title == "" {
		title = strings.TrimSpace(requested)
	}
	e.Name = title
	e.ID = ToID(title)
	e.Kind = KindRole
	if e.Category == "" {
		e.Category = "custom"
	}
	keepCouncilTensions(e)
}

// keepCouncilTensions drops tensions with anyone who isn't on the council.
func keepCouncilTensions(e *Expert) {
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
