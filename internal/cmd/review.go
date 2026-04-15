package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/luuuc/council-cli/internal/config"
	"github.com/luuuc/council-cli/internal/expert"
	"github.com/luuuc/council-cli/internal/pack"
	"github.com/luuuc/council-cli/internal/review"
	"github.com/spf13/cobra"
)

var (
	reviewPack        string
	reviewExpert      string
	reviewFile        string
	reviewJSON        bool
	reviewConcurrency int
)

func init() {
	rootCmd.AddCommand(reviewCmd)

	reviewCmd.Flags().StringVar(&reviewPack, "pack", "", "Review with a specific pack")
	reviewCmd.Flags().StringVar(&reviewExpert, "expert", "", "Review with a single expert")
	reviewCmd.Flags().StringVar(&reviewFile, "file", "", "File to review (reads diff from stdin if omitted)")
	reviewCmd.Flags().BoolVar(&reviewJSON, "json", false, "Output as JSON")
	reviewCmd.Flags().IntVar(&reviewConcurrency, "concurrency", 0, "Max parallel reviews (default: from config or 4)")
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run a blind council review",
	Long: `Run a council review with blind expert isolation.

Each expert reviews independently — no expert sees another's output.
Results are synthesized into a structured verdict with agreements,
tensions, and a recommendation.

Input can be a diff from stdin or a file via --file.

Examples:
  git diff main | council review --pack rails
  council review --pack code --file src/controller.rb
  council review --expert kent-beck --file lib/utils.rb
  git diff main | council review --pack rails --json
  git diff main | council review --pack rails --concurrency 2`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReview(cmd)
	},
}

func runReview(cmd *cobra.Command) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Detect AI command
	aiCmd, err := cfg.DetectAICommand()
	if err != nil {
		return fmt.Errorf("cannot run review: %w", err)
	}

	// Resolve experts
	inputs, packName, err := resolveReviewExperts()
	if err != nil {
		return err
	}

	if len(inputs) == 0 {
		return fmt.Errorf("no experts to review with — add experts or specify a --pack")
	}

	// Read submission
	sub, err := readSubmission()
	if err != nil {
		return err
	}

	// Build backend and runner
	backend := review.NewCLIBackend(aiCmd, cfg.AI.Args)

	concurrency := cfg.AI.Concurrency
	if reviewConcurrency > 0 {
		concurrency = reviewConcurrency
	}

	runner := &review.Runner{
		Backend: backend,
		Options: review.ReviewOptions{
			Concurrency: concurrency,
			Timeout:     cfg.AI.Timeout,
		},
	}

	// Progress message
	if packName != "" {
		fmt.Fprintf(os.Stderr, "Reviewing with %d experts (pack: %s)...\n", len(inputs), packName)
	} else {
		fmt.Fprintf(os.Stderr, "Reviewing with %d experts...\n", len(inputs))
	}

	// Run review
	result := runner.Run(cmd.Context(), inputs, sub)

	// Output
	if reviewJSON {
		data, err := review.FormatJSON(result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Print(review.FormatHuman(result, packName, len(inputs)))
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
		p, err := pack.Get(reviewPack)
		if err != nil {
			return nil, "", fmt.Errorf("pack '%s' not found: %w", reviewPack, err)
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
