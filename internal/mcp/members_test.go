package mcp

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestAddSavesVirtualPersona(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	s := NewServer(strings.NewReader(""), io.Discard, "test")
	res := s.handleAdd(map[string]any{"persona": `name: Jay
focus: TypeScript and agentic coding
sources:
  - "A talk on agentic coding"
principles:
  - Give the agent a way to verify its work
tensions:
  - expert: cleo
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

	e, err := expert.Load("jay")
	if err != nil {
		t.Fatalf("persona not saved: %v", err)
	}
	if e.Name != "Virtual Jay" {
		t.Errorf("name = %q, want Virtual Jay", e.Name)
	}
	if len(e.Tensions) != 1 || e.Tensions[0].Expert != "cleo" {
		t.Errorf("tensions should keep only council members, got %+v", e.Tensions)
	}
}

func TestCouncilPrompt(t *testing.T) {
	input := sendRequest(1, "prompts/list", nil) + "\n" +
		sendRequest(2, "prompts/get", map[string]any{"name": "council", "arguments": map[string]string{"pack": "go", "topic": "the auth refactor"}}) + "\n"

	output, err := runServer(input)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}
	resps, err := parseResponses(output)
	if err != nil || len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d (%v)", len(resps), err)
	}

	data, _ := json.Marshal(resps[1].Result)
	text := string(data)
	for _, want := range []string{"the auth refactor", "council_room", "council_record"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q: %s", want, text)
		}
	}

	list, _ := json.Marshal(resps[0].Result)
	if !strings.Contains(string(list), `"name":"assemble"`) {
		t.Errorf("prompts/list should offer assemble: %s", list)
	}
}

func TestAddCustomer(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	s := NewServer(strings.NewReader(""), io.Discard, "test")
	res := s.handleAdd(map[string]any{"persona": `name: Freelance Designer
kind: customer
focus: Solo designer who bills clients monthly
sources:
  - "Support threads about invoicing"
principles:
  - Send an invoice in two minutes`})
	if res.IsError {
		t.Fatalf("add customer failed: %s", res.Content[0].Text)
	}
	e, err := expert.Load("customer-freelance-designer")
	if err != nil || e.Name != "Customer: Freelance Designer" || e.Kind != expert.KindCustomer {
		t.Fatalf("customer not saved as expected: %+v, %v", e, err)
	}

	if bad := s.handleAdd(map[string]any{"persona": "name: X\nkind: alien\nfocus: Y"}); !bad.IsError {
		t.Error("unknown kind should be rejected")
	}
	if bad := s.handleAdd(map[string]any{"persona": "name: Jane Doe\nfocus: Y\nprinciples:\n  - x"}); !bad.IsError || !strings.Contains(bad.Content[0].Text, "public sources") {
		t.Errorf("a person without sources should be rejected, got %+v", bad)
	}
}

func TestAssembleBrief(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	s := NewServer(strings.NewReader(""), io.Discard, "test")
	res := s.handleAssemble()
	if res.IsError {
		t.Fatalf("assemble failed: %s", res.Content[0].Text)
	}
	for _, want := range []string{"# Assemble a Council", "- ada: Virtual Ada", "Current councils (packs):", "council add -"} {
		if !strings.Contains(res.Content[0].Text, want) {
			t.Errorf("brief missing %q", want)
		}
	}
}
