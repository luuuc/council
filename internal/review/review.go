// Package review implements the council review engine: sequential expert
// reviews where each expert reacts to the ones before, with structured
// verdict parsing and tension-aware synthesis.
package review

// Verdict represents the possible review outcomes.
type Verdict string

const (
	VerdictPass     Verdict = "pass"
	VerdictComment  Verdict = "comment"
	VerdictBlock    Verdict = "block"
	VerdictEscalate Verdict = "escalate"
)

// ValidVerdicts is the set of recognized verdict values.
var ValidVerdicts = map[Verdict]bool{
	VerdictPass:     true,
	VerdictComment:  true,
	VerdictBlock:    true,
	VerdictEscalate: true,
}

// Severity returns the numeric severity for ordering verdicts.
// Higher is more severe.
func (v Verdict) Severity() int {
	switch v {
	case VerdictPass:
		return 0
	case VerdictComment:
		return 1
	case VerdictBlock:
		return 2
	case VerdictEscalate:
		return 3
	default:
		return 1 // unknown defaults to comment-level
	}
}

// ExpertVerdict is the structured output from a single expert review.
type ExpertVerdict struct {
	Expert     string   `json:"expert"`
	Name       string   `json:"name,omitempty"`
	Verdict    Verdict  `json:"verdict"`
	Confidence float64  `json:"confidence"`
	Notes      []string `json:"notes"`
	Replies    []Reply  `json:"replies,omitempty"`
	Blocking   bool     `json:"blocking"`
	Error      string   `json:"error,omitempty"`
}

// Stance is how an expert reacts to an earlier review.
type Stance string

const (
	StanceAgree    Stance = "agree"
	StanceDisagree Stance = "disagree"
	StanceAdds     Stance = "adds"
)

// Reply is an expert's reaction to an earlier expert's review.
type Reply struct {
	To     string `json:"to"`
	Stance Stance `json:"stance"`
	Note   string `json:"note"`
}

// Submission is the material being reviewed.
type Submission struct {
	Content   string // The diff, file content, or text to review
	Context   string // Optional context (e.g., PR title)
	RawPrompt string // When set, backends use this as the prompt directly (bypasses BuildPrompt and ParseVerdict)
	Prior     []ExpertVerdict // Reviews from experts who spoke earlier in a sequential review
}

// SynthesizedResult is the aggregated output from all expert reviews.
type SynthesizedResult struct {
	Verdict      Verdict         `json:"verdict"`
	Blocking     bool            `json:"blocking"`
	Perspectives []ExpertVerdict `json:"perspectives"`
	Agreements   []string        `json:"agreements"`
	Tension      string          `json:"tension"`
	Summary      string          `json:"summary"`
	Errors       []string        `json:"errors,omitempty"`
}

// Mode selects how experts review.
type Mode string

const (
	// ModeSequential runs one call per expert, in order. Each expert sees
	// the earlier reviews and reacts to them. This is the default.
	ModeSequential Mode = "sequential"
	// ModeCollective runs one call that plays every expert at once.
	// Cheaper, but the debate is simulated rather than real.
	ModeCollective Mode = "collective"
)

// ReviewOptions controls review execution.
type ReviewOptions struct {
	Mode    Mode
	Timeout int // per-call timeout in seconds
}
