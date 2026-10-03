package cmd

import (
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestCouncilCandidates(t *testing.T) {
	picks := []*expert.Expert{findExpertByID("rob-pike"), findExpertByID("kent-beck")}

	got := councilCandidates(picks)

	if len(got) < 3 {
		t.Fatalf("expected picks plus challengers, got %d candidates", len(got))
	}
	for i, c := range got[:2] {
		if !c.Checked || c.Why != "" || c.Expert.ID != picks[i].ID {
			t.Errorf("candidate %d should be the checked pick %s, got %+v", i, picks[i].ID, c)
		}
	}

	seen := map[string]bool{}
	sawTension := false
	for _, c := range got[2:] {
		if c.Checked {
			t.Errorf("challenger %s should start unchecked", c.Expert.ID)
		}
		if c.Expert.ID == "rob-pike" || c.Expert.ID == "kent-beck" || seen[c.Expert.ID] {
			t.Errorf("challenger %s is listed twice", c.Expert.ID)
		}
		seen[c.Expert.ID] = true
		if strings.HasPrefix(c.Why, "disagrees with Virtual Rob Pike") {
			sawTension = true
		}
	}
	if !sawTension {
		t.Error("expected a challenger drawn from Rob Pike's tensions")
	}
	sawAngle := false
	for _, c := range got[2:] {
		if !strings.HasPrefix(c.Why, "disagrees with") {
			sawAngle = true
		}
	}
	if !sawAngle {
		t.Error("expected room for a different-angle voice (product, customers, security, operations)")
	}
	if len(got)-len(picks) > maxChallengers {
		t.Errorf("expected at most %d challengers, got %d", maxChallengers, len(got)-len(picks))
	}
}

func TestPickCouncil(t *testing.T) {
	picks := []*expert.Expert{findExpertByID("rob-pike"), findExpertByID("kent-beck")}
	candidates := councilCandidates(picks)
	challenger := candidates[2].Expert.ID

	// Uncheck Kent Beck, check the first challenger, ignore junk, confirm.
	var out strings.Builder
	got, err := pickCouncil(strings.NewReader("2 3 99 x\n\n"), &out, picks, nil)
	if err != nil {
		t.Fatalf("pickCouncil: %v", err)
	}

	var ids []string
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	if strings.Join(ids, ",") != "rob-pike,"+challenger {
		t.Errorf("picked %v, want rob-pike and %s", ids, challenger)
	}
	if !strings.Contains(out.String(), "[x] Virtual Rob Pike") || !strings.Contains(out.String(), `Ignoring "99"`) {
		t.Errorf("unexpected picker output:\n%s", out.String())
	}

	// Unchecking everyone requires picking someone before confirming.
	if _, err := pickCouncil(strings.NewReader("1 2\n\n"), &out, picks, nil); err == nil {
		t.Error("expected an error when input ends with nobody picked")
	}
}

func TestPickCouncilAddsCustomer(t *testing.T) {
	picks := []*expert.Expert{findExpertByID("rob-pike")}
	newCustomer := func(desc string) (*expert.Expert, error) {
		if desc != "solo founders" {
			t.Errorf("description = %q", desc)
		}
		return &expert.Expert{ID: "customer-solo-founder", Name: "Customer: Solo Founder", Focus: "runs everything alone", Kind: expert.KindCustomer}, nil
	}

	var out strings.Builder
	got, err := pickCouncil(strings.NewReader("c\nsolo founders\n\n"), &out, picks, newCustomer)
	if err != nil {
		t.Fatalf("pickCouncil: %v", err)
	}
	if len(got) != 2 || got[1].ID != "customer-solo-founder" {
		t.Fatalf("expected Rob Pike plus the new customer, got %d members", len(got))
	}
	if !strings.Contains(out.String(), "[x] Customer: Solo Founder — customer: runs everything alone") {
		t.Errorf("the new customer should be listed and checked:\n%s", out.String())
	}
}
