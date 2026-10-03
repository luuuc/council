package review

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatHuman(t *testing.T) {
	result := &SynthesizedResult{
		Verdict:  VerdictComment,
		Blocking: false,
		Perspectives: []ExpertVerdict{
			{Expert: "kent-beck", Verdict: VerdictComment, Confidence: 0.85, Notes: []string{"Missing test coverage"}},
			{Expert: "bruce-schneier", Verdict: VerdictPass, Confidence: 0.95},
		},
		Agreements: []string{"All 2 experts agree code structure is clean."},
		Tension:    "Kent Beck vs Jason Fried on abstraction",
		Summary:    "2 experts reviewed. 1 pass, 1 comment. Ship with comments.",
	}

	output := FormatHuman(result, "rails", 2)

	checks := []string{
		"Council Review",
		"pack: rails",
		"kent-beck",
		"comment",
		"bruce-schneier",
		"pass",
		"Missing test coverage",
		"Tension:",
		"ship with comments",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("output missing %q\n\nFull output:\n%s", check, output)
		}
	}
}

func TestFormatHumanNoPack(t *testing.T) {
	result := &SynthesizedResult{
		Verdict: VerdictPass,
		Perspectives: []ExpertVerdict{
			{Expert: "kent-beck", Verdict: VerdictPass, Confidence: 0.9},
		},
		Summary: "1 expert reviewed. 1 pass. Ship it.",
	}

	output := FormatHuman(result, "", 1)

	if strings.Contains(output, "pack:") {
		t.Error("should not contain 'pack:' when no pack specified")
	}
	if !strings.Contains(output, "1 experts") {
		t.Errorf("expected expert count in header, got:\n%s", output)
	}
}

func TestFormatHumanWithErrors(t *testing.T) {
	result := &SynthesizedResult{
		Verdict: VerdictPass,
		Perspectives: []ExpertVerdict{
			{Expert: "kent-beck", Verdict: VerdictPass, Confidence: 0.9},
		},
		Errors:  []string{"bruce-schneier: timeout"},
		Summary: "2 experts reviewed. 1 pass, 1 failed.",
	}

	output := FormatHuman(result, "rails", 2)

	if !strings.Contains(output, "Error: bruce-schneier: timeout") {
		t.Errorf("expected error in output, got:\n%s", output)
	}
}

func TestFormatJSON(t *testing.T) {
	result := &SynthesizedResult{
		Verdict:  VerdictComment,
		Blocking: false,
		Perspectives: []ExpertVerdict{
			{Expert: "kent-beck", Verdict: VerdictComment, Confidence: 0.85},
		},
		Summary: "1 expert reviewed.",
	}

	data, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	// Verify it's valid JSON
	var parsed SynthesizedResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed.Verdict != VerdictComment {
		t.Errorf("parsed verdict = %q, want %q", parsed.Verdict, VerdictComment)
	}
}

func TestFormatHumanShowsNamesAndReplies(t *testing.T) {
	result := &SynthesizedResult{
		Verdict: VerdictComment,
		Perspectives: []ExpertVerdict{
			{Expert: "dhh", Name: "Virtual DHH", Verdict: VerdictBlock, Notes: []string{"Too many layers"}},
			{Expert: "kent-beck", Name: "Virtual Kent Beck", Verdict: VerdictComment,
				Replies: []Reply{{To: "dhh", Stance: StanceDisagree, Note: "The layers make it testable"}}},
		},
		Tension: "Virtual Kent Beck disagrees with Virtual DHH: The layers make it testable\nVirtual DHH disagrees with nobody",
	}

	out := FormatHuman(result, "", 2)

	for _, want := range []string{
		"Virtual DHH",
		"  → disagrees with Virtual DHH:\n    The layers make it testable",
		"Tension:\n  - Virtual Kent Beck disagrees with Virtual DHH",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n\n%s", want, out)
		}
	}
}
