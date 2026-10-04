// Package mcp implements a Model Context Protocol server over stdin/stdout.
// It exposes council review functionality as MCP tools that any MCP-capable
// AI tool (Claude Code, Cursor, Claude Desktop) can call.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
)

// JSON-RPC 2.0 types

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // may be null for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	errCodeParse          = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternal       = -32603
)

// MCP protocol types

type initializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	ServerInfo      mcpServerInfo    `json:"serverInfo"`
	Capabilities    serverCapability `json:"capabilities"`
}

type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapability struct {
	Tools   *toolsCapability   `json:"tools,omitempty"`
	Prompts *promptsCapability `json:"prompts,omitempty"`
}

type toolsCapability struct{}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

type toolDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema toolSchema `json:"inputSchema"`
}

type toolSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type schemaProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Server is the MCP server that reads JSON-RPC from reader and writes to writer.
// It makes no model calls: the client's own model answers the room prompt.
type Server struct {
	reader  io.Reader
	writer  io.Writer
	version string
}

// NewServer creates an MCP server that communicates over the given reader/writer.
func NewServer(r io.Reader, w io.Writer, version string) *Server {
	return &Server{reader: r, writer: w, version: version}
}

// Run starts the server loop, reading JSON-RPC requests until EOF.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.reader)
	scanner.Buffer(make([]byte, 0, 4096), 10*1024*1024) // 10MB max message

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, errCodeParse, "parse error", err.Error())
			continue
		}

		if req.JSONRPC != "2.0" {
			s.sendError(req.ID, errCodeInvalidRequest, "invalid request", "jsonrpc must be \"2.0\"")
			continue
		}

		s.dispatch(ctx, &req)
	}

	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, req *jsonrpcRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// Client acknowledgment — no response needed
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(ctx, req)
	case "prompts/list":
		s.handlePromptsList(req)
	case "prompts/get":
		s.handlePromptsGet(req)
	default:
		s.sendError(req.ID, errCodeMethodNotFound, "method not found", req.Method)
	}
}

func (s *Server) handleInitialize(req *jsonrpcRequest) {
	v := s.version
	if v == "" {
		v = "dev"
	}
	s.sendResult(req.ID, initializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: mcpServerInfo{
			Name:    "council",
			Version: v,
		},
		Capabilities: serverCapability{
			Tools:   &toolsCapability{},
			Prompts: &promptsCapability{},
		},
	})
}

func (s *Server) handleToolsList(req *jsonrpcRequest) {
	s.sendResult(req.ID, toolsListResult{
		Tools: toolDefinitions(),
	})
}

func (s *Server) handleToolsCall(ctx context.Context, req *jsonrpcRequest) {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, errCodeInvalidParams, "invalid params", err.Error())
		return
	}

	var result toolCallResult
	switch params.Name {
	case "council_room":
		result = s.handleRoom(params.Arguments)
	case "council_record":
		result = s.handleRecord(params.Arguments)
	case "council_list":
		result = s.handleList(params.Arguments)
	case "council_assemble":
		result = s.handleAssemble()
	case "council_add":
		result = s.handleAdd(params.Arguments)
	default:
		s.sendError(req.ID, errCodeInvalidParams, "unknown tool", params.Name)
		return
	}

	s.sendResult(req.ID, result)
}

func (s *Server) sendResult(id json.RawMessage, result any) {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.writeResponse(resp)
}

func (s *Server) sendError(id json.RawMessage, code int, message string, data any) {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonrpcError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.writeResponse(resp)
}

func (s *Server) writeResponse(resp jsonrpcResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return // can't do much if marshaling fails
	}
	data = append(data, '\n')
	_, _ = s.writer.Write(data)
}

// toolDefinitions returns the MCP tool definitions for all council tools.
func toolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name: "council_room",
			Description: "Convene the council: returns the room prompt with every member, the submission, and the debate rules. " +
				"Answer it yourself, in one pass, with the whole debate as the JSON object it asks for, then pass that to council_record. " +
				"Members speak in order and react to each other, earlier members get a final word, and a neutral moderator lists " +
				"disagreements and decisions. No recommendation: the user decides.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"pack": {
						Type:        "string",
						Description: "Pack to convene (e.g. \"product\", \"code\"); omit for every member",
					},
					"councils": {
						Type:        "string",
						Description: "Optional: several packs, comma-separated (e.g. \"product,security\"). Each council debates, then their spokespersons answer each other; pack is ignored.",
					},
					"content": {
						Type:        "string",
						Description: "What the council reviews: a diff, code, a document, or a brief describing a question, plan, or decision",
					},
					"context": {
						Type:        "string",
						Description: "Optional background for the council (e.g., the goal, constraints, or options being considered)",
					},
				},
				Required: []string{"content"},
			},
		},
		{
			Name: "council_record",
			Description: "Record your answer to council_room's prompt. Council checks it and returns the review to show the user, " +
				"or says what to fix so you can call it again. Pass the same pack or councils as council_room.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"pack": {
						Type:        "string",
						Description: "Pack to convene (e.g. \"product\", \"code\"); omit for every member",
					},
					"councils": {
						Type:        "string",
						Description: "Optional: several packs, comma-separated (e.g. \"product,security\"). Each council debates, then their spokespersons answer each other; pack is ignored.",
					},
					"answer": {
						Type:        "string",
						Description: "The whole debate: the JSON object the room prompt asks for",
					},
				},
				Required: []string{"answer"},
			},
		},
		{
			Name: "council_assemble",
			Description: "Get the brief for assembling this project's council. Council ships no people: follow the brief to read the project, " +
				"propose members (people with documented public positions who disagree, roles, customers) with reasons, let the user choose, " +
				"build each persona from public material or evidence, and save it with council_add.",
			InputSchema: toolSchema{Type: "object", Properties: map[string]schemaProperty{}},
		},
		{
			Name: "council_add",
			Description: "Save a member you built by following council_assemble: a person (needs public sources; named \"Virtual {Name}\" with a no-affiliation disclaimer), " +
				"a role, or a customer (needs evidence). Council checks the persona and explains anything to fix.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"persona": {
						Type:        "string",
						Description: "The persona as YAML, in the format the council_assemble brief shows (with kind: person, role, or customer)",
					},
				},
				Required: []string{"persona"},
			},
		},
		{
			Name:        "council_list",
			Description: "List experts in a pack with their focus areas, blocking status, and tension relationships. No LLM calls — reads pack configuration.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"pack": {
						Type:        "string",
						Description: "Pack name to list (e.g., \"rails\", \"go\", \"writing\")",
					},
				},
				Required: []string{"pack"},
			},
		},
	}
}
