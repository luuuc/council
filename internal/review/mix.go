package review

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/luuuc/council/internal/expert"
)

// MixBackend spreads council members across several AI backends, e.g.
// Claude, Codex, and a Kimi model through opencode. Models from different
// labs disagree more honestly than one model playing every member.
//
// Members are assigned round-robin in the order they first speak and keep
// their backend for later turns (their final word). The moderator, council
// spokespersons, and collective reviews use the first backend.
type MixBackend struct {
	Backends []Backend
	mu       sync.Mutex // councils run in parallel
	assigned map[string]int
	next     int
}

// NewMixBackend creates a MixBackend over the given backends.
func NewMixBackend(backends ...Backend) *MixBackend {
	return &MixBackend{Backends: backends, assigned: map[string]int{}}
}

// For returns the backend a member speaks through, assigning one on first use.
func (m *MixBackend) For(id string) Backend {
	if id == Moderator.ID || strings.HasSuffix(id, "-council") {
		return m.Backends[0]
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.assigned[id]
	if !ok {
		i = m.next % len(m.Backends)
		m.assigned[id] = i
		m.next++
	}
	return m.Backends[i]
}

// LabelFor names the backend a member speaks through, if it has a label.
func (m *MixBackend) LabelFor(id string) string {
	if l, ok := m.For(id).(interface{ Label() string }); ok {
		return l.Label()
	}
	return ""
}

// Review sends the member's review to their assigned backend.
func (m *MixBackend) Review(ctx context.Context, e *expert.Expert, sub Submission) (ExpertVerdict, error) {
	return m.For(e.ID).Review(ctx, e, sub)
}

// ReviewCollective uses the first backend: one call plays every member.
func (m *MixBackend) ReviewCollective(ctx context.Context, experts []*expert.Expert, sub Submission) (*SynthesizedResult, error) {
	return m.Backends[0].ReviewCollective(ctx, experts, sub)
}

// CLISpec is one AI CLI in a mix: the command and an optional model.
type CLISpec struct {
	Command string   `yaml:"command"`
	Model   string   `yaml:"model,omitempty"`
	Args    []string `yaml:"args,omitempty"`
}

// ParseMix reads a --mix value such as
// "claude,codex,opencode=kimi-code-plan-global/k3".
func ParseMix(spec string) []CLISpec {
	var out []CLISpec
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		command, model, _ := strings.Cut(part, "=")
		out = append(out, CLISpec{Command: strings.TrimSpace(command), Model: strings.TrimSpace(model)})
	}
	return out
}

// NewCLIMix builds a MixBackend from AI CLIs, checking each is installed.
func NewCLIMix(specs []CLISpec) (*MixBackend, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("mix needs at least one AI CLI")
	}
	var backends []Backend
	for _, s := range specs {
		if _, err := exec.LookPath(s.Command); err != nil {
			return nil, fmt.Errorf("mix: AI CLI %q not found", s.Command)
		}
		backends = append(backends, NewCLIBackend(s.Command, s.Args).WithModel(s.Model))
	}
	return NewMixBackend(backends...), nil
}
