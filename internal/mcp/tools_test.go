package mcp

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoomThenRecord(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()
	s := NewServer(strings.NewReader(""), io.Discard, "test")

	room := s.handleRoom(map[string]any{"pack": "go", "content": "func main() {}", "context": "a CLI"})
	if room.IsError {
		t.Fatalf("council_room failed: %s", room.Content[0].Text)
	}
	prompt := room.Content[0].Text
	for _, want := range []string{"func main() {}", "a CLI", "(id: cleo)", "(id: ada)", `Call council_record with pack "go"`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("room prompt is missing %q", want)
		}
	}
	if strings.Index(prompt, "(id: cleo)") > strings.Index(prompt, "(id: ada)") {
		t.Error("members should speak in pack order (cleo, then ada)")
	}

	bad := s.handleRecord(map[string]any{"pack": "go", "answer": `{"reviews":[{"expert":"cleo","verdict":"pass","notes":["ok"]}]}`})
	if !bad.IsError || !strings.Contains(bad.Content[0].Text, "no review from ada") || !strings.Contains(bad.Content[0].Text, "Call council_record again") {
		t.Errorf("an incomplete answer should say what to fix: %+v", bad)
	}

	good := s.handleRecord(map[string]any{"pack": "go", "answer": `{
		"reviews":[
			{"expert":"cleo","verdict":"pass","notes":["clear"]},
			{"expert":"ada","verdict":"block","notes":["no tests"],"replies":[{"to":"cleo","stance":"disagree","note":"clear isn't tested"}]}],
		"disagreements":[{"topic":"tests first?","sides":[{"experts":["cleo"],"position":"no"},{"experts":["ada"],"position":"yes"}]}],
		"decisions":["Ship without tests?"]}`})
	if good.IsError {
		t.Fatalf("council_record failed: %s", good.Content[0].Text)
	}
	text := good.Content[0].Text
	for _, want := range []string{"Virtual Ada", "disagrees with Virtual Cleo", "Ship without tests?", "Saved to"} {
		if !strings.Contains(text, want) {
			t.Errorf("recorded review is missing %q:\n%s", want, text)
		}
	}
	saved, _ := filepath.Glob(filepath.Join(".council", "reviews", "*-go.txt"))
	if len(saved) != 1 {
		t.Fatalf("expected one saved review, got %v", saved)
	}
	if data, _ := os.ReadFile(saved[0]); !strings.Contains(string(data), "Ship without tests?") {
		t.Error("the saved review should hold the rendered debate")
	}
}

func TestRoomSeveralCouncils(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()
	s := NewServer(strings.NewReader(""), io.Discard, "test")

	room := s.handleRoom(map[string]any{"councils": "product,security", "content": "a plan"})
	if room.IsError {
		t.Fatalf("council_room failed: %s", room.Content[0].Text)
	}
	if !strings.Contains(room.Content[0].Text, "## The security council") || !strings.Contains(room.Content[0].Text, `councils "product,security"`) {
		t.Errorf("multi-council room prompt is off:\n%s", room.Content[0].Text)
	}

	rec := s.handleRecord(map[string]any{"councils": "product,security", "answer": `{
		"councils":[
			{"council":"product","reviews":[{"expert":"ada","verdict":"pass","notes":["users want it"]}]},
			{"council":"security","reviews":[{"expert":"cleo","verdict":"block","notes":["leaks tokens"]}]}],
		"statements":[{"council":"product","position":"Ship."},{"council":"security","position":"Not yet."}],
		"decisions":["Ship before the fix?"]}`})
	if rec.IsError || !strings.Contains(rec.Content[0].Text, "The councils answer each other") {
		t.Errorf("council_record = %+v", rec)
	}
}

func TestRoomMissingFields(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()
	s := NewServer(strings.NewReader(""), io.Discard, "test")

	if r := s.handleRoom(map[string]any{"pack": "go"}); !r.IsError || !strings.Contains(r.Content[0].Text, "content") {
		t.Errorf("council_room without content = %+v", r)
	}
	if r := s.handleRoom(map[string]any{"pack": "nope", "content": "x"}); !r.IsError || !strings.Contains(r.Content[0].Text, "not found") {
		t.Errorf("council_room with an unknown pack = %+v", r)
	}
	if r := s.handleRecord(map[string]any{"pack": "go"}); !r.IsError || !strings.Contains(r.Content[0].Text, "answer") {
		t.Errorf("council_record without answer = %+v", r)
	}
}
