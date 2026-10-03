package cmd

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

//go:embed prompts/research.txt
var researchPrompt string

// errUnknownPerson means the AI doesn't know enough public work by the
// person to build a faithful persona.
var errUnknownPerson = errors.New("not enough public work to build a persona")

// virtualPrefix names personas modeled on real people.
const virtualPrefix = "Virtual "

// researchPerson builds a "Virtual {name}" persona from the person's public
// work using the headless AI CLI. Current council members are offered as
// tension partners so the new member arrives with disagreements.
func researchPerson(name string) (*expert.Expert, error) {
	raw, err := aiPrompt(fmt.Sprintf(researchPrompt, name, councilMemberList()))
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "UNKNOWN") {
		return nil, errUnknownPerson
	}

	exp, err := parseGeneratedExpert(raw)
	if err != nil {
		return nil, err
	}
	normalizeVirtualPersona(exp, name)
	return exp, nil
}

// normalizeVirtualPersona enforces the naming rule for real-person personas:
// name "Virtual {Name}", ID from the real name, and tensions only with
// members who are actually on the council.
func normalizeVirtualPersona(exp *expert.Expert, requested string) {
	base := strings.TrimSpace(strings.TrimPrefix(exp.Name, virtualPrefix))
	if base == "" {
		base = strings.TrimSpace(strings.TrimPrefix(requested, virtualPrefix))
	}
	exp.Name = virtualPrefix + base
	exp.ID = expert.ToID(base)
	if exp.Category == "" {
		exp.Category = "custom"
	}

	members := map[string]bool{}
	if list, err := expert.List(); err == nil {
		for _, m := range list {
			members[m.ID] = true
		}
	}
	var kept []expert.Tension
	for _, t := range exp.Tensions {
		if members[t.Expert] && t.Expert != exp.ID {
			kept = append(kept, t)
		}
	}
	exp.Tensions = kept
}

// councilMemberList renders current members for the research prompt.
func councilMemberList() string {
	list, err := expert.List()
	if err != nil || len(list) == 0 {
		return "(none yet)"
	}
	var b strings.Builder
	for _, m := range list {
		fmt.Fprintf(&b, "- %s: %s — %s\n", m.ID, m.Name, m.Focus)
	}
	return b.String()
}
