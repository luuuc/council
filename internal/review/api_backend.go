package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// APIBackend sends the room prompt to a model API with the user's key,
// for unattended reviews (the GitHub Action).
type APIBackend struct {
	Provider string // "anthropic", "openai", "ollama", "github"
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

// Complete sends a prompt to the provider's API and returns the answer.
func (b *APIBackend) Complete(ctx context.Context, prompt string) (string, error) {
	return b.doRequest(ctx, prompt)
}

// doRequest sends a prompt to the provider API and returns the extracted text.
func (b *APIBackend) doRequest(ctx context.Context, prompt string) (string, error) {
	label := b.Provider
	body, err := json.Marshal(b.config.BuildBody(b.Model, prompt))
	if err != nil {
		return "", fmt.Errorf("marshal request for %s: %w", label, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config.URL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request for %s: %w", label, err)
	}

	req.Header.Set("Content-Type", "application/json")
	if b.config.Headers != nil {
		for name, value := range b.config.Headers() {
			req.Header.Set(name, value)
		}
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed for %s: %w", label, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("read response for %s: %w", label, err)
	}

	if resp.StatusCode != http.StatusOK {
		detail := truncateBytes(respBody, 200)
		return "", fmt.Errorf("API returned %d for %s: %s", resp.StatusCode, label, detail)
	}

	text, err := b.config.ExtractText(respBody)
	if err != nil {
		return "", fmt.Errorf("parse response for %s: %w", label, err)
	}

	return text, nil
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
				"max_tokens": 16000, // the whole debate comes back in one answer
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

// --- OpenAI-compatible shared layer (used by openai and github providers) ---

func openaiCompatBuildBody(model, persona string) any {
	return map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": persona},
		},
	}
}

func openaiCompatExtractText(respBody []byte) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("unmarshal openai-compatible response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in openai-compatible response")
	}
	return resp.Choices[0].Message.Content, nil
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
		BuildBody:   openaiCompatBuildBody,
		ExtractText: openaiCompatExtractText,
	}
}

// --- GitHub Models provider ---

func githubProvider() providerConfig {
	return providerConfig{
		URL: "https://models.github.ai/inference/chat/completions",
		Headers: func() map[string]string {
			return map[string]string{
				"Authorization": "Bearer " + os.Getenv("GITHUB_TOKEN"),
			}
		},
		BuildBody:   openaiCompatBuildBody,
		ExtractText: openaiCompatExtractText,
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
	case "github":
		return githubProvider(), nil
	default:
		return providerConfig{}, fmt.Errorf("unknown provider: %s (supported: anthropic, openai, ollama, github)", name)
	}
}
