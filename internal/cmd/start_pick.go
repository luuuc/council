package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

// maxChallengers caps how many unchecked extra voices the picker offers.
// maxDissenters keeps room in that list for differentAngles.
const (
	maxChallengers = 6
	maxDissenters  = 4
)

// differentAngles are voices with incentives a stack-based council usually
// lacks: product scope, product discovery, security, and operations.
var differentAngles = []struct {
	id, why string
}{
	{"jason-fried", "product: cuts scope"},
	{"marty-cagan", "product: is it worth building?"},
	{"teresa-torres", "customers: what do users need?"},
	{"bruce-schneier", "security"},
	{"gene-kim", "operations"},
}

// councilCandidate is one row in the council picker.
type councilCandidate struct {
	Expert  *expert.Expert
	Why     string // why it's offered; empty for stack picks
	Checked bool
}

// councilCandidates returns the stack picks (checked) followed by unchecked
// challengers: people the picks have documented disagreements with, then
// voices with different incentives. A council of similar experts agrees too
// easily, so the picker makes the dissenting options visible.
func councilCandidates(picks []*expert.Expert) []councilCandidate {
	seen := map[string]bool{}
	var out []councilCandidate
	for _, e := range picks {
		seen[e.ID] = true
		out = append(out, councilCandidate{Expert: e, Checked: true})
	}

	add := func(id, why string, limit int) {
		if seen[id] || len(out)-len(picks) >= limit {
			return
		}
		e := findExpertByID(id)
		if e == nil {
			return
		}
		seen[id] = true
		out = append(out, councilCandidate{Expert: e, Why: why})
	}

	for _, p := range picks {
		for _, t := range p.Tensions {
			add(t.Expert, fmt.Sprintf("disagrees with %s on %s", p.Name, t.Topic), maxDissenters)
		}
	}
	for _, a := range differentAngles {
		add(a.id, a.why, maxChallengers)
	}
	return out
}

// pickCouncil lets the user check and uncheck members before saving.
// It's a plain numbered list read line by line, so it works in any terminal.
func pickCouncil(in io.Reader, out io.Writer, picks []*expert.Expert) ([]*expert.Expert, error) {
	candidates := councilCandidates(picks)
	reader := bufio.NewReader(in)

	for {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "Pick your council. Checked members match your stack; the others are there to disagree.")
		_, _ = fmt.Fprintln(out)
		for i, c := range candidates {
			box := "[ ]"
			if c.Checked {
				box = "[x]"
			}
			detail := c.Expert.Focus
			if c.Why != "" {
				detail = c.Why
			}
			_, _ = fmt.Fprintf(out, "  %2d. %s %s — %s\n", i+1, box, c.Expert.Name, detail)
		}
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprint(out, "Type numbers to check or uncheck (e.g. \"7 9\"), or press Enter to confirm: ")

		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line == "" {
			var selected []*expert.Expert
			for _, c := range candidates {
				if c.Checked {
					selected = append(selected, c.Expert)
				}
			}
			if len(selected) > 0 {
				return selected, nil
			}
			if err != nil {
				return nil, fmt.Errorf("no members picked")
			}
			_, _ = fmt.Fprintln(out, "Pick at least one member.")
			continue
		}

		for _, field := range strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == ',' }) {
			n, convErr := strconv.Atoi(field)
			if convErr != nil || n < 1 || n > len(candidates) {
				_, _ = fmt.Fprintf(out, "Ignoring %q: not a number from 1 to %d.\n", field, len(candidates))
				continue
			}
			candidates[n-1].Checked = !candidates[n-1].Checked
		}
		if err != nil {
			return nil, fmt.Errorf("input ended before confirming")
		}
	}
}

// isTerminal reports whether f is a real terminal. Unlike isInteractive,
// it treats /dev/null as not a terminal, so scripted runs stay zero-config.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(fi, null)
}
