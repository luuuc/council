package review

import (
	"context"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestKnownCLIDefaults(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"claude", "-p --output-format text"},
		{"/usr/local/bin/claude", "-p --output-format text"},
		{"opencode", "run"},
		{"codex", "exec --skip-git-repo-check --ephemeral --sandbox read-only --color never"},
		{"llm", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := strings.Join(knownCLIDefaults(tt.command), " ")
			if got != tt.want {
				t.Errorf("knownCLIDefaults(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestCLIBackendRun(t *testing.T) {
	// sh -c '<script>' sh <prompt>: the prompt arrives as $1.
	tests := []struct {
		name    string
		script  string
		want    string
		wantErr string
	}{
		{"answer on stdout, logs on stderr", `echo "log line" >&2; echo "answer to $1"`, "answer to hi", ""},
		{"answer only on stderr", `echo "answer on stderr" >&2`, "answer on stderr", ""},
		{"failure includes stderr", `echo "bad key" >&2; exit 3`, "", "bad key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewCLIBackend("sh", []string{"-c", tt.script, "sh"})
			got, err := b.Run(context.Background(), "hi")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Run() = %q, want %q", got, tt.want)
			}
		})
	}
}

type namedBackend struct{ name string }

func (n namedBackend) Label() string { return n.name }
func (n namedBackend) Review(_ context.Context, e *expert.Expert, _ Submission) (ExpertVerdict, error) {
	return ExpertVerdict{Expert: e.ID, Verdict: VerdictPass, Notes: []string{n.name}}, nil
}
func (n namedBackend) ReviewCollective(context.Context, []*expert.Expert, Submission) (*SynthesizedResult, error) {
	return &SynthesizedResult{Summary: n.name}, nil
}

func TestMixBackendAssignsMembersRoundRobin(t *testing.T) {
	m := NewMixBackend(namedBackend{"claude"}, namedBackend{"codex"}, namedBackend{"opencode"})

	order := []string{"dhh", "rob-pike", "boris-cherny", "kent-beck", "dhh", "moderator", "product-council"}
	var got []string
	for _, id := range order {
		got = append(got, m.LabelFor(id))
	}
	want := "claude codex opencode claude claude claude claude"
	if strings.Join(got, " ") != want {
		t.Errorf("assignments = %v, want %s", got, want)
	}

	v, _ := m.Review(context.Background(), &expert.Expert{ID: "rob-pike"}, Submission{})
	if v.Notes[0] != "codex" {
		t.Errorf("rob-pike should keep speaking through codex, got %s", v.Notes[0])
	}
}

func TestCLIBackendModelArgs(t *testing.T) {
	tests := []struct{ command, model, want string }{
		{"claude", "opus", "--model opus"},
		{"codex", "gpt-5", "-m gpt-5"},
		{"opencode", "kimi-code-plan-global/k3", "-m kimi-code-plan-global/k3"},
		{"aichat", "x", ""},
		{"claude", "", ""},
	}
	for _, tt := range tests {
		if got := strings.Join(modelArgs(tt.command, tt.model), " "); got != tt.want {
			t.Errorf("modelArgs(%q, %q) = %q, want %q", tt.command, tt.model, got, tt.want)
		}
	}

	// The model flag goes after the headless args and before the prompt.
	b := NewCLIBackend("sh", []string{"-c", `echo "$@"`, "sh"}).WithModel("ignored-for-sh")
	if out, err := b.Run(context.Background(), "hi"); err != nil || out != "hi" {
		t.Errorf("Run() = %q, %v", out, err)
	}
	if NewCLIBackend("opencode", nil).WithModel("ollama-cloud/deepseek-v4-pro").Label() != "opencode ollama-cloud/deepseek-v4-pro" {
		t.Error("label should name the CLI and model")
	}
}
