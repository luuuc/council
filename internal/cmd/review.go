package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/pack"
	"github.com/luuuc/council/internal/review"
	"github.com/spf13/cobra"
)

var (
	reviewPack     string
	reviewExpert   string
	reviewFile     string
	reviewJSON     bool
	reviewOutput   string
	reviewBackend  string
	reviewProvider string
	reviewModel    string
	reviewMode     string
	reviewQuick    bool
	reviewCouncils string
)

func init() {
	rootCmd.AddCommand(reviewCmd)

	reviewCmd.Flags().StringVar(&reviewPack, "pack", "", "Review with a specific pack")
	reviewCmd.Flags().StringVar(&reviewExpert, "expert", "", "Review with a single expert")
	reviewCmd.Flags().StringVar(&reviewFile, "file", "", "File to review (reads diff from stdin if omitted)")
	reviewCmd.Flags().BoolVar(&reviewJSON, "json", false, "Output as JSON")
	reviewCmd.Flags().StringVar(&reviewOutput, "output", "", "Output format: github-pr (implies --json)")
	reviewCmd.Flags().StringVar(&reviewBackend, "backend", "", "Backend: cli or api")
	reviewCmd.Flags().StringVar(&reviewProvider, "provider", "", "API provider: anthropic, openai, ollama, github")
	reviewCmd.Flags().StringVar(&reviewModel, "model", "", "LLM model override")
	reviewCmd.Flags().StringVar(&reviewCouncils, "councils", "", "Several packs that each review, then challenge each other's conclusions (e.g. product,security,code)")
	reviewCmd.Flags().BoolVar(&reviewQuick, "quick", false, "Sequential mode: skip the final word and the moderator (about half the AI calls)")
	reviewCmd.Flags().StringVar(&reviewMode, "mode", string(review.ModeSequential), "Review mode: sequential (one call per expert, each reacts to the others) or collective (one call, cheaper)")
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run a council review where experts react to each other",
	Long: `Run a council review. Experts speak one at a time, in pack order.
Each one sees the earlier reviews and can disagree, back them up, or add
what they missed. Then the earlier members get a final word on what came
after them, and may change their verdict. A neutral moderator closes with
where the members disagree and what you need to decide. Council doesn't
decide for you.

--quick skips the final word and the moderator (about half the AI calls).

--councils runs several packs on the same submission. Each council debates
on its own, then each council's spokesperson answers the other councils'
conclusions, and a moderator lists where the councils disagree and what you
need to decide.

--mode collective runs one call that plays every expert at once. It is
cheaper (one call instead of one per expert) but the debate is simulated.

Input can be a diff from stdin or a file via --file.

Note: per-file review (--provider github) reviews each file in isolation.
Cross-file issues (e.g. function defined in A, misused in B) are invisible.
Use BYOK (--provider anthropic/openai) for cross-file analysis.

Examples:
  git diff main | council review --pack rails
  council review --pack code --file src/controller.rb
  council review --expert kent-beck --file lib/utils.rb
  git diff main | council review --pack rails --json
  git diff main | council review --pack go --mode collective
  council review --councils product,security,code --file plan.md
  git diff main | council review --backend api --provider github --output github-pr`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReview(cmd)
	},
}

func runReview(cmd *cobra.Command) error {
	mode := review.Mode(reviewMode)
	if mode != review.ModeSequential && mode != review.ModeCollective {
		return fmt.Errorf("invalid --mode %q: use sequential or collective", reviewMode)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Resolve experts: one council (--pack, --expert, or the project's), or several (--councils)
	var councils []review.Council
	var inputs []review.ExpertInput
	var packName string
	if reviewCouncils != "" {
		if reviewPack != "" || reviewExpert != "" || reviewOutput != "" {
			return fmt.Errorf("--councils can't be combined with --pack, --expert, or --output")
		}
		if councils, err = resolveCouncils(reviewCouncils); err != nil {
			return err
		}
	} else {
		if inputs, packName, err = resolveReviewExperts(); err != nil {
			return err
		}
		if len(inputs) == 0 {
			return fmt.Errorf("no experts to review with — add experts or specify a --pack")
		}
	}

	// Read submission
	sub, err := readSubmission()
	if err != nil {
		return err
	}

	// Build backend
	backend, err := buildBackend(cfg)
	if err != nil {
		return fmt.Errorf("cannot run review: %w", err)
	}

	runner := &review.Runner{
		Backend: backend,
		Options: review.ReviewOptions{
			Mode:      mode,
			Timeout:   cfg.AI.Timeout,
			FinalWord: !reviewQuick,
			Moderate:  !reviewQuick,
		},
	}

	// Progress goes to stderr so JSON output on stdout stays clean.
	runner.OnStart = func(t review.Turn) {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s...\n", t.Number, t.Total, t.Label())
	}

	if councils != nil {
		return runCouncilsReview(cmd, runner, councils, sub)
	}

	// Human output streams each expert as soon as they finish.
	human := reviewOutput != "github-pr" && !reviewJSON
	streamed := 0
	if human {
		fmt.Print(review.FormatHeader(packName, len(inputs)))
		runner.OnVerdict = func(t review.Turn, verdicts []review.ExpertVerdict) {
			if t.Kind == review.TurnFinalWord {
				fmt.Print(review.FormatFinalWord(t.Expert.ID, verdicts))
			} else {
				fmt.Print(review.FormatPerspective(verdicts))
			}
			streamed++
		}
	}

	if mode == review.ModeCollective {
		fmt.Fprintf(os.Stderr, "Reviewing with %d experts in one call...\n", len(inputs))
	}

	// Run review
	result := runner.Run(cmd.Context(), inputs, sub)

	// Output
	if reviewOutput == "github-pr" {
		var dp *review.DiffPosition
		if sub.Content != "" {
			dp = review.NewDiffPosition(sub.Content)
		}
		output := review.FormatGitHubReview(result, packName, len(inputs), dp)
		data, err := review.FormatGitHubJSON(output)
		if err != nil {
			return fmt.Errorf("failed to marshal github review: %w", err)
		}
		fmt.Println(string(data))
	} else if reviewJSON {
		data, err := review.FormatJSON(result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		fmt.Println(string(data))
	} else {
		// Collective mode returns everything at once: print what wasn't streamed.
		if streamed == 0 {
			for i := range result.Perspectives {
				fmt.Print(review.FormatPerspective(result.Perspectives[:i+1]))
			}
		}
		fmt.Print(review.FormatOutcome(result))
	}

	return nil
}

// resolveReviewExperts determines which experts to use based on flags.
func resolveReviewExperts() ([]review.ExpertInput, string, error) {
	// --expert: single expert
	if reviewExpert != "" {
		e, err := expert.Load(reviewExpert)
		if err != nil {
			return nil, "", fmt.Errorf("expert '%s' not found: %w", reviewExpert, err)
		}
		return []review.ExpertInput{{Expert: e, Blocking: false}}, "", nil
	}

	// --pack: resolve pack members
	if reviewPack != "" {
		return resolvePackInputs(reviewPack)
	}

	// Default: all council experts
	experts, err := expert.List()
	if err != nil {
		return nil, "", fmt.Errorf("failed to list experts: %w", err)
	}

	inputs := make([]review.ExpertInput, len(experts))
	for i, e := range experts {
		inputs[i] = review.ExpertInput{Expert: e, Blocking: false}
	}
	return inputs, "", nil
}

// readSubmission reads the review content from --file or stdin.
func readSubmission() (review.Submission, error) {
	if reviewFile != "" {
		data, err := os.ReadFile(reviewFile)
		if err != nil {
			return review.Submission{}, fmt.Errorf("failed to read file: %w", err)
		}
		return review.Submission{
			Content: string(data),
			Context: fmt.Sprintf("File: %s", reviewFile),
		}, nil
	}

	// Read from stdin
	info, _ := os.Stdin.Stat()
	if info.Mode()&os.ModeCharDevice != 0 {
		return review.Submission{}, fmt.Errorf("no input: pipe a diff or use --file\n\nExamples:\n  git diff main | council review --pack rails\n  council review --pack rails --file src/main.go")
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return review.Submission{}, fmt.Errorf("failed to read stdin: %w", err)
	}

	content := string(data)
	if content == "" {
		return review.Submission{}, fmt.Errorf("empty input from stdin")
	}

	return review.Submission{Content: content}, nil
}

// buildBackend creates the appropriate review backend based on config and environment.
// CLI flags (--backend, --provider, --model) override config values.
func buildBackend(cfg *config.Config) (review.Backend, error) {
	// Copy config to avoid mutating the caller's struct
	overridden := *cfg
	if reviewBackend != "" {
		overridden.AI.Backend = reviewBackend
	}
	if reviewProvider != "" {
		overridden.AI.Provider = reviewProvider
	}
	if reviewModel != "" {
		overridden.AI.Model = reviewModel
	}

	backend, provider, model := overridden.DetectBackend()

	switch backend {
	case "api":
		if provider == "" {
			return nil, fmt.Errorf("api backend requires a provider (anthropic, openai, ollama, github)")
		}
		return review.NewAPIBackend(provider, model)
	case "cli":
		aiCmd, err := cfg.DetectAICommand()
		if err != nil {
			return nil, err
		}
		return review.NewCLIBackend(aiCmd, cfg.AI.Args), nil
	default:
		return nil, fmt.Errorf("no backend available\n\nInstall an AI CLI (claude, opencode, codex) or set an API key (ANTHROPIC_API_KEY, OPENAI_API_KEY, GITHUB_TOKEN)")
	}
}

// resolvePackInputs resolves a pack's members into review inputs, in pack order.
func resolvePackInputs(name string) ([]review.ExpertInput, string, error) {
	p, err := pack.Get(name)
	if err != nil {
		return nil, "", fmt.Errorf("pack '%s' not found: %w", name, err)
	}

	available, err := expert.List()
	if err != nil {
		return nil, "", fmt.Errorf("failed to list experts: %w", err)
	}

	resolved, warnings := pack.Resolve(p, available)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	inputs := make([]review.ExpertInput, len(resolved))
	for i, rm := range resolved {
		inputs[i] = review.ExpertInput{
			Expert:   rm.Expert,
			Blocking: rm.Blocking,
		}
	}
	return inputs, p.Name, nil
}

// resolveCouncils resolves --councils into named councils, in the given order.
func resolveCouncils(list string) ([]review.Council, error) {
	var councils []review.Council
	seen := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		inputs, packName, err := resolvePackInputs(name)
		if err != nil {
			return nil, err
		}
		if len(inputs) == 0 {
			return nil, fmt.Errorf("pack '%s' has no members", name)
		}
		councils = append(councils, review.Council{Name: packName, Inputs: inputs})
	}
	if len(councils) < 2 {
		return nil, fmt.Errorf("--councils needs at least two packs, e.g. --councils product,security,code")
	}
	return councils, nil
}

// runCouncilsReview runs several councils on the same submission, then has
// them answer each other. Human output streams as it goes.
func runCouncilsReview(cmd *cobra.Command, runner *review.Runner, councils []review.Council, sub review.Submission) error {
	human := !reviewJSON
	streamed := 0
	if human {
		runner.OnVerdict = func(t review.Turn, verdicts []review.ExpertVerdict) {
			if t.Kind == review.TurnFinalWord {
				fmt.Print(review.FormatFinalWord(t.Expert.ID, verdicts))
			} else {
				fmt.Print(review.FormatPerspective(verdicts))
			}
			streamed++
		}
	}

	debateStarted := false
	hooks := review.CouncilHooks{
		OnCouncilStart: func(name string, members int) {
			fmt.Fprintf(os.Stderr, "The %s council (%d members)...\n", name, members)
			if human {
				fmt.Print(review.FormatCouncilHeader(name, members))
			}
			streamed = 0
		},
		OnCouncilDone: func(name string, result *review.SynthesizedResult) {
			if !human {
				return
			}
			if streamed == 0 {
				for i := range result.Perspectives {
					fmt.Print(review.FormatPerspective(result.Perspectives[:i+1]))
				}
			}
			fmt.Print(review.FormatOutcome(result))
		},
		OnCrossStart: func(label string) {
			fmt.Fprintf(os.Stderr, "%s...\n", label)
		},
		OnStatement: func(st review.CouncilStatement) {
			if !human {
				return
			}
			if !debateStarted {
				fmt.Print(review.FormatCouncilsDebateHeader())
				debateStarted = true
			}
			fmt.Print(review.FormatStatement(st))
		},
	}

	result := runner.RunCouncils(cmd.Context(), councils, sub, hooks)

	if reviewJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(review.FormatCouncilsOutcome(result))
	return nil
}
