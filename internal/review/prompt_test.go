package review

import (
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestBuildPrompt(t *testing.T) {
	e := &expert.Expert{
		ID:   "ada-redgrave",
		Name: "Ada Redgrave",
		Body: "# Ada Redgrave - Test-driven development\n\nYou are Ada Redgrave.",
	}

	sub := Submission{
		Content: "diff --git a/main.go\n+func Add(a, b int) int {\n+    return a + b\n+}",
		Context: "PR: Add math utilities",
	}

	prompt := BuildPrompt(e, sub)

	checks := []string{
		"You are Ada Redgrave",
		"Ada Redgrave - Test-driven development",
		"diff --git a/main.go",
		"PR: Add math utilities",
		`"verdict"`,
		`"ada-redgrave"`,
		"pass|comment|block|escalate",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing %q\n\nPrompt:\n%s", check, prompt)
		}
	}
}

func TestBuildPromptNoContext(t *testing.T) {
	e := &expert.Expert{
		ID:   "ada-redgrave",
		Name: "Ada Redgrave",
		Body: "# Ada Redgrave\n\nExpert.",
	}

	sub := Submission{Content: "some code"}

	prompt := BuildPrompt(e, sub)

	if strings.Contains(prompt, "## Context") {
		t.Error("prompt should not contain Context section when context is empty")
	}
}

func TestBuildCollectivePrompt(t *testing.T) {
	experts := []*expert.Expert{
		{ID: "ada-redgrave", Name: "Ada Redgrave", Focus: "Test-driven development", Body: "TDD expert."},
		{ID: "diego-valdez", Name: "Diego Valdez", Focus: "Convention over configuration", Body: "Rails creator."},
		{ID: "owasp-sentinel", Name: "OWASP Sentinel", Focus: "Application security", Body: "Security expert."},
	}

	sub := Submission{
		Content: "diff --git a/main.go\n+func Add(a, b int) int {\n+    return a + b\n+}",
		Context: "PR: Add math utilities",
	}

	prompt := BuildCollectivePrompt(experts, sub)

	checks := []string{
		"council of expert reviewers",
		"Ada Redgrave — Test-driven development",
		"Diego Valdez — Convention over configuration",
		"OWASP Sentinel — Application security",
		"TDD expert.",
		"Rails creator.",
		"Security expert.",
		"diff --git a/main.go",
		"PR: Add math utilities",
		`"perspectives"`,
		`"agreements"`,
		`"tension"`,
		"react to each other",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("collective prompt missing %q", check)
		}
	}
}

func TestBuildCollectivePromptNoContext(t *testing.T) {
	experts := []*expert.Expert{
		{ID: "ada-redgrave", Name: "Ada Redgrave", Focus: "TDD", Body: "Expert."},
	}

	prompt := BuildCollectivePrompt(experts, Submission{Content: "some code"})

	if strings.Contains(prompt, "## Context") {
		t.Error("collective prompt should not contain Context section when context is empty")
	}
}
