package cmd

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"os"
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

//go:embed prompts/customer.txt
var customerPrompt string

//go:embed prompts/role.txt
var rolePrompt string

// generateCustomer builds a customer persona from a description of the
// people the work is for.
func generateCustomer(description string) (*expert.Expert, error) {
	exp, err := generateExpert(fmt.Sprintf(customerPrompt, description))
	if err != nil {
		return nil, err
	}
	expert.NormalizeCustomer(exp)
	return exp, nil
}

// generateRole builds a persona for a role such as SRE or security engineer.
func generateRole(title string) (*expert.Expert, error) {
	exp, err := generateExpert(fmt.Sprintf(rolePrompt, title, councilMemberList()))
	if err != nil {
		return nil, err
	}
	expert.NormalizeRole(exp, title)
	return exp, nil
}

// runAddGenerated builds a persona with the AI, then previews it (or saves
// it directly with --yes or without a terminal).
func runAddGenerated(what string, generate func() (*expert.Expert, error)) error {
	fmt.Printf("Building %s...\n\n", what)

	exp, err := generate()
	if err != nil {
		return fmt.Errorf("could not build %s: %w", what, err)
	}
	if expert.Exists(exp.ID) {
		return fmt.Errorf("expert '%s' already exists", exp.ID)
	}

	if addYes || !isInteractive() {
		if err := exp.Save(); err != nil {
			return err
		}
		fmt.Printf("Added %s (%s)\n", exp.Name, exp.ID)
		fmt.Printf("File: %s\n", exp.Path())
		runAutoSync(addNoSync, nil)
		return nil
	}

	return reviewGeneratedExpert(bufio.NewReader(os.Stdin), exp, generate)
}
