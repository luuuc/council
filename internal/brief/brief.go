// Package brief holds the instructions Council gives the user's AI.
// Council ships no people: the AI proposes and builds members by
// following these briefs, and Council checks and stores the result.
package brief

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/pack"
)

//go:embed assemble.md
var assembleText string

var assembleTemplate = template.Must(template.New("assemble").Parse(assembleText))

// Assemble returns the brief for assembling a council, listing the
// current members and packs so the AI doesn't propose duplicates.
func Assemble() (string, error) {
	members, err := expert.List()
	if err != nil {
		return "", err
	}
	packs, err := pack.ListAll()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = assembleTemplate.Execute(&buf, struct {
		Members []*expert.Expert
		Packs   []*pack.Pack
	}{members, packs})
	return buf.String(), err
}
