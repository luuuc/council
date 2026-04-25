package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/review"
)

// mockBackend returns canned verdicts for testing.
type mockBackend struct {
	results        map[string]review.ExpertVerdict
	errors         map[string]error
	delay          time.Duration
	calls          atomic.Int32
	lastSubmission review.Submission // captures the most recent Submission for assertions
}

func (m *mockBackend) Review(ctx context.Context, e *expert.Expert, sub review.Submission) (review.ExpertVerdict, error) {
	m.calls.Add(1)
	m.lastSubmission = sub

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return review.ExpertVerdict{}, ctx.Err()
		}
	}

	if err, ok := m.errors[e.ID]; ok {
		return review.ExpertVerdict{}, err
	}

	if v, ok := m.results[e.ID]; ok {
		return v, nil
	}

	return review.ExpertVerdict{
		Expert:     e.ID,
		Verdict:    review.VerdictPass,
		Confidence: 0.9,
		Notes:      []string{"Looks good."},
	}, nil
}

func (m *mockBackend) ReviewCollective(ctx context.Context, experts []*expert.Expert, sub review.Submission) (*review.SynthesizedResult, error) {
	m.calls.Add(1)
	m.lastSubmission = sub

	perspectives := make([]review.ExpertVerdict, len(experts))
	for i, e := range experts {
		if v, ok := m.results[e.ID]; ok {
			perspectives[i] = v
		} else {
			perspectives[i] = review.ExpertVerdict{
				Expert: e.ID, Verdict: review.VerdictPass, Confidence: 0.9,
			}
		}
	}
	return &review.SynthesizedResult{
		Verdict:      review.VerdictPass,
		Perspectives: perspectives,
		Summary:      "All good.",
	}, nil
}

// sendRequest marshals a JSON-RPC request and returns it as a line.
func sendRequest(id int, method string, params any) string {
	var p json.RawMessage
	if params != nil {
		p, _ = json.Marshal(params)
	}
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      mustMarshal(id),
		Method:  method,
		Params:  p,
	}
	data, _ := json.Marshal(req)
	return string(data)
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// parseResponse reads the first JSON-RPC response from output.
func parseResponse(output string) (*jsonrpcResponse, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("no output")
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (line: %s)", err, lines[0])
	}
	return &resp, nil
}

// parseResponses reads all JSON-RPC responses from output.
func parseResponses(output string) ([]*jsonrpcResponse, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var resps []*jsonrpcResponse
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w (line: %s)", err, line)
		}
		resps = append(resps, &resp)
	}
	return resps, nil
}

func runServer(input string, backend review.Backend) (string, error) {
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	var opts []Option
	if backend != nil {
		opts = append(opts, WithBackend(backend))
	}
	srv := NewServer(reader, &writer, "test", opts...)
	srv.config = &config.Config{
		AI: config.AIConfig{
			Concurrency: 2,
			Timeout:     10,
		},
	}

	err := srv.Run(context.Background())
	return writer.String(), err
}

func TestInitialize(t *testing.T) {
	input := sendRequest(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]string{"name": "test", "version": "1.0"},
		"capabilities":    map[string]any{},
	}) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	// Check result has serverInfo
	data, _ := json.Marshal(resp.Result)
	var result initializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.ServerInfo.Name != "council" {
		t.Errorf("expected server name 'council', got %q", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability")
	}
}

func TestToolsList(t *testing.T) {
	input := sendRequest(1, "tools/list", nil) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result toolsListResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result.Tools))
	}

	names := make(map[string]bool)
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}

	for _, name := range []string{"council_review", "council_list", "council_explain"} {
		if !names[name] {
			t.Errorf("missing tool %q", name)
		}
	}
}

func TestToolsCallReview(t *testing.T) {
	backend := &mockBackend{
		results: map[string]review.ExpertVerdict{
			"ada-redgrave": {
				Expert: "ada-redgrave", Verdict: review.VerdictComment,
				Confidence: 0.8, Notes: []string{"Add test for edge case"},
			},
			"nadia-kowalski": {
				Expert: "nadia-kowalski", Verdict: review.VerdictPass,
				Confidence: 0.95, Notes: []string{"No security concerns"},
			},
		},
	}

	// council_review needs real experts and packs on disk.
	// For a unit test, we test the server dispatch and error handling.
	// Full integration with pack resolution requires .council/ on disk.
	input := sendRequest(1, "tools/call", toolCallParams{
		Name: "council_review",
		Arguments: map[string]any{
			"pack":    "nonexistent-pack",
			"content": "test diff content",
		},
	}) + "\n"

	output, err := runServer(input, backend)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	// Should return a tool result (with isError=true since pack doesn't exist)
	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected isError=true for nonexistent pack")
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in error result")
	}

	if !strings.Contains(result.Content[0].Text, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", result.Content[0].Text)
	}
}

func TestToolsCallReviewMissingFields(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "missing pack",
			args: map[string]any{"content": "test"},
			want: "missing required field: pack",
		},
		{
			name: "missing content",
			args: map[string]any{"pack": "go"},
			want: "missing required field: content",
		},
		{
			name: "empty pack",
			args: map[string]any{"pack": "", "content": "test"},
			want: "missing required field: pack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := sendRequest(1, "tools/call", toolCallParams{
				Name:      "council_review",
				Arguments: tt.args,
			}) + "\n"

			output, err := runServer(input, &mockBackend{})
			if err != nil {
				t.Fatalf("server error: %v", err)
			}

			resp, err := parseResponse(output)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			data, _ := json.Marshal(resp.Result)
			var result toolCallResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if !result.IsError {
				t.Error("expected isError=true")
			}
			if !strings.Contains(result.Content[0].Text, tt.want) {
				t.Errorf("expected %q in error, got: %s", tt.want, result.Content[0].Text)
			}
		})
	}
}

func TestToolsCallList(t *testing.T) {
	// council_list with a builtin pack should work without .council/ on disk
	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "council_list",
		Arguments: map[string]any{"pack": "go"},
	}) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The builtin pack exists, but experts may not be on disk.
	// It should still return a valid response (possibly empty experts list).
	if result.IsError {
		// If experts aren't on disk, the list will be empty but not an error
		t.Logf("list returned error (expected if no experts on disk): %s", result.Content[0].Text)
	}
}

func TestToolsCallListMissingPack(t *testing.T) {
	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "council_list",
		Arguments: map[string]any{},
	}) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("expected isError=true for missing pack field")
	}
}

func TestToolsCallExplain(t *testing.T) {
	// Explain with nonexistent expert returns error
	input := sendRequest(1, "tools/call", toolCallParams{
		Name: "council_explain",
		Arguments: map[string]any{
			"expert": "nonexistent-expert",
			"note":   "Test note",
		},
	}) + "\n"

	output, err := runServer(input, &mockBackend{})
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("expected isError=true for nonexistent expert")
	}
	if !strings.Contains(result.Content[0].Text, "not found") {
		t.Errorf("expected 'not found' in error, got: %s", result.Content[0].Text)
	}
}

func TestToolsCallExplainMissingFields(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "missing expert",
			args: map[string]any{"note": "test"},
			want: "missing required field: expert",
		},
		{
			name: "missing note",
			args: map[string]any{"expert": "ada-redgrave"},
			want: "missing required field: note",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := sendRequest(1, "tools/call", toolCallParams{
				Name:      "council_explain",
				Arguments: tt.args,
			}) + "\n"

			output, err := runServer(input, &mockBackend{})
			if err != nil {
				t.Fatalf("server error: %v", err)
			}

			resp, err := parseResponse(output)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			data, _ := json.Marshal(resp.Result)
			var result toolCallResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if !result.IsError {
				t.Error("expected isError=true")
			}
			if !strings.Contains(result.Content[0].Text, tt.want) {
				t.Errorf("expected %q in error, got: %s", tt.want, result.Content[0].Text)
			}
		})
	}
}

func TestUnknownTool(t *testing.T) {
	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "nonexistent_tool",
		Arguments: map[string]any{},
	}) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected JSON-RPC error for unknown tool")
	}
	if resp.Error.Code != errCodeInvalidParams {
		t.Errorf("expected error code %d, got %d", errCodeInvalidParams, resp.Error.Code)
	}
}

func TestUnknownMethod(t *testing.T) {
	input := sendRequest(1, "unknown/method", nil) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected JSON-RPC error for unknown method")
	}
	if resp.Error.Code != errCodeMethodNotFound {
		t.Errorf("expected error code %d, got %d", errCodeMethodNotFound, resp.Error.Code)
	}
}

func TestMalformedJSON(t *testing.T) {
	input := "this is not json\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected JSON-RPC error for malformed JSON")
	}
	if resp.Error.Code != errCodeParse {
		t.Errorf("expected error code %d, got %d", errCodeParse, resp.Error.Code)
	}
}

func TestServerStaysAliveAfterError(t *testing.T) {
	// Send malformed JSON followed by a valid request
	input := "bad json\n" + sendRequest(1, "tools/list", nil) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resps, err := parseResponses(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}

	// First response is parse error
	if resps[0].Error == nil || resps[0].Error.Code != errCodeParse {
		t.Error("expected parse error for first response")
	}

	// Second response is valid tools/list
	if resps[1].Error != nil {
		t.Errorf("expected no error for tools/list, got: %v", resps[1].Error)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	// notifications/initialized should not produce a response
	input := sendRequest(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]string{"name": "test", "version": "1.0"},
		"capabilities":    map[string]any{},
	}) + "\n"

	// Add notification (no response expected)
	notif, _ := json.Marshal(jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	})
	input += string(notif) + "\n"

	// Add tools/list to verify we get exactly 2 responses
	input += sendRequest(2, "tools/list", nil) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resps, err := parseResponses(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Should have exactly 2 responses (initialize + tools/list), not 3
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses (notification should not generate one), got %d", len(resps))
	}
}

// setupTestCouncil creates a temp directory with .council/experts/ containing
// test experts, and changes into it. Returns a cleanup function.
func setupTestCouncil(t *testing.T) func() {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create .council/experts/
	expertsDir := filepath.Join(tmpDir, ".council", "experts")
	if err := os.MkdirAll(expertsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write test experts
	for _, e := range testExperts() {
		if err := expert.SaveToPath(e, filepath.Join(expertsDir, e.ID+".md")); err != nil {
			t.Fatalf("save expert %s: %v", e.ID, err)
		}
	}

	return func() {
		_ = os.Chdir(origDir)
	}
}

func testExperts() []*expert.Expert {
	return []*expert.Expert{
		{
			ID:    "ada-redgrave",
			Name:  "The TDD Advocate",
			Focus: "TDD",
			Body:  "# The TDD Advocate - TDD\n\nYou are The TDD Advocate.",
		},
		{
			ID:    "sable-okoro",
			Name:  "The Go Purist",
			Focus: "Go clarity",
			Body:  "# The Go Purist - Go clarity\n\nYou are The Go Purist.",
		},
	}
}

func TestToolsCallReviewHappyPath(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	backend := &mockBackend{
		results: map[string]review.ExpertVerdict{
			"ada-redgrave": {
				Expert: "ada-redgrave", Verdict: review.VerdictComment,
				Confidence: 0.8, Notes: []string{"Add test for edge case"},
			},
			"sable-okoro": {
				Expert: "sable-okoro", Verdict: review.VerdictPass,
				Confidence: 0.95, Notes: []string{"Clean and idiomatic"},
			},
		},
	}

	// Use the "go" builtin pack — it includes ada-redgrave and sable-okoro
	input := sendRequest(1, "tools/call", toolCallParams{
		Name: "council_review",
		Arguments: map[string]any{
			"pack":    "go",
			"content": "func main() { fmt.Println(\"hello\") }",
		},
	}) + "\n"

	output, err := runServer(input, backend)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].Text)
	}

	// Parse the verdict JSON from the tool result
	var verdict review.SynthesizedResult
	if err := json.Unmarshal([]byte(result.Content[0].Text), &verdict); err != nil {
		t.Fatalf("unmarshal verdict: %v", err)
	}

	if len(verdict.Perspectives) == 0 {
		t.Error("expected at least one perspective")
	}
	if verdict.Verdict == "" {
		t.Error("expected a verdict")
	}

	// Verify the mock was called
	if backend.calls.Load() == 0 {
		t.Error("expected backend to be called")
	}
}

func TestToolsCallExplainHappyPath(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	backend := &mockBackend{
		results: map[string]review.ExpertVerdict{
			"ada-redgrave": {
				Expert: "ada-redgrave", Verdict: review.VerdictComment,
				Confidence: 0.9,
				Notes: []string{"This pattern violates the Single Responsibility Principle. The function handles both parsing and validation, which should be separated for testability."},
			},
		},
	}

	input := sendRequest(1, "tools/call", toolCallParams{
		Name: "council_explain",
		Arguments: map[string]any{
			"expert": "ada-redgrave",
			"note":   "No test for the empty-state CSV.",
		},
	}) + "\n"

	output, err := runServer(input, backend)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].Text)
	}

	// Explain should return natural language text, not JSON verdict
	explanation := result.Content[0].Text
	if explanation == "" {
		t.Error("expected non-empty explanation")
	}

	// Should NOT contain verdict JSON structure
	if strings.Contains(explanation, `"verdict"`) {
		t.Error("explain should return natural language, not verdict JSON")
	}

	if backend.calls.Load() != 1 {
		t.Errorf("expected 1 backend call, got %d", backend.calls.Load())
	}

	// Verify the backend received RawPrompt (not the review prompt template)
	if backend.lastSubmission.RawPrompt == "" {
		t.Error("expected RawPrompt to be set for explain calls")
	}
	if backend.lastSubmission.Content != "" {
		t.Error("expected Content to be empty when RawPrompt is used")
	}
}

func TestToolsCallListHappyPath(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "council_list",
		Arguments: map[string]any{"pack": "go"},
	}) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result toolCallResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].Text)
	}

	// Parse the list output
	var listOutput struct {
		Pack    string `json:"pack"`
		Experts []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"experts"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &listOutput); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}

	if listOutput.Pack != "go" {
		t.Errorf("expected pack 'go', got %q", listOutput.Pack)
	}
	if len(listOutput.Experts) == 0 {
		t.Error("expected at least one expert in list")
	}

	// Verify sable-okoro is in the list (composite expert in the go builtin pack)
	found := false
	for _, e := range listOutput.Experts {
		if e.ID == "sable-okoro" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sable-okoro in go pack list")
	}
}

func TestInvalidJSONRPCVersion(t *testing.T) {
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "1.0",
		"id":      1,
		"method":  "tools/list",
	})
	input := string(req) + "\n"

	output, err := runServer(input, nil)
	if err != nil {
		t.Fatalf("server error: %v", err)
	}

	resp, err := parseResponse(output)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for invalid jsonrpc version")
	}
	if resp.Error.Code != errCodeInvalidRequest {
		t.Errorf("expected code %d, got %d", errCodeInvalidRequest, resp.Error.Code)
	}
}
