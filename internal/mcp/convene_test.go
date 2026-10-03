package mcp

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

var sessionIDRe = regexp.MustCompile(`Council session ([0-9a-f]+):`)

func TestConveneRunsTurnsInOrder(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	s := NewServer(strings.NewReader(""), io.Discard, "test")

	res := s.handleConvene(map[string]any{"pack": "go", "content": "func main() {}"})
	if res.IsError {
		t.Fatalf("convene failed: %s", res.Content[0].Text)
	}
	text := res.Content[0].Text
	m := sessionIDRe.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("no session ID in:\n%s", text)
	}
	id := m[1]
	if !strings.Contains(text, "Turn 1 of 6: Virtual Rob Pike") || !strings.Contains(text, "func main() {}") {
		t.Fatalf("first turn should be Rob Pike's prompt with the submission:\n%s", text)
	}
	if strings.Contains(text, "The Council So Far") {
		t.Error("the first speaker should not see earlier reviews")
	}

	// A malformed review is rejected without advancing the turn.
	bad := s.handleTurn(map[string]any{"session": id, "review": "looks fine to me"})
	if !bad.IsError || !strings.Contains(bad.Content[0].Text, "Virtual Rob Pike") {
		t.Fatalf("expected a retry request for Rob Pike, got: %+v", bad)
	}

	res = s.handleTurn(map[string]any{"session": id, "review": `{"expert":"rob-pike","verdict":"block","confidence":0.9,"notes":["Interface has one implementation"],"blocking":false}`})
	text = res.Content[0].Text
	if res.IsError || !strings.Contains(text, "Turn 2 of 6: Virtual Kent Beck") {
		t.Fatalf("expected Kent Beck's turn, got:\n%s", text)
	}
	if !strings.Contains(text, "### Virtual Rob Pike (rob-pike) — block") || !strings.Contains(text, `"replies"`) {
		t.Errorf("second turn should include Rob Pike's review and ask for replies:\n%s", text)
	}

	res = s.handleTurn(map[string]any{"session": id, "review": `{"expert":"kent-beck","verdict":"comment","confidence":0.8,"notes":["Add a test"],"replies":[{"to":"rob-pike","stance":"disagree","note":"The interface makes it testable"}],"blocking":false}`})

	// The remaining four members of the go pack pass.
	for i := 3; i <= 6; i++ {
		if res.IsError || !strings.Contains(res.Content[0].Text, "Turn ") {
			t.Fatalf("expected turn %d, got:\n%s", i, res.Content[0].Text)
		}
		res = s.handleTurn(map[string]any{"session": id, "review": `{"verdict":"pass","confidence":0.9,"notes":["Fine"],"blocking":false}`})
	}
	text = res.Content[0].Text
	if res.IsError {
		t.Fatalf("final turn failed: %s", text)
	}
	for _, want := range []string{"The council has finished", "Virtual Kent Beck disagrees with Virtual Rob Pike", "Verdict:"} {
		if !strings.Contains(text, want) {
			t.Errorf("final result missing %q:\n%s", want, text)
		}
	}

	// The session is gone once the council finishes.
	if again := s.handleTurn(map[string]any{"session": id, "review": "{}"}); !again.IsError {
		t.Error("finished session should be rejected")
	}
}

func TestAddPersonaSavesVirtualPersona(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	s := NewServer(strings.NewReader(""), io.Discard, "test")
	res := s.handleAddPersona(map[string]any{"persona": `name: Boris Cherny
focus: TypeScript and agentic coding
principles:
  - Give the agent a way to verify its work
tensions:
  - expert: rob-pike
    topic: AI-written code
    position: Agents write most code now
    counterpoint: The craft matters
  - expert: someone-else
    topic: x
    position: y
    counterpoint: z`})
	if res.IsError {
		t.Fatalf("add persona failed: %s", res.Content[0].Text)
	}

	e, err := expert.Load("boris-cherny")
	if err != nil {
		t.Fatalf("persona not saved: %v", err)
	}
	if e.Name != "Virtual Boris Cherny" {
		t.Errorf("name = %q, want Virtual Boris Cherny", e.Name)
	}
	if len(e.Tensions) != 1 || e.Tensions[0].Expert != "rob-pike" {
		t.Errorf("tensions should keep only council members, got %+v", e.Tensions)
	}
}

func TestCouncilPrompt(t *testing.T) {
	input := sendRequest(1, "prompts/list", nil) + "\n" +
		sendRequest(2, "prompts/get", map[string]any{"name": "council", "arguments": map[string]string{"pack": "go", "topic": "the auth refactor"}}) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}
	resps, err := parseResponses(output)
	if err != nil || len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d (%v)", len(resps), err)
	}

	data, _ := json.Marshal(resps[1].Result)
	text := string(data)
	for _, want := range []string{"the auth refactor", "council_review", "council_convene"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q: %s", want, text)
		}
	}
}
