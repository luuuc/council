// Package mcp implements a Model Context Protocol server over stdin/stdout.
// It exposes council review functionality as MCP tools that any MCP-capable
// AI tool (Claude Code, Cursor, Claude Desktop) can call.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/review"
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
type Server struct {
	reader   io.Reader
	writer   io.Writer
	config   *config.Config
	backend  review.Backend
	version  string
	sessions map[string]*session // council_convene runs in progress
}

// Option configures a Server.
type Option func(*Server)

// WithBackend sets the review backend (useful for testing).
func WithBackend(b review.Backend) Option {
	return func(s *Server) { s.backend = b }
}

// NewServer creates an MCP server that communicates over the given reader/writer.
func NewServer(r io.Reader, w io.Writer, version string, opts ...Option) *Server {
	s := &Server{
		reader:  r,
		writer:  w,
		version: version,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
	case "council_review":
		result = s.handleReview(ctx, params.Arguments)
	case "council_list":
		result = s.handleList(params.Arguments)
	case "council_explain":
		result = s.handleExplain(ctx, params.Arguments)
	case "council_convene":
		result = s.handleConvene(params.Arguments)
	case "council_turn":
		result = s.handleTurn(params.Arguments)
	case "council_add_persona":
		result = s.handleAddPersona(params.Arguments)
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

func (s *Server) loadConfig() (*config.Config, error) {
	if s.config != nil {
		return s.config, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	s.config = cfg
	return cfg, nil
}

func (s *Server) getBackend() (review.Backend, error) {
	if s.backend != nil {
		return s.backend, nil
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	backend, provider, model := cfg.DetectBackend()
	switch backend {
	case "api":
		if provider == "" {
			return nil, fmt.Errorf("api backend requires a provider (anthropic, openai, ollama)")
		}
		b, err := review.NewAPIBackend(provider, model)
		if err != nil {
			return nil, err
		}
		s.backend = b
		return b, nil
	case "cli":
		aiCmd, err := cfg.DetectAICommand()
		if err != nil {
			return nil, err
		}
		b := review.NewCLIBackend(aiCmd, cfg.AI.Args)
		s.backend = b
		return b, nil
	default:
		return nil, fmt.Errorf("no backend available — install an AI CLI or set an API key")
	}
}

// toolDefinitions returns the MCP tool definitions for all council tools.
func toolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "council_review",
			Description: "Submit code for council review. Experts review one at a time and each sees the earlier reviews, so they can disagree, agree, or add to them. Earlier experts then get a final word, and a neutral moderator lists where they disagree and what the author must decide. Returns each expert's verdict, notes, and replies, plus disagreements, decisions, and agreements. No recommendation: the author decides.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"pack": {
						Type:        "string",
						Description: "Pack name to review with (e.g., \"rails\", \"go\", \"writing\")",
					},
					"content": {
						Type:        "string",
						Description: "The code diff, file content, or text to review",
					},
				},
				Required: []string{"pack", "content"},
			},
		},
		{
			Name: "council_convene",
			Description: "Start a council review where you take each member's turn yourself. Use this when council_review reports no AI backend " +
				"(for example in Claude Desktop with no AI CLI or API key). Returns the first member's prompt; write that member's review " +
				"and pass it to council_turn, which returns the next member's prompt with the earlier reviews, until the council finishes.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"pack": {
						Type:        "string",
						Description: "Pack name to review with (e.g., \"rails\", \"go\", \"writing\")",
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
				Required: []string{"pack", "content"},
			},
		},
		{
			Name:        "council_turn",
			Description: "Submit the current member's review in a council_convene session. Returns the next member's prompt, or the final debate when every member has spoken.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"session": {
						Type:        "string",
						Description: "Session ID returned by council_convene",
					},
					"review": {
						Type:        "string",
						Description: "The member's review as the JSON object their prompt asks for",
					},
				},
				Required: []string{"session", "review"},
			},
		},
		{
			Name: "council_add_persona",
			Description: "Add a persona modeled on a real person to the project council. Research the person's public talks, writing, and decisions first, " +
				"and stay faithful to what they have actually said. Council names them \"Virtual {Name}\". Check council_list first so tensions use real member IDs.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"persona": {
						Type:        "string",
						Description: "The persona as YAML in this format:\n" + addPersonaFormat,
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
		{
			Name:        "council_explain",
			Description: "Ask an expert to expand on a specific note from a review. Returns the expert's reasoning — which principles triggered the flag and what their worldview says about the pattern.",
			InputSchema: toolSchema{
				Type: "object",
				Properties: map[string]schemaProperty{
					"expert": {
						Type:        "string",
						Description: "Expert ID (e.g., \"kent-beck\", \"bruce-schneier\")",
					},
					"note": {
						Type:        "string",
						Description: "The specific note or flag from the review to explain",
					},
				},
				Required: []string{"expert", "note"},
			},
		},
	}
}
