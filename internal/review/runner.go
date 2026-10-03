package review

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/luuuc/council/internal/expert"
)

// Runner orchestrates expert reviews.
type Runner struct {
	Backend Backend
	Options ReviewOptions
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

// runSequential runs one review per expert, in order. Each expert receives
// the earlier verdicts in its prompt so it can react to them. A failed expert
// is recorded as an error and the review continues with the next one.
func (r *Runner) runSequential(ctx context.Context, inputs []ExpertInput, sub Submission) *SynthesizedResult {
	var verdicts []ExpertVerdict
	var errors []string
	experts := make([]*expert.Expert, 0, len(inputs))

	for _, inp := range inputs {
		experts = append(experts, inp.Expert)

		turn := sub
		turn.Prior = verdicts

		callCtx, cancel := context.WithTimeout(ctx, r.timeout())
		verdict, err := r.Backend.Review(callCtx, inp.Expert, turn)
		cancel()
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", inp.Expert.ID, err))
			continue
		}

		verdict.Name = inp.Expert.Name
		verdict.Blocking = inp.Blocking
		verdicts = append(verdicts, verdict)
	}

	return Synthesize(verdicts, experts, errors)
}
