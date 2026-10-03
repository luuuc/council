package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

// TestMain keeps tests from calling a real AI CLI installed on the machine.
func TestMain(m *testing.M) {
	aiPrompt = func(string) (string, error) {
		return "", errors.New("AI disabled in tests")
	}
	os.Exit(m.Run())
}

func stubAI(t *testing.T, answer string) *string {
	t.Helper()
	var gotPrompt string
	orig := aiPrompt
	aiPrompt = func(prompt string) (string, error) {
		gotPrompt = prompt
		return answer, nil
	}
	t.Cleanup(func() { aiPrompt = orig })
	return &gotPrompt
}

const borisPersona = `---
id: boris
name: Boris Cherny
focus: TypeScript and AI-assisted engineering
influences:
  - "Programming TypeScript — types as design tools"
philosophy: |
  I let the types carry the design.
principles:
  - Make illegal states unrepresentable
red_flags:
  - any types leaking through APIs
tensions:
  - expert: rob-pike
    topic: type systems
    position: Rich types catch bugs early
    counterpoint: Simple types keep code readable
  - expert: not-on-council
    topic: anything
    position: dropped
    counterpoint: dropped
---`

func TestAddCmd_ResearchesRealPerson(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		if err := addCmd.RunE(addCmd, []string{"Rob Pike"}); err != nil {
			t.Fatalf("adding Rob Pike: %v", err)
		}
		prompt := stubAI(t, borisPersona)
		addYes = true
		t.Cleanup(func() { addYes = false })

		if err := addCmd.RunE(addCmd, []string{"Boris Cherny"}); err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}

		if !strings.Contains(*prompt, `"Boris Cherny"`) || !strings.Contains(*prompt, "rob-pike: Virtual Rob Pike") {
			t.Errorf("research prompt should name the person and list council members, got:\n%s", *prompt)
		}

		e, err := expert.Load("boris-cherny")
		if err != nil {
			t.Fatalf("expected boris-cherny to be saved: %v", err)
		}
		if e.Name != "Virtual Boris Cherny" {
			t.Errorf("name = %q, want %q", e.Name, "Virtual Boris Cherny")
		}
		if len(e.Tensions) != 1 || e.Tensions[0].Expert != "rob-pike" {
			t.Errorf("tensions should keep only council members, got %+v", e.Tensions)
		}
		if !strings.Contains(e.Body, "## Drawn From") {
			t.Errorf("body should list the sources the persona is drawn from:\n%s", e.Body)
		}
	})
}

func TestAddCmd_UnknownPersonFallsBackToCustom(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		stubAI(t, "UNKNOWN")

		err := addCmd.RunE(addCmd, []string{"My CTO"})

		// Falls back to the manual flow, which needs a focus typed in.
		if err == nil || (!strings.Contains(err.Error(), "focus is required") && !strings.Contains(err.Error(), "custom persona")) {
			t.Fatalf("expected fallback to custom persona creation, got %v", err)
		}
		if expert.Exists("my-cto") {
			t.Error("unknown person should not be saved as a researched persona")
		}
	})
}

func TestFormatExpertForEditKeepsAllFields(t *testing.T) {
	e, err := parseGeneratedExpert(borisPersona)
	if err != nil {
		t.Fatal(err)
	}

	text, err := formatExpertForEdit(e)
	if err != nil {
		t.Fatal(err)
	}
	back, err := expert.Parse([]byte(text))
	if err != nil {
		t.Fatalf("edited text should parse: %v\n%s", err, text)
	}
	if len(back.Influences) != 1 || len(back.Tensions) != 2 {
		t.Errorf("influences and tensions should survive editing, got %d and %d", len(back.Influences), len(back.Tensions))
	}
}
