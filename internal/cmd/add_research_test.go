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

const jayPersona = `---
id: boris
name: Jay
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
  - expert: cleo
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
		member := &expert.Expert{ID: "cleo", Name: "Virtual Cleo", Focus: "Simple code"}
		if err := member.Save(); err != nil {
			t.Fatalf("saving Cleo: %v", err)
		}
		prompt := stubAI(t, jayPersona)
		addYes = true
		t.Cleanup(func() { addYes = false })

		if err := addCmd.RunE(addCmd, []string{"Jay"}); err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}

		if !strings.Contains(*prompt, `"Jay"`) || !strings.Contains(*prompt, "cleo: Virtual Cleo") {
			t.Errorf("research prompt should name the person and list council members, got:\n%s", *prompt)
		}

		e, err := expert.Load("jay")
		if err != nil {
			t.Fatalf("expected jay to be saved: %v", err)
		}
		if e.Name != "Virtual Jay" {
			t.Errorf("name = %q, want %q", e.Name, "Virtual Jay")
		}
		if len(e.Tensions) != 1 || e.Tensions[0].Expert != "cleo" {
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
	e, err := parseGeneratedExpert(jayPersona)
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

func TestAddCmd_Customer(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		prompt := stubAI(t, `---
name: Freelance Designer
focus: Solo designer who bills five to ten clients a month
kind: customer
philosophy: |
  I juggle client work and invoicing alone.
principles:
  - Send an invoice in under two minutes
red_flags:
  - Setup that takes an afternoon
tensions:
  - expert: cleo
    topic: x
    position: y
    counterpoint: z
---`)
		addYes, addCustomer = true, true
		t.Cleanup(func() { addYes, addCustomer = false, false })

		if err := addCmd.RunE(addCmd, []string{"solo designers who invoice monthly"}); err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}
		if !strings.Contains(*prompt, "solo designers who invoice monthly") {
			t.Errorf("prompt should carry the description:\n%s", *prompt)
		}

		e, err := expert.Load("customer-freelance-designer")
		if err != nil {
			t.Fatalf("customer not saved: %v", err)
		}
		if e.Name != "Customer: Freelance Designer" || e.Kind != expert.KindCustomer || len(e.Tensions) != 0 {
			t.Errorf("unexpected customer: name=%q kind=%q tensions=%d", e.Name, e.Kind, len(e.Tensions))
		}
		if !strings.Contains(e.Body, "What You're Trying to Get Done") || !strings.Contains(e.Body, "react to it as a user") {
			t.Errorf("customer body should frame them as a user:\n%s", e.Body)
		}
	})
}

func TestAddCmd_Role(t *testing.T) {
	testInTempDir(t, func(t *testing.T, dir string) {
		stubAI(t, `---
name: Virtual Site Reliability Engineer
focus: Uptime, rollbacks, and on-call load
principles:
  - Every change needs a rollback plan
red_flags:
  - Deploys without alerts
---`)
		addYes, addRole = true, true
		t.Cleanup(func() { addYes, addRole = false, false })

		if err := addCmd.RunE(addCmd, []string{"SRE"}); err != nil {
			t.Fatalf("addCmd failed: %v", err)
		}
		e, err := expert.Load("site-reliability-engineer")
		if err != nil {
			t.Fatalf("role not saved: %v", err)
		}
		if e.Name != "Site Reliability Engineer" || e.Kind != expert.KindRole {
			t.Errorf("roles are never Virtual: name=%q kind=%q", e.Name, e.Kind)
		}
	})
}

func TestParseGeneratedExpertWithoutClosingMarker(t *testing.T) {
	for _, raw := range []string{
		"---\nname: Freelance Designer\nfocus: Bills clients monthly\n",
		"name: Freelance Designer\nfocus: Bills clients monthly",
	} {
		e, err := parseGeneratedExpert(raw)
		if err != nil || e.Name != "Freelance Designer" {
			t.Errorf("parseGeneratedExpert(%q) = %+v, %v", raw, e, err)
		}
	}
}
