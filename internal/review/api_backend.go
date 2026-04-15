package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/luuuc/council-cli/internal/expert"
)

// APIBackend makes direct HTTP calls to LLM provider APIs.
type APIBackend struct {
	Provider string // "anthropic", "openai", "ollama"
	Model    string
	client   *http.Client
	config   providerConfig
}

// providerConfig captures the provider-specific API shape.
type providerConfig struct {
	URL         string
	Headers     func() map[string]string // provider-specific headers (auth, versioning, etc.)
	BuildBody   func(model, persona string) any
	ExtractText func(respBody []byte) (string, error)
}

// maxResponseSize caps response body reads to prevent OOM from misbehaving APIs.
const maxResponseSize = 1 << 20 // 1MB

// NewAPIBackend creates an APIBackend for the given provider and model.
func NewAPIBackend(provider, model string) (*APIBackend, error) {
	cfg, err := providerFor(provider)
	if err != nil {
		return nil, err
	}
	return &APIBackend{
		Provider: provider,
		Model:    model,
		client:   &http.Client{},
		config:   cfg,
	}, nil
}

// newAPIBackendWithClient is used by tests to inject a custom HTTP client.
func newAPIBackendWithClient(provider, model string, client *http.Client) (*APIBackend, error) {
	cfg, err := providerFor(provider)
	if err != nil {
		return nil, err
	}
	return &APIBackend{
		Provider: provider,
		Model:    model,
		client:   client,
		config:   cfg,
	}, nil
}

// SetBaseURL overrides the provider URL (used for testing with httptest).
func (b *APIBackend) SetBaseURL(url string) {
	b.config.URL = url
}

// Review executes a single expert review via the provider's API.
func (b *APIBackend) Review(ctx context.Context, e *expert.Expert, sub Submission) (ExpertVerdict, error) {
	prompt := sub.RawPrompt
	if prompt == "" {
		prompt = BuildPrompt(e, sub)
	}

	body, err := json.Marshal(b.config.BuildBody(b.Model, prompt))
	if err != nil {
		return ExpertVerdict{}, fmt.Errorf("marshal request for %s: %w", e.ID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config.URL, bytes.NewReader(body))
	if err != nil {
		return ExpertVerdict{}, fmt.Errorf("create request for %s: %w", e.ID, err)
	}

	req.Header.Set("Content-Type", "application/json")
	if b.config.Headers != nil {
		for name, value := range b.config.Headers() {
			req.Header.Set(name, value)
		}
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return ExpertVerdict{}, fmt.Errorf("API call failed for %s: %w", e.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return ExpertVerdict{}, fmt.Errorf("read response for %s: %w", e.ID, err)
	}

	if resp.StatusCode != http.StatusOK {
		detail := truncateBytes(respBody, 200)
		return ExpertVerdict{}, fmt.Errorf("API returned %d for %s: %s", resp.StatusCode, e.ID, detail)
	}

	text, err := b.config.ExtractText(respBody)
	if err != nil {
		return ExpertVerdict{}, fmt.Errorf("parse response for %s: %w", e.ID, err)
	}

	// RawPrompt mode: return the raw text directly instead of parsing verdict JSON.
	if sub.RawPrompt != "" {
		return ExpertVerdict{
			Expert:     e.ID,
			Verdict:    VerdictComment,
			Confidence: 1.0,
			Notes:      []string{strings.TrimSpace(text)},
		}, nil
	}

	verdict := ParseVerdict(e.ID, []byte(text))
	return verdict, nil
}

// --- Anthropic provider ---

func anthropicProvider() providerConfig {
	return providerConfig{
		URL: "https://api.anthropic.com/v1/messages",
		Headers: func() map[string]string {
			return map[string]string{
				"x-api-key":         os.Getenv("ANTHROPIC_API_KEY"),
				"anthropic-version": "2023-06-01",
			}
		},
		BuildBody: func(model, persona string) any {
			return map[string]any{
				"model":      model,
				"max_tokens": 1024,
				"messages": []map[string]string{
					{"role": "user", "content": persona},
				},
			}
		},
		ExtractText: func(respBody []byte) (string, error) {
			var resp struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			}
			if err := json.Unmarshal(respBody, &resp); err != nil {
				return "", fmt.Errorf("unmarshal anthropic response: %w", err)
			}
			if len(resp.Content) == 0 {
				return "", fmt.Errorf("empty content in anthropic response")
			}
			return resp.Content[0].Text, nil
		},
	}
}

// --- OpenAI provider ---

func openaiProvider() providerConfig {
	return providerConfig{
		URL: "https://api.openai.com/v1/chat/completions",
		Headers: func() map[string]string {
			return map[string]string{
				"Authorization": "Bearer " + os.Getenv("OPENAI_API_KEY"),
			}
		},
		BuildBody: func(model, persona string) any {
			return map[string]any{
				"model": model,
				"messages": []map[string]string{
					{"role": "user", "content": persona},
				},
			}
		},
		ExtractText: func(respBody []byte) (string, error) {
			var resp struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(respBody, &resp); err != nil {
				return "", fmt.Errorf("unmarshal openai response: %w", err)
			}
			if len(resp.Choices) == 0 {
				return "", fmt.Errorf("no choices in openai response")
			}
			return resp.Choices[0].Message.Content, nil
		},
	}
}

// --- Ollama provider ---

func ollamaProvider() providerConfig {
	return providerConfig{
		URL:     "http://localhost:11434/api/chat",
		Headers: nil, // no auth
		BuildBody: func(model, persona string) any {
			return map[string]any{
				"model":  model,
				"stream": false,
				"messages": []map[string]string{
					{"role": "user", "content": persona},
				},
			}
		},
		ExtractText: func(respBody []byte) (string, error) {
			var resp struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}
			if err := json.Unmarshal(respBody, &resp); err != nil {
				return "", fmt.Errorf("unmarshal ollama response: %w", err)
			}
			return resp.Message.Content, nil
		},
	}
}

// providerFor returns the providerConfig for a given provider name.
func providerFor(name string) (providerConfig, error) {
	switch name {
	case "anthropic":
		return anthropicProvider(), nil
	case "openai":
		return openaiProvider(), nil
	case "ollama":
		return ollamaProvider(), nil
	default:
		return providerConfig{}, fmt.Errorf("unknown provider: %s (supported: anthropic, openai, ollama)", name)
	}
}
