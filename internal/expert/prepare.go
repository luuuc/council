package expert

import (
	"fmt"
	"regexp"
	"strings"
)

// frontmatterStart matches a line that is exactly "---".
var frontmatterStart = regexp.MustCompile(`(?m)^---[ \t]*$`)

// Disclaimer is stamped on every persona of a real person.
const disclaimerFormat = "Modeled on public material. Not affiliated with or endorsed by %s."

// ParseLoose reads a persona written by an AI: markdown with YAML
// frontmatter, or bare YAML, with or without the "---" markers and with
// any text before the first "---".
func ParseLoose(text string) (*Expert, error) {
	text = strings.TrimSpace(text)
	if loc := frontmatterStart.FindStringIndex(text); loc != nil && loc[0] > 0 {
		text = text[loc[0]:] // drop any preamble before the frontmatter
	}
	if !strings.HasPrefix(text, "---") {
		text = "---\n" + text
	}
	if !strings.Contains(text[3:], "\n---") {
		text += "\n---\n"
	}
	return Parse([]byte(text))
}

// Prepare checks a new persona and applies Council's rules for its kind,
// so it's ready to save:
//
//   - person: needs public sources; named "Virtual {Name}"; disclaimer stamped
//   - role: a plain role title
//   - customer: needs evidence (sources); named "Customer: {label}"; no tensions
//
// Errors are written for the AI that produced the persona, so it can fix
// the file and try again. Notes say what Council dropped (tensions with
// people who aren't on the council, or any tension on a customer).
func Prepare(e *Expert) (notes []string, err error) {
	e.Name = strings.TrimSpace(e.Name)
	e.Focus = strings.TrimSpace(e.Focus)
	if e.Name == "" || e.Focus == "" {
		return nil, fmt.Errorf("a persona needs at least name and focus")
	}
	before := e.Tensions
	defer func() {
		if err == nil {
			notes = droppedTensions(e, before)
		}
	}()

	switch e.Kind {
	case "", KindPerson:
		if len(e.Sources) == 0 {
			return nil, fmt.Errorf("%s is a person: list the public sources the persona is drawn from (talks, books, essays, interviews, decisions) under sources", e.Name)
		}
		if len(e.Principles)+len(e.Inferred) == 0 {
			return nil, fmt.Errorf("%s needs principles (documented positions) or inferred (positions read from their work)", e.Name)
		}
		NormalizeVirtual(e, e.Name)
		e.Disclaimer = fmt.Sprintf(disclaimerFormat, strings.TrimPrefix(e.Name, VirtualPrefix))

	case KindRole:
		if len(e.Principles) == 0 {
			return nil, fmt.Errorf("the %s role needs principles: the positions people in this role hold", e.Name)
		}
		NormalizeRole(e, e.Name)

	case KindCustomer:
		if len(e.Sources) == 0 {
			return nil, fmt.Errorf("%s is a customer: list the evidence the persona is drawn from (docs, support threads, analytics, interviews, or what the user told you) under sources", e.Name)
		}
		if len(e.Principles) == 0 {
			return nil, fmt.Errorf("%s needs principles: what this customer is trying to get done", e.Name)
		}
		NormalizeCustomer(e)

	default:
		return nil, fmt.Errorf("unknown kind %q: use person, role, or customer", e.Kind)
	}

	e.Body = "" // regenerated from the fields on save
	return nil, nil
}

// droppedTensions explains the tensions Prepare removed.
func droppedTensions(e *Expert, before []Tension) []string {
	if len(before) == len(e.Tensions) {
		return nil
	}
	if e.Kind == KindCustomer {
		return []string{"dropped the tensions: customers react to the work, not to other members; put what they object to under red_flags"}
	}
	kept := map[string]bool{}
	for _, t := range e.Tensions {
		kept[t.Expert] = true
	}
	var notes []string
	for _, t := range before {
		if !kept[t.Expert] {
			notes = append(notes, fmt.Sprintf("dropped the tension with %q: not a current member (see 'council list'). To keep it, add %s again after them (council remove %s, then council add)", t.Expert, e.Name, e.ID))
		}
	}
	return notes
}
