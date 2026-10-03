package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// isInteractive returns true if stdin is a terminal (not piped).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Confirm asks user for confirmation with a y/n prompt
func Confirm(prompt string) bool {
	fmt.Print(prompt + " [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}

// openInEditor opens a file in the user's configured editor.
func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		if _, err := exec.LookPath("vim"); err == nil {
			editor = "vim"
		} else if _, err := exec.LookPath("nano"); err == nil {
			editor = "nano"
		} else if _, err := exec.LookPath("notepad"); err == nil {
			editor = "notepad"
		} else {
			return fmt.Errorf("no editor found: set $EDITOR environment variable")
		}
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// isTerminal reports whether f is a real terminal. Unlike isInteractive,
// it treats /dev/null as not a terminal, so scripted and AI-driven runs
// never wait for input.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(fi, null)
}
