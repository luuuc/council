package review

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luuuc/council/internal/expert"
)

// MockBackend returns canned verdicts for testing.
type MockBackend struct {
	Results          map[string]ExpertVerdict
	Errors           map[string]error
	CollectiveResult *SynthesizedResult
	CollectiveErr    error
	Delay            time.Duration
	calls            atomic.Int32
	collectiveCalls  atomic.Int32
	seen             []reviewCall // Review calls in order (sequential runs only)
}

type reviewCall struct {
	expert string
	sub    Submission
}

func (m *MockBackend) Review(ctx context.Context, e *expert.Expert, sub Submission) (ExpertVerdict, error) {
	m.calls.Add(1)
	m.seen = append(m.seen, reviewCall{expert: e.ID, sub: sub})

	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return ExpertVerdict{}, ctx.Err()
		}
	}

	if err, ok := m.Errors[e.ID]; ok {
		return ExpertVerdict{}, err
	}

	if v, ok := m.Results[e.ID]; ok {
		return v, nil
	}

	return ExpertVerdict{
		Expert:     e.ID,
		Verdict:    VerdictPass,
		Confidence: 0.9,
	}, nil
}

func (m *MockBackend) ReviewCollective(ctx context.Context, experts []*expert.Expert, sub Submission) (*SynthesizedResult, error) {
	m.collectiveCalls.Add(1)

	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.CollectiveErr != nil {
		return nil, m.CollectiveErr
	}

	if m.CollectiveResult != nil {
		return m.CollectiveResult, nil
	}

	perspectives := make([]ExpertVerdict, len(experts))
	for i, e := range experts {
		perspectives[i] = ExpertVerdict{
			Expert: e.ID, Verdict: VerdictPass, Confidence: 0.9,
		}
	}
	return &SynthesizedResult{
		Verdict:      VerdictPass,
		Perspectives: perspectives,
		Summary:      "All good.",
	}, nil
}

func TestRunnerCollectiveHappyPath(t *testing.T) {
	backend := &MockBackend{
		CollectiveResult: &SynthesizedResult{
			Verdict: VerdictComment,
			Perspectives: []ExpertVerdict{
				{Expert: "kent-beck", Verdict: VerdictComment, Confidence: 0.8, Notes: []string{"Add test"}},
				{Expert: "bruce-schneier", Verdict: VerdictPass, Confidence: 0.95},
			},
			Agreements: []string{"Code structure is clean"},
			Tension:    "Kent wants more tests, Bruce is satisfied with security",
			Summary:    "Ship with comments.",
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Mode: ModeCollective, Timeout: 10},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}, Blocking: false},
		{Expert: &expert.Expert{ID: "bruce-schneier", Name: "Bruce Schneier", Focus: "Security"}, Blocking: true},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	if backend.collectiveCalls.Load() != 1 {
		t.Errorf("expected 1 ReviewCollective call, got %d", backend.collectiveCalls.Load())
	}
	if backend.calls.Load() != 0 {
		t.Errorf("expected 0 Review calls, got %d", backend.calls.Load())
	}
	if len(result.Perspectives) != 2 {
		t.Fatalf("expected 2 perspectives, got %d", len(result.Perspectives))
	}
	if result.Verdict != VerdictComment {
		t.Errorf("expected comment verdict, got %s", result.Verdict)
	}
	if result.Tension == "" {
		t.Error("expected tension to be preserved from LLM response")
	}
}

func TestRunnerCollectiveBlockingFromPackConfig(t *testing.T) {
	backend := &MockBackend{
		CollectiveResult: &SynthesizedResult{
			Verdict: VerdictBlock,
			Perspectives: []ExpertVerdict{
				{Expert: "kent-beck", Verdict: VerdictPass, Confidence: 0.9, Blocking: true},
				{Expert: "bruce-schneier", Verdict: VerdictBlock, Confidence: 0.9, Blocking: false},
			},
			Summary: "Block.",
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Mode: ModeCollective, Timeout: 10},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}, Blocking: false},
		{Expert: &expert.Expert{ID: "bruce-schneier", Name: "Bruce Schneier", Focus: "Security"}, Blocking: true},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	// kent-beck: LLM said blocking=true but pack says false
	if result.Perspectives[0].Blocking {
		t.Error("kent-beck blocking should be overridden to false from pack config")
	}
	// bruce-schneier: LLM said blocking=false but pack says true
	if !result.Perspectives[1].Blocking {
		t.Error("bruce-schneier blocking should be overridden to true from pack config")
	}
	// With bruce-schneier blocking + VerdictBlock, result should be blocking
	if !result.Blocking {
		t.Error("expected overall result to be blocking")
	}
}

func TestRunnerCollectiveHierarchyOverride(t *testing.T) {
	backend := &MockBackend{
		CollectiveResult: &SynthesizedResult{
			Verdict: VerdictPass, // LLM incorrectly says pass
			Perspectives: []ExpertVerdict{
				{Expert: "kent-beck", Verdict: VerdictPass, Confidence: 0.9},
				{Expert: "bruce-schneier", Verdict: VerdictBlock, Confidence: 0.9},
			},
			Summary: "Ship it.",
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Mode: ModeCollective, Timeout: 10},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}, Blocking: false},
		{Expert: &expert.Expert{ID: "bruce-schneier", Name: "Bruce Schneier", Focus: "Security"}, Blocking: true},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	if result.Verdict != VerdictBlock {
		t.Errorf("hierarchy should override LLM verdict to block, got %s", result.Verdict)
	}
}

func TestRunnerSingleExpertUsesSequentialPath(t *testing.T) {
	backend := &MockBackend{
		Results: map[string]ExpertVerdict{
			"kent-beck": {Expert: "kent-beck", Verdict: VerdictPass, Confidence: 0.9},
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Timeout: 10},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	if backend.calls.Load() != 1 {
		t.Errorf("expected 1 Review call for single expert, got %d", backend.calls.Load())
	}
	if backend.collectiveCalls.Load() != 0 {
		t.Errorf("expected 0 ReviewCollective calls for single expert, got %d", backend.collectiveCalls.Load())
	}
	if result.Verdict != VerdictPass {
		t.Errorf("expected pass, got %s", result.Verdict)
	}
}

func TestRunnerFallbackOnLargePrompt(t *testing.T) {
	backend := &MockBackend{
		Results: map[string]ExpertVerdict{
			"expert-a": {Expert: "expert-a", Verdict: VerdictPass, Confidence: 0.9},
			"expert-b": {Expert: "expert-b", Verdict: VerdictPass, Confidence: 0.9},
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Mode: ModeCollective, Timeout: 10},
	}

	// Create a submission large enough to exceed CollectiveThreshold
	largeContent := strings.Repeat("x", CollectiveThreshold)

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "expert-a", Name: "Expert A", Focus: "Testing", Body: "Expert A body."}},
		{Expert: &expert.Expert{ID: "expert-b", Name: "Expert B", Focus: "Testing", Body: "Expert B body."}},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: largeContent})

	if backend.calls.Load() != 2 {
		t.Errorf("expected 2 sequential Review calls for fallback, got %d", backend.calls.Load())
	}
	if backend.collectiveCalls.Load() != 0 {
		t.Errorf("expected 0 ReviewCollective calls for fallback, got %d", backend.collectiveCalls.Load())
	}
	if len(result.Perspectives) != 2 {
		t.Errorf("expected 2 perspectives, got %d", len(result.Perspectives))
	}
}

func TestRunnerCollectiveContextCancellation(t *testing.T) {
	backend := &MockBackend{
		Delay: 5 * time.Second,
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Mode: ModeCollective, Timeout: 1},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}},
		{Expert: &expert.Expert{ID: "bruce-schneier", Name: "Bruce Schneier", Focus: "Security"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := runner.Run(ctx, inputs, Submission{Content: "test diff"})

	// Collective fails due to timeout, falls back to sequential which also times out
	if len(result.Errors) == 0 {
		t.Error("expected errors from timeout, got none")
	}
}

func TestRunnerCollectiveErrorFallsBackToSequential(t *testing.T) {
	backend := &MockBackend{
		CollectiveErr: fmt.Errorf("API rate limited"),
		Results: map[string]ExpertVerdict{
			"kent-beck":      {Expert: "kent-beck", Verdict: VerdictPass, Confidence: 0.9},
			"bruce-schneier": {Expert: "bruce-schneier", Verdict: VerdictComment, Confidence: 0.8, Notes: []string{"Check auth"}},
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Mode: ModeCollective, Timeout: 10},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}},
		{Expert: &expert.Expert{ID: "bruce-schneier", Name: "Bruce Schneier", Focus: "Security"}},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	if backend.collectiveCalls.Load() != 1 {
		t.Errorf("expected 1 collective call, got %d", backend.collectiveCalls.Load())
	}
	if backend.calls.Load() != 2 {
		t.Errorf("expected 2 sequential fallback calls, got %d", backend.calls.Load())
	}
	if len(result.Perspectives) != 2 {
		t.Fatalf("expected 2 perspectives from fallback, got %d", len(result.Perspectives))
	}
	if result.Verdict != VerdictComment {
		t.Errorf("expected comment verdict from fallback, got %s", result.Verdict)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors after successful fallback, got %v", result.Errors)
	}
}

func TestRunnerSequentialPartialFailure(t *testing.T) {
	backend := &MockBackend{
		Results: map[string]ExpertVerdict{
			"kent-beck": {Expert: "kent-beck", Verdict: VerdictPass, Confidence: 0.9},
		},
		Errors: map[string]error{
			"bruce-schneier": fmt.Errorf("timeout"),
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Timeout: 10},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	if len(result.Perspectives) != 1 {
		t.Errorf("expected 1 perspective, got %d", len(result.Perspectives))
	}
}

func TestRunnerSequentialAllFail(t *testing.T) {
	backend := &MockBackend{
		Errors: map[string]error{
			"kent-beck": fmt.Errorf("timeout"),
		},
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Timeout: 10},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	if len(result.Perspectives) != 0 {
		t.Errorf("expected 0 perspectives, got %d", len(result.Perspectives))
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestRunnerSequentialContextCancellation(t *testing.T) {
	backend := &MockBackend{
		Delay: 5 * time.Second,
	}

	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Timeout: 1},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Kent Beck", Focus: "TDD"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := runner.Run(ctx, inputs, Submission{Content: "test diff"})

	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error from cancellation, got %d errors", len(result.Errors))
	}
}

func TestRunnerDefaultsToSequential(t *testing.T) {
	backend := &MockBackend{}
	runner := &Runner{Backend: backend, Options: ReviewOptions{Timeout: 10}}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Virtual Kent Beck", Focus: "TDD"}},
		{Expert: &expert.Expert{ID: "bruce-schneier", Name: "Virtual Bruce Schneier", Focus: "Security"}},
	}

	runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	if backend.calls.Load() != 2 {
		t.Errorf("expected 2 Review calls, got %d", backend.calls.Load())
	}
	if backend.collectiveCalls.Load() != 0 {
		t.Errorf("expected 0 ReviewCollective calls, got %d", backend.collectiveCalls.Load())
	}
}

func TestRunnerSequentialPassesPriorReviews(t *testing.T) {
	backend := &MockBackend{
		Results: map[string]ExpertVerdict{
			"dhh":       {Expert: "dhh", Verdict: VerdictBlock, Notes: []string{"Too many layers"}},
			"kent-beck": {Expert: "kent-beck", Verdict: VerdictComment, Replies: []Reply{{To: "dhh", Stance: StanceDisagree, Note: "The layers make it testable"}}},
		},
		Errors: map[string]error{"rob-pike": fmt.Errorf("timeout")},
	}
	runner := &Runner{Backend: backend, Options: ReviewOptions{Timeout: 10}}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "dhh", Name: "Virtual DHH"}},
		{Expert: &expert.Expert{ID: "rob-pike", Name: "Virtual Rob Pike"}},
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Virtual Kent Beck"}},
		{Expert: &expert.Expert{ID: "bruce-schneier", Name: "Virtual Bruce Schneier"}},
	}

	result := runner.Run(context.Background(), inputs, Submission{Content: "test diff"})

	// Order is preserved and each expert sees only successful earlier reviews.
	wantPrior := map[string][]string{
		"dhh":            nil,
		"rob-pike":       {"dhh"},
		"kent-beck":      {"dhh"},
		"bruce-schneier": {"dhh", "kent-beck"},
	}
	if len(backend.seen) != len(inputs) {
		t.Fatalf("expected %d calls, got %d", len(inputs), len(backend.seen))
	}
	for i, inp := range inputs {
		call := backend.seen[i]
		if call.expert != inp.Expert.ID {
			t.Errorf("call %d: expert = %s, want %s", i, call.expert, inp.Expert.ID)
		}
		var got []string
		for _, p := range call.sub.Prior {
			got = append(got, p.Expert)
		}
		if fmt.Sprint(got) != fmt.Sprint(wantPrior[inp.Expert.ID]) {
			t.Errorf("%s saw prior %v, want %v", inp.Expert.ID, got, wantPrior[inp.Expert.ID])
		}
	}

	if backend.seen[2].sub.Prior[0].Name != "Virtual DHH" {
		t.Errorf("prior review should carry the expert name, got %q", backend.seen[2].sub.Prior[0].Name)
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %v", result.Errors)
	}
	if !strings.Contains(result.Tension, "Virtual Kent Beck disagrees with Virtual DHH") {
		t.Errorf("tension should come from the disagree reply, got %q", result.Tension)
	}
}

func TestRunnerSequentialHooks(t *testing.T) {
	backend := &MockBackend{}
	var events []string
	runner := &Runner{
		Backend: backend,
		Options: ReviewOptions{Timeout: 10},
		OnStart: func(t Turn) {
			events = append(events, fmt.Sprintf("start %s %d/%d", t.Expert.ID, t.Number, t.Total))
		},
		OnVerdict: func(t Turn, verdicts []ExpertVerdict) {
			events = append(events, fmt.Sprintf("verdict %s (%d so far)", verdicts[len(verdicts)-1].Expert, len(verdicts)))
		},
	}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "dhh", Name: "Virtual DHH"}},
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Virtual Kent Beck"}},
	}
	runner.Run(context.Background(), inputs, Submission{Content: "diff"})

	want := []string{
		"start dhh 1/2",
		"verdict dhh (1 so far)",
		"start kent-beck 2/2",
		"verdict kent-beck (2 so far)",
	}
	if fmt.Sprint(events) != fmt.Sprint(want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}

func TestRunnerFinalWordAndModerator(t *testing.T) {
	backend := &MockBackend{
		Results: map[string]ExpertVerdict{
			"dhh":       {Verdict: VerdictBlock, Notes: []string{"Drop the interface"}},
			"kent-beck": {Verdict: VerdictComment, Notes: []string{"Keep it for tests"}, Replies: []Reply{{To: "dhh", Stance: StanceDisagree, Note: "Fakes need it"}}},
			"rob-pike":  {Verdict: VerdictBlock, Notes: []string{"Callers define interfaces"}},
			// The moderator is called with RawPrompt; its raw answer comes back in Notes[0].
			"moderator": {Notes: []string{`{"disagreements":[{"topic":"Keep the interface?","sides":[{"experts":["dhh","rob-pike"],"position":"Drop it"},{"experts":["kent-beck"],"position":"Keep it"}]}],"decisions":["Do you need a fake now?"]}`}},
		},
	}
	runner := &Runner{Backend: backend, Options: ReviewOptions{Timeout: 10, FinalWord: true, Moderate: true}}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "dhh", Name: "Virtual DHH"}},
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Virtual Kent Beck"}},
		{Expert: &expert.Expert{ID: "rob-pike", Name: "Virtual Rob Pike"}},
	}
	result := runner.Run(context.Background(), inputs, Submission{Content: "diff"})

	// 3 reviews + 2 final words (the last speaker has nobody after them) + 1 moderator.
	var calls []string
	for _, c := range backend.seen {
		kind := "review"
		if c.sub.Own != nil {
			kind = "final"
		} else if c.sub.RawPrompt != "" {
			kind = "moderator"
		}
		calls = append(calls, c.expert+":"+kind)
	}
	want := "[dhh:review kent-beck:review rob-pike:review dhh:final kent-beck:final moderator:moderator]"
	if fmt.Sprint(calls) != want {
		t.Errorf("calls = %v, want %s", calls, want)
	}

	// DHH's final word sees only the members who spoke after him.
	dhhFinal := backend.seen[3].sub
	if dhhFinal.Own == nil || dhhFinal.Own.Verdict != VerdictBlock || len(dhhFinal.Prior) != 2 {
		t.Errorf("DHH's final word should carry his review and the 2 later reviews, got own=%+v prior=%d", dhhFinal.Own, len(dhhFinal.Prior))
	}
	if !strings.Contains(BuildPrompt(inputs[0].Expert, dhhFinal), "## What Came After You") {
		t.Error("final-word prompt should show what came after the member")
	}

	if len(result.Disagreements) != 1 || len(result.Decisions) != 1 {
		t.Errorf("expected the moderator's disagreement and decision, got %+v / %+v", result.Disagreements, result.Decisions)
	}
	if strings.Contains(result.Summary, "Ship") {
		t.Errorf("summary should not recommend an outcome: %q", result.Summary)
	}
}

func TestRunnerFinalWordChangesVerdict(t *testing.T) {
	backend := &changingBackend{}
	runner := &Runner{Backend: backend, Options: ReviewOptions{Timeout: 10, FinalWord: true}}

	inputs := []ExpertInput{
		{Expert: &expert.Expert{ID: "dhh", Name: "Virtual DHH"}},
		{Expert: &expert.Expert{ID: "kent-beck", Name: "Virtual Kent Beck"}},
	}
	result := runner.Run(context.Background(), inputs, Submission{Content: "diff"})

	dhh := result.Perspectives[0]
	if dhh.Verdict != VerdictComment || dhh.ChangedFrom != VerdictBlock || dhh.ChangeReason != "Convinced" {
		t.Errorf("DHH should change block → comment with a reason, got %+v", dhh)
	}
	if len(dhh.FinalWord) != 1 || dhh.FinalWord[0].To != "kent-beck" {
		t.Errorf("DHH's final word should answer Kent Beck, got %+v", dhh.FinalWord)
	}
}

// changingBackend blocks on first reviews and softens to comment in final words.
type changingBackend struct{}

func (changingBackend) Review(_ context.Context, e *expert.Expert, sub Submission) (ExpertVerdict, error) {
	if sub.Own != nil {
		return ExpertVerdict{Verdict: VerdictComment, ChangeReason: "Convinced",
			Replies: []Reply{{To: "kent-beck", Stance: StanceAgree, Note: "Fair point"}}}, nil
	}
	return ExpertVerdict{Verdict: VerdictBlock, Notes: []string{e.ID + " note"}}, nil
}

func (changingBackend) ReviewCollective(context.Context, []*expert.Expert, Submission) (*SynthesizedResult, error) {
	return nil, fmt.Errorf("not used")
}
