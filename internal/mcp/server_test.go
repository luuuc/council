package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/pack"
)

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

func runServer(input string) (string, error) {
	var writer bytes.Buffer
	err := NewServer(strings.NewReader(input), &writer, "test").Run(context.Background())
	return writer.String(), err
}

func TestInitialize(t *testing.T) {
	input := sendRequest(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]string{"name": "test", "version": "1.0"},
		"capabilities":    map[string]any{},
	}) + "\n"

	output, err := runServer(input)
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

	output, err := runServer(input)
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

	if len(result.Tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(result.Tools))
	}

	names := make(map[string]bool)
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}

	for _, name := range []string{"council_room", "council_record", "council_list", "council_assemble", "council_add"} {
		if !names[name] {
			t.Errorf("missing tool %q", name)
		}
	}
}

func TestToolsCallList(t *testing.T) {
	// council_list with a builtin pack should work without .council/ on disk
	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "council_list",
		Arguments: map[string]any{"pack": "go"},
	}) + "\n"

	output, err := runServer(input)
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

	output, err := runServer(input)
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

func TestUnknownTool(t *testing.T) {
	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "nonexistent_tool",
		Arguments: map[string]any{},
	}) + "\n"

	output, err := runServer(input)
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

	output, err := runServer(input)
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

	output, err := runServer(input)
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

	output, err := runServer(input)
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

	output, err := runServer(input)
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

	// Packs: Council ships none, so the test council defines its own.
	for name, members := range map[string][]string{
		"go":       {"cleo", "ada"},
		"product":  {"ada"},
		"security": {"cleo"},
		"writing":  {"ada", "cleo"},
	} {
		p := &pack.Pack{Name: name}
		for _, id := range members {
			p.Members = append(p.Members, pack.Member{ID: id})
		}
		if err := pack.Save(p); err != nil {
			t.Fatalf("save pack %s: %v", name, err)
		}
	}

	return func() {
		_ = os.Chdir(origDir)
	}
}

func testExperts() []*expert.Expert {
	return []*expert.Expert{
		{
			ID:    "ada",
			Name:  "Virtual Ada",
			Focus: "TDD",
			Body:  "# Virtual Ada - TDD\n\nYou are Virtual Ada.",
		},
		{
			ID:    "cleo",
			Name:  "Virtual Cleo",
			Focus: "Go clarity",
			Body:  "# Virtual Cleo - Go clarity\n\nYou are Virtual Cleo.",
		},
	}
}

func TestToolsCallListHappyPath(t *testing.T) {
	cleanup := setupTestCouncil(t)
	defer cleanup()

	input := sendRequest(1, "tools/call", toolCallParams{
		Name:      "council_list",
		Arguments: map[string]any{"pack": "go"},
	}) + "\n"

	output, err := runServer(input)
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

	// Verify cleo is in the list (real-person expert in the go builtin pack)
	found := false
	for _, e := range listOutput.Experts {
		if e.ID == "cleo" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cleo in go pack list")
	}
}

func TestInvalidJSONRPCVersion(t *testing.T) {
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "1.0",
		"id":      1,
		"method":  "tools/list",
	})
	input := string(req) + "\n"

	output, err := runServer(input)
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
