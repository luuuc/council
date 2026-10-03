package review

import (
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestSynthesize(t *testing.T) {
	securityExpert := &expert.Expert{
		ID:    "dev",
		Name:  "Dev",
		Focus: "Application security",
	}
	qualityExpert := &expert.Expert{
		ID:    "ada",
		Name:  "Ada",
		Focus: "Test-driven development",
		Tensions: []expert.Tension{
			{
				Expert:       "eve",
				Topic:        "abstraction",
				Position:     "Extract when the pattern emerges three times",
				Counterpoint: "Don't build for formats nobody asked for",
			},
		},
	}
	scopeExpert := &expert.Expert{
		ID:    "eve",
		Name:  "Eve",
		Focus: "Product simplicity and scope",
	}

	tests := []struct {
		name         string
		verdicts     []ExpertVerdict
		experts      []*expert.Expert
		errors       []string
		wantVerdict  Verdict
		wantBlocking bool
		wantTension  bool
	}{
		{
			name: "all pass",
			verdicts: []ExpertVerdict{
				{Expert: "dev", Verdict: VerdictPass, Confidence: 0.9},
				{Expert: "ada", Verdict: VerdictPass, Confidence: 0.85},
			},
			experts:      []*expert.Expert{securityExpert, qualityExpert},
			wantVerdict:  VerdictPass,
			wantBlocking: false,
		},
		{
			name: "one block overrides passes",
			verdicts: []ExpertVerdict{
				{Expert: "dev", Verdict: VerdictBlock, Confidence: 0.9, Blocking: true},
				{Expert: "ada", Verdict: VerdictPass, Confidence: 0.85},
			},
			experts:      []*expert.Expert{securityExpert, qualityExpert},
			wantVerdict:  VerdictBlock,
			wantBlocking: true,
		},
		{
			name: "block without blocking flag is not blocking",
			verdicts: []ExpertVerdict{
				{Expert: "dev", Verdict: VerdictBlock, Confidence: 0.9, Blocking: false},
				{Expert: "ada", Verdict: VerdictPass, Confidence: 0.85},
			},
			experts:      []*expert.Expert{securityExpert, qualityExpert},
			wantVerdict:  VerdictBlock,
			wantBlocking: false,
		},
		{
			name: "mixed with tension pair",
			verdicts: []ExpertVerdict{
				{Expert: "ada", Verdict: VerdictComment, Confidence: 0.8},
				{Expert: "eve", Verdict: VerdictPass, Confidence: 0.85},
			},
			experts:     []*expert.Expert{qualityExpert, scopeExpert},
			wantVerdict: VerdictComment,
			wantTension: true,
		},
		{
			name: "escalate is highest severity",
			verdicts: []ExpertVerdict{
				{Expert: "dev", Verdict: VerdictEscalate, Confidence: 0.5},
				{Expert: "ada", Verdict: VerdictBlock, Confidence: 0.9},
			},
			experts:     []*expert.Expert{securityExpert, qualityExpert},
			wantVerdict: VerdictEscalate,
		},
		{
			name:        "no verdicts with errors",
			verdicts:    nil,
			experts:     nil,
			errors:      []string{"ada: timeout"},
			wantVerdict: VerdictPass, // default
		},
		{
			name: "failed expert excluded from verdict",
			verdicts: []ExpertVerdict{
				{Expert: "ada", Verdict: VerdictPass, Confidence: 0.9},
			},
			experts:     []*expert.Expert{qualityExpert},
			errors:      []string{"dev: timeout"},
			wantVerdict: VerdictPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Synthesize(tt.verdicts, tt.experts, tt.errors)

			if result.Verdict != tt.wantVerdict {
				t.Errorf("Verdict = %q, want %q", result.Verdict, tt.wantVerdict)
			}
			if result.Blocking != tt.wantBlocking {
				t.Errorf("Blocking = %v, want %v", result.Blocking, tt.wantBlocking)
			}
			if tt.wantTension && result.Tension == "" {
				t.Error("expected Tension to be set, got empty")
			}
			if !tt.wantTension && result.Tension != "" {
				t.Errorf("unexpected Tension: %q", result.Tension)
			}
			if result.Summary == "" {
				t.Error("Summary should not be empty")
			}
		})
	}
}

func TestResolveOverallVerdict(t *testing.T) {
	securityExpert := &expert.Expert{ID: "sec", Name: "Sec", Focus: "Application security"}
	qualityExpert := &expert.Expert{ID: "qual", Name: "Qual", Focus: "Test-driven development"}

	byID := map[string]*expert.Expert{
		"sec":  securityExpert,
		"qual": qualityExpert,
	}

	tests := []struct {
		name     string
		verdicts []ExpertVerdict
		want     Verdict
	}{
		{
			name: "correct LLM verdict passes through",
			verdicts: []ExpertVerdict{
				{Expert: "sec", Verdict: VerdictBlock},
				{Expert: "qual", Verdict: VerdictPass},
			},
			want: VerdictBlock,
		},
		{
			name: "all pass",
			verdicts: []ExpertVerdict{
				{Expert: "sec", Verdict: VerdictPass},
				{Expert: "qual", Verdict: VerdictPass},
			},
			want: VerdictPass,
		},
		{
			name: "escalate wins over block",
			verdicts: []ExpertVerdict{
				{Expert: "sec", Verdict: VerdictEscalate},
				{Expert: "qual", Verdict: VerdictBlock},
			},
			want: VerdictEscalate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveOverallVerdict(tt.verdicts, byID)
			if got != tt.want {
				t.Errorf("ResolveOverallVerdict = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveBlocking(t *testing.T) {
	tests := []struct {
		name     string
		verdicts []ExpertVerdict
		want     bool
	}{
		{
			name: "blocking expert with block verdict",
			verdicts: []ExpertVerdict{
				{Expert: "sec", Verdict: VerdictBlock, Blocking: true},
				{Expert: "qual", Verdict: VerdictPass, Blocking: false},
			},
			want: true,
		},
		{
			name: "blocking expert with pass verdict",
			verdicts: []ExpertVerdict{
				{Expert: "sec", Verdict: VerdictPass, Blocking: true},
				{Expert: "qual", Verdict: VerdictBlock, Blocking: false},
			},
			want: false,
		},
		{
			name: "no blocking experts",
			verdicts: []ExpertVerdict{
				{Expert: "sec", Verdict: VerdictBlock, Blocking: false},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBlocking(tt.verdicts)
			if got != tt.want {
				t.Errorf("ResolveBlocking = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSummary(t *testing.T) {
	summary := buildSummary(
		[]ExpertVerdict{
			{Expert: "a", Verdict: VerdictPass},
			{Expert: "b", Verdict: VerdictComment},
		},
		[]string{"c: timeout"},
		VerdictComment,
		false,
	)

	if !strings.Contains(summary, "3 experts reviewed") {
		t.Errorf("expected '3 experts reviewed' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "1 pass") {
		t.Errorf("expected '1 pass' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "1 failed") {
		t.Errorf("expected '1 failed' in summary, got: %s", summary)
	}
}
