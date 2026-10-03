package review

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

// Backend defines the interface for executing expert reviews.
type Backend interface {
	Review(ctx context.Context, e *expert.Expert, sub Submission) (ExpertVerdict, error)
	ReviewCollective(ctx context.Context, experts []*expert.Expert, sub Submission) (*SynthesizedResult, error)
}

// CLIBackend spawns subprocess calls to an AI CLI for reviews.
type CLIBackend struct {
	Command string
	Args    []string
}

// knownCLIDefaults returns the headless-mode args for known AI CLIs.
// The prompt is appended as the last argument.
func knownCLIDefaults(command string) []string {
	base := command
	// Handle full paths: /usr/local/bin/claude -> claude
	if idx := strings.LastIndex(command, "/"); idx >= 0 {
		base = command[idx+1:]
	}

	switch base {
	case "claude":
		return []string{"-p", "--output-format", "text"}
	case "opencode":
		return []string{"run"}
	case "codex":
		return []string{"exec", "--skip-git-repo-check", "--ephemeral", "--sandbox", "read-only", "--color", "never"}
	default:
		return nil
	}
}

// NewCLIBackend creates a CLIBackend with sensible defaults for the given command.
// If args is nil or empty, defaults are applied for known CLIs.
func NewCLIBackend(command string, args []string) *CLIBackend {
	if len(args) == 0 {
		args = knownCLIDefaults(command)
	}
	return &CLIBackend{
		Command: command,
		Args:    args,
	}
}

// Run sends a prompt to the CLI in headless mode and returns its answer.
// The answer is read from stdout; stderr is used only when stdout is empty,
// since CLIs like codex and opencode write progress logs to stderr.
func (b *CLIBackend) Run(ctx context.Context, prompt string) (string, error) {
	args := make([]string, len(b.Args))
	copy(args, b.Args)

	cmd := exec.CommandContext(ctx, b.Command, args...)

	// Use stdin for large prompts to avoid ARG_MAX limits (~256KB on most systems).
	// Threshold set conservatively below typical limits.
	const argMaxSafe = 128 * 1024
	if len(prompt) > argMaxSafe {
		cmd.Stdin = strings.NewReader(prompt)
	} else {
		cmd.Args = append(cmd.Args, prompt)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := stderr.Bytes()
		if len(detail) == 0 {
			detail = stdout.Bytes()
		}
		if len(detail) > 0 {
			return "", fmt.Errorf("%s failed: %s: %w", b.Command, truncateBytes(bytes.TrimSpace(detail), 200), err)
		}
		return "", fmt.Errorf("%s failed: %w", b.Command, err)
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = strings.TrimSpace(stderr.String())
	}
	return out, nil
}

// Review executes a single expert review via subprocess.
func (b *CLIBackend) Review(ctx context.Context, e *expert.Expert, sub Submission) (ExpertVerdict, error) {
	prompt := sub.RawPrompt
	if prompt == "" {
		prompt = BuildPrompt(e, sub)
	}

	output, err := b.Run(ctx, prompt)
	if err != nil {
		return ExpertVerdict{}, fmt.Errorf("review by %s: %w", e.ID, err)
	}

	// RawPrompt mode: return the raw text directly instead of parsing verdict JSON.
	if sub.RawPrompt != "" {
		return ExpertVerdict{
			Expert:     e.ID,
			Verdict:    VerdictComment,
			Confidence: 1.0,
			Notes:      []string{output},
		}, nil
	}

	return ParseVerdict(e.ID, []byte(output)), nil
}

// ReviewCollective executes a collective review with all experts via subprocess.
func (b *CLIBackend) ReviewCollective(ctx context.Context, experts []*expert.Expert, sub Submission) (*SynthesizedResult, error) {
	output, err := b.Run(ctx, BuildCollectivePrompt(experts, sub))
	if err != nil {
		return nil, fmt.Errorf("collective review: %w", err)
	}

	expertIDs := make([]string, len(experts))
	for i, e := range experts {
		expertIDs[i] = e.ID
	}

	return ParseCollectiveResult([]byte(output), expertIDs), nil
}

// truncateBytes returns a string of at most maxLen bytes from b.
func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}
