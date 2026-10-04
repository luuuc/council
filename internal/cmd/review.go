package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/pack"
	"github.com/luuuc/council/internal/review"
	"github.com/spf13/cobra"
)

var (
	reviewPack     string
	reviewCouncils string
	reviewFile     string
	reviewRecord   string
	reviewAPI      bool
	reviewProvider string
	reviewModel    string
	reviewJSON     bool
	reviewOutput   string
)

// apiTimeout bounds the one model API call of an unattended review.
const apiTimeout = 10 * time.Minute

func init() {
	rootCmd.AddCommand(reviewCmd)

	reviewCmd.Flags().StringVar(&reviewPack, "pack", "", "Review with one pack (default: every member)")
	reviewCmd.Flags().StringVar(&reviewCouncils, "councils", "", "Several packs that each debate, then answer each other (e.g. product,security)")
	reviewCmd.Flags().StringVar(&reviewFile, "file", "", "File to review (reads stdin if omitted)")
	reviewCmd.Flags().StringVar(&reviewRecord, "record", "", "Check the AI's answer to the room prompt (a file, or - for stdin), show it, and save it")
	reviewCmd.Flags().BoolVar(&reviewAPI, "api", false, "Unattended: send the room prompt to a model API with your key, then show the review")
	reviewCmd.Flags().StringVar(&reviewProvider, "provider", "", "With --api: anthropic, openai, ollama, or github (default: ai.provider, else the first API key found)")
	reviewCmd.Flags().StringVar(&reviewModel, "model", "", "With --api: the model (default: ai.model, else the provider's default)")
	reviewCmd.Flags().BoolVar(&reviewJSON, "json", false, "With --record or --api: print the review as JSON")
	reviewCmd.Flags().StringVar(&reviewOutput, "output", "", "With --api: github-pr prints a GitHub PR review payload (one pack)")
}

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Get the room prompt for a council review, then record the debate",
	Long: `A council review is one room: every member, one prompt, answered in one
pass. Members speak in order and react to each other (agree, disagree,
adds), earlier members get a final word, and a neutral moderator lists
where they disagree and what you need to decide. Council doesn't decide.

In an AI tool (/council does this for you):

  1. council review [--pack p | --councils a,b] [--file f]
     prints the room prompt. The AI answers it with the whole debate as JSON.
  2. council review [--pack p | --councils a,b] --record <file|->
     checks the answer, shows the review, and saves it in .council/reviews/.
     If something is wrong, it says what to fix so the AI can answer again.

Unattended (CI, the GitHub Action):

  council review --api [--provider p] [--model m] [--pack p] [--file f]
     sends the room prompt to a model API with your key and shows the review.
     --output github-pr prints a GitHub PR review payload instead.

Input is a diff on stdin or a file via --file.

Examples:
  git diff main | council review --pack code
  council review --councils product,security --file plan.md
  council review --pack code --record answer.json
  git diff main | council review --api --provider anthropic --pack code`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runReview(cmd.Context())
	},
}

func runReview(ctx context.Context) error {
	if reviewRecord != "" && reviewAPI {
		return fmt.Errorf("use --record or --api, not both")
	}
	if reviewOutput != "" && reviewOutput != "github-pr" {
		return fmt.Errorf("unknown --output %q: the only one is github-pr", reviewOutput)
	}
	if reviewOutput != "" && (!reviewAPI || reviewCouncils != "") {
		return fmt.Errorf("--output github-pr works with --api and one pack")
	}

	councils, err := resolveReviewCouncils()
	if err != nil {
		return err
	}

	if reviewRecord != "" {
		answer, err := readInput(reviewRecord)
		if err != nil {
			return err
		}
		result, err := review.ParseRoom(answer, councils)
		if err != nil {
			return fmt.Errorf("%w\n\n(Record with the same --pack or --councils as the prompt.)", err)
		}
		return showReview(result, councils, review.Submission{})
	}

	sub, err := readSubmission()
	if err != nil {
		return err
	}
	prompt := review.BuildRoomPrompt(councils, sub)

	if !reviewAPI {
		fmt.Print(prompt)
		return nil
	}

	result, err := askAPI(ctx, prompt, councils)
	if err != nil {
		return err
	}
	return showReview(result, councils, sub)
}

// askAPI sends the room prompt to the model API. If the answer doesn't
// check out, it asks once more with what to fix.
func askAPI(ctx context.Context, prompt string, councils []review.Council) (*review.CouncilsResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if reviewProvider != "" {
		cfg.AI.Provider = reviewProvider
	}
	if reviewModel != "" {
		cfg.AI.Model = reviewModel
	}
	provider, model, err := cfg.DetectProvider()
	if err != nil {
		return nil, err
	}
	backend, err := review.NewAPIBackend(provider, model)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "The council is debating (%s %s)...\n", provider, model)
	answer, err := backend.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}
	result, err := review.ParseRoom(answer, councils)
	if err == nil {
		return result, nil
	}

	fmt.Fprintln(os.Stderr, "The answer needs fixing; asking again...")
	retry := prompt + "\n\n## Your previous answer\n\n" + answer + "\n\n## Fix it\n\n" + err.Error()
	if answer, err = backend.Complete(ctx, retry); err != nil {
		return nil, err
	}
	return review.ParseRoom(answer, councils)
}

// showReview prints a checked review and saves it in .council/reviews/.
func showReview(result *review.CouncilsResult, councils []review.Council, sub review.Submission) error {
	text := review.FormatRoom(result)
	path, saveErr := review.Save(text, councilsLabel(councils), time.Now())

	switch {
	case reviewOutput == "github-pr":
		c := result.Councils[0]
		var dp *review.DiffPosition
		if sub.Content != "" {
			dp = review.NewDiffPosition(sub.Content)
		}
		data, err := review.FormatGitHubJSON(review.FormatGitHubReview(c.Result, c.Name, len(c.Result.Perspectives), dp))
		if err != nil {
			return fmt.Errorf("marshal github review: %w", err)
		}
		fmt.Println(string(data))
	case reviewJSON:
		var v any = result
		if len(result.Councils) == 1 {
			v = result.Councils[0].Result
		}
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal review: %w", err)
		}
		fmt.Println(string(data))
	default:
		fmt.Print(text)
	}

	if saveErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", saveErr)
	} else {
		fmt.Fprintf(os.Stderr, "Saved to %s\n", path)
	}
	return nil
}

// resolveReviewCouncils returns the councils in the room: several
// (--councils), one pack (--pack), or every member.
func resolveReviewCouncils() ([]review.Council, error) {
	if reviewCouncils != "" {
		if reviewPack != "" {
			return nil, fmt.Errorf("use --pack or --councils, not both")
		}
		return resolveCouncils(reviewCouncils)
	}
	if reviewPack != "" {
		inputs, name, err := resolvePackInputs(reviewPack)
		if err != nil {
			return nil, err
		}
		if len(inputs) == 0 {
			return nil, fmt.Errorf("pack '%s' has no members", reviewPack)
		}
		return []review.Council{{Name: name, Inputs: inputs}}, nil
	}

	experts, err := expert.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list experts: %w", err)
	}
	if len(experts) == 0 {
		return nil, fmt.Errorf("the council has no members yet: run /council-assemble in your AI tool")
	}
	inputs := make([]review.ExpertInput, len(experts))
	for i, e := range experts {
		inputs[i] = review.ExpertInput{Expert: e}
	}
	return []review.Council{{Inputs: inputs}}, nil
}

func councilsLabel(councils []review.Council) string {
	names := make([]string, len(councils))
	for i, c := range councils {
		names[i] = c.Name
	}
	return strings.Join(names, "-")
}

// readInput reads a file, or stdin for "-".
func readInput(path string) (string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", fmt.Errorf("read the answer: %w", err)
	}
	return string(data), nil
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

	info, _ := os.Stdin.Stat()
	if info.Mode()&os.ModeCharDevice != 0 {
		return review.Submission{}, fmt.Errorf("no input: pipe a diff or use --file\n\nExamples:\n  git diff main | council review --pack code\n  council review --pack code --file src/main.go")
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return review.Submission{}, fmt.Errorf("failed to read stdin: %w", err)
	}
	if len(data) == 0 {
		return review.Submission{}, fmt.Errorf("empty input from stdin")
	}
	return review.Submission{Content: string(data)}, nil
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
		return nil, fmt.Errorf("--councils needs at least two packs, e.g. --councils product,security")
	}
	return councils, nil
}
