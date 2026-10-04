package review

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// serve starts a test server answering every request with the given status
// and body, and records the last request's headers and JSON body.
func serve(t *testing.T, status int, body string) (*httptest.Server, *http.Header, *map[string]any) {
	t.Helper()
	var headers http.Header
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &headers, &reqBody
}

func backendFor(t *testing.T, provider, model string, srv *httptest.Server) *APIBackend {
	t.Helper()
	b, err := newAPIBackendWithClient(provider, model, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	b.SetBaseURL(srv.URL)
	return b
}

func TestAPIBackendComplete(t *testing.T) {
	openaiAnswer := `{"choices":[{"message":{"content":"the debate"}}]}`
	tests := []struct {
		provider  string
		env       string
		response  string
		header    string
		headerVal string
	}{
		{"anthropic", "ANTHROPIC_API_KEY", `{"content":[{"type":"text","text":"the debate"}]}`, "x-api-key", "key"},
		{"openai", "OPENAI_API_KEY", openaiAnswer, "Authorization", "Bearer key"},
		{"github", "GITHUB_TOKEN", openaiAnswer, "Authorization", "Bearer key"},
		{"ollama", "", `{"message":{"content":"the debate"}}`, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv(tt.env, "key")
			}
			srv, headers, body := serve(t, http.StatusOK, tt.response)

			got, err := backendFor(t, tt.provider, "some-model", srv).Complete(context.Background(), "the room prompt")
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if got != "the debate" {
				t.Errorf("answer = %q, want %q", got, "the debate")
			}
			if tt.header != "" && headers.Get(tt.header) != tt.headerVal {
				t.Errorf("%s = %q, want %q", tt.header, headers.Get(tt.header), tt.headerVal)
			}
			if (*body)["model"] != "some-model" {
				t.Errorf("model = %v", (*body)["model"])
			}
			msgs, _ := (*body)["messages"].([]any)
			if len(msgs) != 1 || !strings.Contains(toJSON(msgs[0]), "the room prompt") {
				t.Errorf("messages = %v, want the prompt as the one user message", msgs)
			}
		})
	}
}

func TestAPIBackendAnthropicLeavesRoomForTheDebate(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "key")
	srv, _, body := serve(t, http.StatusOK, `{"content":[{"text":"ok"}]}`)
	if _, err := backendFor(t, "anthropic", "m", srv).Complete(context.Background(), "p"); err != nil {
		t.Fatal(err)
	}
	if n, _ := (*body)["max_tokens"].(float64); n < 8000 {
		t.Errorf("max_tokens = %v, too small for a whole debate", n)
	}
}

func TestAPIBackendErrors(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		status   int
		response string
		want     string
	}{
		{"http error", "openai", http.StatusUnauthorized, `{"error":"bad key"}`, "401"},
		{"server error", "anthropic", http.StatusInternalServerError, "boom", "500"},
		{"empty anthropic content", "anthropic", http.StatusOK, `{"content":[]}`, "empty content"},
		{"no openai choices", "openai", http.StatusOK, `{"choices":[]}`, "no choices"},
		{"malformed", "github", http.StatusOK, `not json`, "parse response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _, _ := serve(t, tt.status, tt.response)
			_, err := backendFor(t, tt.provider, "m", srv).Complete(context.Background(), "p")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestAPIBackendContextCancellation(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := backendFor(t, "ollama", "m", srv).Complete(ctx, "p"); err == nil {
		t.Fatal("expected an error when the context is cancelled")
	}
}

func TestAPIBackendUnknownProvider(t *testing.T) {
	if _, err := NewAPIBackend("nope", "m"); err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("err = %v, want unknown provider", err)
	}
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
