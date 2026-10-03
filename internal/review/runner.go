package review

import (
	"context"
	"log"
	"time"

	"github.com/luuuc/council/internal/expert"
)

// Runner orchestrates expert reviews.
type Runner struct {
	Backend Backend
	Options ReviewOptions

	// OnStart, if set, is called before each turn of a sequential review.
	OnStart func(t Turn)
	// OnVerdict, if set, is called after each member turn (a review or a
	// final word) with all verdicts so far, in speaking order.
	OnVerdict func(t Turn, verdicts []ExpertVerdict)
}

// ExpertInput pairs an expert with their blocking status from the pack.
type ExpertInput struct {
	Expert   *expert.Expert
	Blocking bool
}

// CollectiveThreshold is the byte-count threshold for the collective prompt.
// If the prompt exceeds this, the runner falls back to sequential review.
// Default: 32KB (~8K tokens) — conservative for small-context models.
const CollectiveThreshold = 32 * 1024

// Run executes a sequential review by default: one call per expert, in order,
// each seeing the reviews before it. ModeCollective runs one call for all
// experts instead, falling back to sequential when the prompt is too large
// or the call fails.
func (r *Runner) Run(ctx context.Context, inputs []ExpertInput, sub Submission) *SynthesizedResult {
	if r.Options.Mode != ModeCollective || len(inputs) == 1 {
		return r.runSequential(ctx, inputs, sub)
	}

	if estimateCollectiveSize(inputs, sub) > CollectiveThreshold {
		log.Println("collective prompt exceeds context threshold, falling back to sequential review")
		return r.runSequential(ctx, inputs, sub)
	}

	return r.runCollective(ctx, inputs, sub)
}

func (r *Runner) timeout() time.Duration {
	if r.Options.Timeout <= 0 {
		return 120 * time.Second
	}
	return time.Duration(r.Options.Timeout) * time.Second
}

// estimateCollectiveSize approximates the collective prompt size in bytes
// without building the full string. Sums expert content + submission + template overhead.
func estimateCollectiveSize(inputs []ExpertInput, sub Submission) int {
	const templateOverhead = 800
	size := templateOverhead + len(sub.Content) + len(sub.Context)
	for _, inp := range inputs {
		size += len(inp.Expert.Name) + len(inp.Expert.Focus) + len(inp.Expert.Body) + 20
	}
	return size
}

// runCollective executes a single collective LLM call.
// Falls back to sequential review if the collective call fails.
func (r *Runner) runCollective(ctx context.Context, inputs []ExpertInput, sub Submission) *SynthesizedResult {
	experts := make([]*expert.Expert, len(inputs))
	for i, inp := range inputs {
		experts[i] = inp.Expert
	}

	callCtx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	result, err := r.Backend.ReviewCollective(callCtx, experts, sub)
	if err != nil {
		log.Printf("collective review failed, falling back to sequential: %s", err)
		return r.runSequential(ctx, inputs, sub)
	}

	// Override Blocking per perspective from pack config
	byID := make(map[string]*expert.Expert, len(experts))
	blockingByID := make(map[string]bool, len(inputs))
	for _, inp := range inputs {
		byID[inp.Expert.ID] = inp.Expert
		blockingByID[inp.Expert.ID] = inp.Blocking
	}
	for i := range result.Perspectives {
		p := &result.Perspectives[i]
		p.Blocking = blockingByID[p.Expert]
		if e, ok := byID[p.Expert]; ok {
			p.Name = e.Name
		}
	}

	// Validate overall verdict against hierarchy
	hierarchyVerdict := ResolveOverallVerdict(result.Perspectives, byID)
	if hierarchyVerdict.Severity() > result.Verdict.Severity() {
		result.Verdict = hierarchyVerdict
	}
	result.Blocking = ResolveBlocking(result.Perspectives)

	return result
}

// runSequential runs the debate turn by turn: each member reviews in order
// seeing the earlier reviews, then (if enabled) earlier members get a final
// word, then a moderator lists disagreements and decisions. A failed turn is
// recorded as an error and the debate continues.
func (r *Runner) runSequential(ctx context.Context, inputs []ExpertInput, sub Submission) *SynthesizedResult {
	d := NewDebate(inputs, sub, r.Options.FinalWord, r.Options.Moderate)

	for turn, ok := d.Next(); ok; turn, ok = d.Next() {
		if r.OnStart != nil {
			r.OnStart(turn)
		}

		callCtx, cancel := context.WithTimeout(ctx, r.timeout())
		if turn.Kind == TurnModerator {
			v, err := r.Backend.Review(callCtx, Moderator, Submission{RawPrompt: turn.Prompt()})
			cancel()
			if err != nil {
				d.Fail(err)
				continue
			}
			raw := ""
			if len(v.Notes) > 0 {
				raw = v.Notes[0]
			}
			if !d.RecordModeration(raw) {
				log.Println("moderator response could not be read; showing disagreements from replies instead")
			}
			continue
		}

		verdict, err := r.Backend.Review(callCtx, turn.Expert, turn.Sub)
		cancel()
		if err != nil {
			d.Fail(err)
			continue
		}
		d.RecordVerdict(verdict)
		if r.OnVerdict != nil {
			r.OnVerdict(turn, d.Verdicts())
		}
	}

	return d.Result()
}
