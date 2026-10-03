package cmd

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/review"
	"gopkg.in/yaml.v3"
)

//go:embed prompts/interview.txt
var interviewPrompt string

// runAddInterview uses AI to generate an expert from a description
// and saves it to the project council (.council/experts/).
func runAddInterview() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Interview Mode")
	fmt.Println("==============")
	fmt.Println()
	fmt.Println("Tell me about someone whose feedback you value.")
	fmt.Println("This could be a mentor, tech lead, author, or even a historical figure.")
	fmt.Println("Describe how they think, what they prioritize, and how they give feedback.")
	fmt.Println()
	fmt.Println("(Enter your description, then press Enter twice to finish)")
	fmt.Println()

	// Collect multi-line description
	var lines []string
	emptyCount := 0
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			emptyCount++
			if emptyCount >= 1 || err != nil {
				break
			}
		} else {
			emptyCount = 0
			lines = append(lines, line)
		}
		if err != nil {
			break // EOF or other error
		}
	}

	if len(lines) == 0 {
		return fmt.Errorf("no description provided")
	}

	description := strings.Join(lines, "\n")

	fmt.Println()
	fmt.Println("Generating expert from your description...")
	fmt.Println()

	// Generate expert using AI
	generate := func() (*expert.Expert, error) {
		return generateExpert(fmt.Sprintf(interviewPrompt, description))
	}
	exp, err := generate()
	if err != nil {
		return fmt.Errorf("failed to generate expert: %w", err)
	}

	return reviewGeneratedExpert(reader, exp, generate)
}

// reviewGeneratedExpert previews an AI-generated expert and lets the user
// accept, edit, or regenerate it before saving to the project council.
func reviewGeneratedExpert(reader *bufio.Reader, exp *expert.Expert, regenerate func() (*expert.Expert, error)) error {
	displayExpertPreview(exp)

	for {
		fmt.Println()
		fmt.Print("Accept, Edit, or Regenerate? [a/e/r]: ")
		input, _ := reader.ReadString('\n')
		input = strings.ToLower(strings.TrimSpace(input))
		if len(input) > 1 {
			input = input[:1]
		}

		switch input {
		case "a", "":
			// Accept - prompt for ID and save
			fmt.Println()
			suggestedID := exp.ID
			if suggestedID == "" {
				suggestedID = expert.ToID(exp.Name)
			}
			fmt.Printf("ID: [%s] ", suggestedID)
			idInput, _ := reader.ReadString('\n')
			idInput = strings.TrimSpace(idInput)
			if idInput == "" {
				idInput = suggestedID
			}

			if expert.Exists(idInput) {
				return fmt.Errorf("expert '%s' already exists", idInput)
			}

			exp.ID = idInput
			if err := exp.Save(); err != nil {
				return err
			}

			fmt.Println()
			fmt.Printf("Created %s\n", exp.Name)
			fmt.Printf("File: %s\n", exp.Path())
			runAutoSync(addNoSync, nil)
			return nil

		case "e":
			edited, err := editExpert(exp)
			if err != nil {
				return err
			}
			if edited == nil {
				continue
			}
			exp = edited
			displayExpertPreview(exp)

		case "r":
			fmt.Println()
			fmt.Println("Regenerating...")
			fmt.Println()

			regenerated, err := regenerate()
			if err != nil {
				return fmt.Errorf("failed to regenerate: %w", err)
			}
			exp = regenerated
			displayExpertPreview(exp)

		default:
			fmt.Println("Invalid choice. Enter 'a' to accept, 'e' to edit, or 'r' to regenerate.")
		}
	}
}

// editExpert opens the expert in $EDITOR and returns the edited version.
// Returns nil (and no error) when the edited file doesn't parse, so the
// caller can let the user try again.
func editExpert(exp *expert.Expert) (*expert.Expert, error) {
	tmpfile, err := os.CreateTemp("", "council-expert-*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	content, err := formatExpertForEdit(exp)
	if err != nil {
		return nil, err
	}
	if _, err := tmpfile.WriteString(content); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	_ = tmpfile.Close()

	if err := openInEditor(tmpfile.Name()); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file: %w", err)
	}

	edited, err := expert.Parse(data)
	if err != nil {
		fmt.Printf("Error parsing edited file: %v\n", err)
		fmt.Println("Please fix the formatting and try again.")
		return nil, nil
	}
	return edited, nil
}

// aiPrompt sends a prompt to the AI and returns its answer.
// It is a variable so tests can replace the real AI CLI.
var aiPrompt = runAIPrompt

// generateExpert sends a persona-generation prompt to the headless AI CLI
// and parses the YAML expert it returns.
func generateExpert(prompt string) (*expert.Expert, error) {
	raw, err := aiPrompt(prompt)
	if err != nil {
		return nil, err
	}
	return parseGeneratedExpert(raw)
}

// runAIPrompt sends a prompt to the configured AI CLI in headless mode.
func runAIPrompt(prompt string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w\nHint: run 'council start' first", err)
	}

	// Detect or use configured AI command
	aiCmd, err := cfg.DetectAICommand()
	if err != nil {
		return "", err
	}
	if aiCmd == "" {
		return "", fmt.Errorf("persona generation needs an AI CLI (claude, opencode, or codex)")
	}
	if _, err := exec.LookPath(aiCmd); err != nil {
		return "", fmt.Errorf("AI command '%s' not found\n\nInstall it or configure a different command", aiCmd)
	}

	timeout := cfg.AI.Timeout
	if timeout == 0 {
		timeout = 60
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	raw, err := review.NewCLIBackend(aiCmd, cfg.AI.Args).Run(ctx, prompt)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("AI command timed out after %d seconds", timeout)
		}
		return "", fmt.Errorf("AI command failed: %w", err)
	}
	return raw, nil
}

// parseGeneratedExpert extracts the YAML expert from an AI response.
func parseGeneratedExpert(raw string) (*expert.Expert, error) {
	response := raw
	if idx := findYAMLStart(response); idx >= 0 {
		response = response[idx:]
	}

	exp, err := expert.Parse([]byte(response))
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w\n\nRaw response:\n%s", err, raw)
	}
	return exp, nil
}

// findYAMLStart finds the start of YAML frontmatter in a string.
func findYAMLStart(s string) int {
	// Look for --- at start of line
	for i := 0; i < len(s); i++ {
		if i == 0 || s[i-1] == '\n' {
			if i+3 <= len(s) && s[i:i+3] == "---" {
				return i
			}
		}
	}
	return -1
}

// displayExpertPreview shows a formatted preview of an expert.
func displayExpertPreview(e *expert.Expert) {
	fmt.Println("+---------------------------------------------------------+")
	fmt.Printf("| Name: %-49s |\n", truncate(e.Name, 49))
	fmt.Printf("| Focus: %-48s |\n", truncate(e.Focus, 48))
	fmt.Println("|                                                         |")

	if e.Philosophy != "" {
		fmt.Println("| Philosophy:                                             |")
		for _, line := range wrapText(e.Philosophy, 53) {
			fmt.Printf("|   %-54s |\n", line)
		}
	}

	if len(e.Principles) > 0 {
		fmt.Println("|                                                         |")
		fmt.Println("| Principles:                                             |")
		for _, pr := range e.Principles {
			fmt.Printf("|   - %-52s |\n", truncate(pr, 52))
		}
	}

	if len(e.RedFlags) > 0 {
		fmt.Println("|                                                         |")
		fmt.Println("| Red Flags:                                              |")
		for _, rf := range e.RedFlags {
			fmt.Printf("|   - %-52s |\n", truncate(rf, 52))
		}
	}

	fmt.Println("+---------------------------------------------------------+")
}

// formatExpertForEdit formats an expert's frontmatter for editing in a text editor.
func formatExpertForEdit(e *expert.Expert) (string, error) {
	fm, err := yaml.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("failed to format expert: %w", err)
	}
	return "---\n" + string(fm) + "---\n", nil
}

// truncate shortens a string to maxLen, adding "..." if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// wrapText wraps text to a given width.
func wrapText(s string, width int) []string {
	var lines []string
	words := strings.Fields(s) // Use standard library
	var current string

	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}
