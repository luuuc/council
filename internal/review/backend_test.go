package review

import (
	"context"
	"strings"
	"testing"
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
