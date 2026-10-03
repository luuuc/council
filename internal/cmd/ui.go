package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Confirm asks user for confirmation with a y/n prompt
func Confirm(prompt string) bool {
	fmt.Print(prompt + " [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}

// isTerminal reports whether f is a real terminal. Unlike a plain char-device check,
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
