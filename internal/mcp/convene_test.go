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
	if !strings.Contains(text, "Turn 1 of up to 12: Virtual Rob Pike.") || !strings.Contains(text, "func main() {}") {
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

	res = s.handleTurn(map[string]any{"session": id, "review": `{"verdict":"block","confidence":0.9,"notes":["Interface has one implementation"],"blocking":false}`})
	text = res.Content[0].Text
	if res.IsError || !strings.Contains(text, "Turn 2 of up to 12: Virtual Kent Beck.") {
		t.Fatalf("expected Kent Beck's turn, got:\n%s", text)
	}
	if !strings.Contains(text, "### Virtual Rob Pike (rob-pike) — block") || !strings.Contains(text, `"replies"`) {
		t.Errorf("second turn should include Rob Pike's review and ask for replies:\n%s", text)
	}

	res = s.handleTurn(map[string]any{"session": id, "review": `{"verdict":"comment","confidence":0.8,"notes":["Add a test"],"replies":[{"to":"rob-pike","stance":"disagree","note":"The interface makes it testable"}],"blocking":false}`})

	// Answer every remaining turn until the council finishes.
	sawFinalWord, sawModerator := false, false
	for i := 0; i < 20 && !strings.Contains(res.Content[0].Text, "The council has finished"); i++ {
		text = res.Content[0].Text
		if res.IsError {
			t.Fatalf("turn failed: %s", text)
		}
		answer := `{"verdict":"pass","confidence":0.9,"notes":["Fine"],"blocking":false}`
		switch {
		case strings.Contains(text, "Virtual Rob Pike (final word)."):
			sawFinalWord = true
			if !strings.Contains(text, "## What Came After You") || !strings.Contains(text, "The interface makes it testable") {
				t.Errorf("Rob Pike's final word should show what came after him:\n%s", text)
			}
			answer = `{"verdict":"comment","change_reason":"A test fake is a fair need","replies":[{"to":"kent-beck","stance":"agree","note":"Fair, keep it small"}]}`
		case strings.Contains(text, "Moderator (disagreements and decisions)."):
			sawModerator = true
			answer = `{"agreements":["Add a test"],"disagreements":[{"topic":"Keep the interface?","sides":[{"experts":["rob-pike"],"position":"Drop it"},{"experts":["kent-beck"],"position":"Keep it for tests"}]}],"decisions":["Do you need a fake now?"]}`
		}
		res = s.handleTurn(map[string]any{"session": id, "review": answer})
	}

	text = res.Content[0].Text
	if !sawFinalWord || !sawModerator {
		t.Fatalf("expected a final word and a moderator turn (final word: %v, moderator: %v)", sawFinalWord, sawModerator)
	}
	for _, want := range []string{
		"The council has finished",
		"Virtual Rob Pike — final word",
		"block → comment",
		"Where they disagree",
		"Keep the interface?",
		"What you need to decide",
		"Do you need a fake now?",
	} {
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

func TestAddPersonaCustomer(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	s := NewServer(strings.NewReader(""), io.Discard, "test")
	res := s.handleAddPersona(map[string]any{"kind": "customer", "persona": `name: Freelance Designer
focus: Solo designer who bills clients monthly
principles:
  - Send an invoice in two minutes`})
	if res.IsError {
		t.Fatalf("add customer failed: %s", res.Content[0].Text)
	}
	e, err := expert.Load("customer-freelance-designer")
	if err != nil || e.Name != "Customer: Freelance Designer" || e.Kind != expert.KindCustomer {
		t.Fatalf("customer not saved as expected: %+v, %v", e, err)
	}

	if bad := s.handleAddPersona(map[string]any{"kind": "alien", "persona": "name: X\nfocus: Y"}); !bad.IsError {
		t.Error("unknown kind should be rejected")
	}
}

func TestReviewWithCouncils(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "council_review",
		Arguments: map[string]any{"pack": "go", "councils": "product, security", "content": "Require 2FA at signup"},
	}) + "\n"
	output, err := runServer(input, &mockBackend{})
	if err != nil {
		t.Fatalf("server error: %v", err)
	}
	resp, err := parseResponse(output)
	if err != nil || resp.Error != nil {
		t.Fatalf("unexpected error: %v %v", err, resp.Error)
	}
	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	_ = json.Unmarshal(data, &result)
	if result.IsError {
		t.Fatalf("tool error: %s", result.Content[0].Text)
	}

	var councils struct {
		Councils []struct {
			Name string `json:"name"`
		} `json:"councils"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &councils); err != nil {
		t.Fatalf("result is not councils JSON: %v\n%s", err, result.Content[0].Text)
	}
	if len(councils.Councils) != 2 || councils.Councils[0].Name != "product" || councils.Councils[1].Name != "security" {
		t.Errorf("expected product and security councils, got %+v", councils.Councils)
	}
}

func TestConveneCouncils(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	s := NewServer(strings.NewReader(""), io.Discard, "test")
	res := s.handleConvene(map[string]any{"councils": "writing, security", "content": "Launch post draft"})
	if res.IsError {
		t.Fatalf("convene failed: %s", res.Content[0].Text)
	}
	text := res.Content[0].Text
	id := sessionIDRe.FindStringSubmatch(text)[1]
	if !strings.Contains(text, "2 councils will each debate in turn (writing, security)") || !strings.Contains(text, ": writing council: Virtual Luc Perussault-Diallo.") {
		t.Fatalf("expected the writing council's first turn:\n%s", text)
	}

	sawSpokesperson, sawCross := 0, false
	for i := 0; i < 60 && !strings.Contains(res.Content[0].Text, "The councils have finished"); i++ {
		text = res.Content[0].Text
		if res.IsError {
			t.Fatalf("turn failed: %s", text)
		}
		answer := `{"verdict":"comment","confidence":0.8,"notes":["Fine"],"blocking":false}`
		switch {
		case strings.Contains(text, "Spokesperson for the writing council."):
			sawSpokesperson++
			answer = `{"position":"Cut it in half.","challenges":[{"to":"security","stance":"disagree","note":"The warning box buries the lede"}]}`
		case strings.Contains(text, "Spokesperson for the security council."):
			sawSpokesperson++
			answer = `{"position":"Mention the breach.","challenges":[{"to":"writing","stance":"disagree","note":"Hiding it costs trust"}]}`
		case strings.Contains(text, "Moderator across councils."):
			sawCross = true
			answer = `{"disagreements":[{"topic":"Mention the breach?","sides":[{"experts":["writing"],"position":"Keep it short"},{"experts":["security"],"position":"Say it plainly"}]}],"decisions":["How prominent is the breach note?"]}`
		case strings.Contains(text, "Moderator (disagreements and decisions)."):
			answer = `{"decisions":["Within-council decision"]}`
		}
		res = s.handleTurn(map[string]any{"session": id, "review": answer})
	}

	text = res.Content[0].Text
	if sawSpokesperson != 2 || !sawCross {
		t.Fatalf("expected 2 spokespersons and a cross moderator (got %d, %v)", sawSpokesperson, sawCross)
	}
	for _, want := range []string{"The writing council (", "The security council (", "The councils answer each other", "→ disagrees with the security council:", "Mention the breach?", "How prominent is the breach note?"} {
		if !strings.Contains(text, want) {
			t.Errorf("final result missing %q:\n%s", want, text)
		}
	}
}
