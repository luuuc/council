package review

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestRunCouncils(t *testing.T) {
	backend := &MockBackend{
		Results: map[string]ExpertVerdict{
			"gia":    {Verdict: VerdictComment, Notes: []string{"Validate demand first"}},
			"dev": {Verdict: VerdictBlock, Notes: []string{"Require 2FA"}},
			// Spokespersons and the moderator answer through RawPrompt (Notes[0]).
			"product-council":  {Notes: []string{`{"position":"Don't add signup friction before we know the drop-off cause.","challenges":[{"to":"security","stance":"disagree","note":"Mandatory 2FA at signup costs conversions"},{"to":"product","stance":"agree","note":"self-reference is dropped"}]}`}},
			"security-council": {Notes: []string{`{"position":"Accounts with bank details need a second factor.","challenges":[{"to":"product","stance":"disagree","note":"A takeover costs more than a signup"},{"to":"nobody","stance":"agree","note":"unknown council is dropped"}]}`}},
			"moderator":        {Notes: []string{`{"disagreements":[{"topic":"2FA at signup?","sides":[{"experts":["product"],"position":"No"},{"experts":["security"],"position":"Yes"}]}],"decisions":["Where do you take the friction?"]}`}},
		},
	}
	runner := &Runner{Backend: backend, Options: ReviewOptions{Timeout: 10}}

	councils := []Council{
		{Name: "product", Inputs: []ExpertInput{{Expert: &expert.Expert{ID: "gia", Name: "Virtual Gia"}}}},
		{Name: "security", Inputs: []ExpertInput{{Expert: &expert.Expert{ID: "dev", Name: "Virtual Dev"}}}},
	}

	var events []string
	hooks := CouncilHooks{
		OnCouncilStart: func(name string, n int) { events = append(events, "council "+name) },
		OnStatement:    func(s CouncilStatement) { events = append(events, "statement "+s.Council) },
		OnCrossStart:   func(label string) { events = append(events, "cross "+label) },
	}
	r := runner.RunCouncils(context.Background(), councils, Submission{Content: "Require 2FA at signup"}, hooks)

	want := "[council product council security cross Spokesperson for the product council statement product cross Spokesperson for the security council statement security cross Moderator across councils]"
	if fmt.Sprint(events) != want {
		t.Errorf("events = %v\nwant %s", events, want)
	}

	if len(r.Councils) != 2 || r.Councils[0].Result.Perspectives[0].Expert != "gia" {
		t.Fatalf("each council should have its own review, got %+v", r.Councils)
	}
	if len(r.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d (errors: %v)", len(r.Statements), r.Errors)
	}
	for _, s := range r.Statements {
		if len(s.Challenges) != 1 || s.Challenges[0].To == s.Council {
			t.Errorf("%s: challenges must name another known council, got %+v", s.Council, s.Challenges)
		}
	}
	if len(r.Disagreements) != 1 || len(r.Decisions) != 1 {
		t.Errorf("expected the cross-council moderation, got %+v / %+v", r.Disagreements, r.Decisions)
	}

	// The spokesperson sees its own council's review and the other council's.
	var productPrompt string
	for _, c := range backend.seen {
		if c.expert == "product-council" {
			productPrompt = c.sub.RawPrompt
		}
	}
	for _, want := range []string{"You speak for the product council", "## Your Council's Review", "### The product council", "### The security council", "Require 2FA"} {
		if !strings.Contains(productPrompt, want) {
			t.Errorf("spokesperson prompt missing %q:\n%s", want, productPrompt)
		}
	}

	out := FormatHumanCouncils(r)
	for _, want := range []string{"The product council (1 members)", "The councils answer each other", "→ disagrees with the security council:", "1. 2FA at signup?", "- the product council: No", "Where do you take the friction?"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
