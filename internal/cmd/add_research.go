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
	expert.NormalizeVirtual(exp, name)
	return exp, nil
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
