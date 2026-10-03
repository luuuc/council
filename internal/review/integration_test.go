package review

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

// realWorldDiff is a representative multi-file PR diff for integration testing.
const realWorldDiff = `diff --git a/internal/handler/export.go b/internal/handler/export.go
--- a/internal/handler/export.go
+++ b/internal/handler/export.go
@@ -10,6 +10,7 @@ import (
 	"encoding/csv"
 	"net/http"
+	"strconv"
 )

 func ExportCSV(w http.ResponseWriter, r *http.Request) {
@@ -20,4 +21,12 @@ func ExportCSV(w http.ResponseWriter, r *http.Request) {
 	w.Header().Set("Content-Type", "text/csv")
 	writer := csv.NewWriter(w)
+	for _, record := range records {
+		row := []string{
+			record.Name,
+			strconv.Itoa(record.Count),
+		}
+		writer.Write(row)
+	}
+	writer.Flush()
 }
diff --git a/internal/handler/export_test.go b/internal/handler/export_test.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/internal/handler/export_test.go
@@ -0,0 +1,15 @@
+package handler
+
+import (
+	"net/http/httptest"
+	"testing"
+)
+
+func TestExportCSV(t *testing.T) {
+	w := httptest.NewRecorder()
+	r := httptest.NewRequest("GET", "/export", nil)
+	ExportCSV(w, r)
+	if w.Code != 200 {
+		t.Errorf("got %d, want 200", w.Code)
+	}
+}
`

func testExperts() []*expert.Expert {
	return []*expert.Expert{
		{
			ID:    "ada",
			Name:  "Virtual Ada",
			Focus: "Testing and quality assurance",
			Body:  "You care about edge cases, test coverage, and correctness.",
		},
		{
			ID:    "kai-westbrook",
			Name:  "Kai Westbrook",
			Focus: "Pragmatic engineering",
			Body:  "You focus on shipping incrementally and avoiding premature abstraction.",
		},
	}
}

// TestEndToEndPRReview verifies the full pipeline: mock LLM → parse → format → GitHub output.
func TestEndToEndPRReview(t *testing.T) {
	collectiveJSON := `{
		"verdict": "comment",
		"blocking": false,
		"perspectives": [
			{
				"expert": "ada",
				"verdict": "comment",
				"confidence": 0.8,
				"notes": ["internal/handler/export.go:24: No error handling on writer.Write — CSV write errors are silently dropped"],
				"blocking": false
			},
			{
				"expert": "kai-westbrook",
				"verdict": "pass",
				"confidence": 0.9,
				"notes": ["Ship it, the test covers the happy path"],
				"blocking": false
			}
		],
		"agreements": ["Export endpoint handles the happy path correctly"],
		"tension": "Ada wants error handling on Write; Kai says CSV to HTTP response won't fail in practice.",
		"summary": "Ship with the error handling comment."
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": collectiveJSON}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("GITHUB_TOKEN", "test-token")

	backend, err := newAPIBackendWithClient("github", "openai/gpt-4.1-mini", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	backend.SetBaseURL(server.URL)

	experts := testExperts()
	sub := Submission{Content: realWorldDiff}

	result, err := backend.ReviewCollective(context.Background(), experts, sub)
	if err != nil {
		t.Fatalf("ReviewCollective failed: %v", err)
	}

	// Verify synthesized result
	if result.Verdict != VerdictComment {
		t.Errorf("verdict = %s, want comment", result.Verdict)
	}
	if len(result.Perspectives) != 2 {
		t.Errorf("expected 2 perspectives, got %d", len(result.Perspectives))
	}
	if result.Tension == "" {
		t.Error("expected tension to be set")
	}

	// Build diff positions from the real diff
	dp := NewDiffPosition(realWorldDiff)

	// Verify diff position mapping
	pos, ok := dp.Position("internal/handler/export.go", 24)
	if !ok {
		t.Fatal("expected position for export.go:24 (added line in diff)")
	}
	if pos <= 0 {
		t.Errorf("position should be positive, got %d", pos)
	}

	// Format as GitHub output
	output := FormatGitHubReview(result, "code", 2, dp)

	if output.Review.Event != GitHubComment {
		t.Errorf("review event = %s, want COMMENT", output.Review.Event)
	}

	// Should have 1 inline comment (ada's note with file:line ref)
	if len(output.Review.Comments) != 1 {
		t.Errorf("expected 1 inline comment, got %d", len(output.Review.Comments))
	} else {
		c := output.Review.Comments[0]
		if c.Path != "internal/handler/export.go" {
			t.Errorf("comment path = %q, want internal/handler/export.go", c.Path)
		}
		if c.Position <= 0 {
			t.Errorf("comment position should be positive, got %d", c.Position)
		}
		if !strings.Contains(c.Body, "**ada**") {
			t.Error("comment should attribute to ada")
		}
	}

	// Check run
	if output.CheckRun.Conclusion != "success" {
		t.Errorf("check conclusion = %s, want success (comment is not a failure)", output.CheckRun.Conclusion)
	}
	if !strings.Contains(output.CheckRun.Output.Title, "2 experts reviewed") {
		t.Errorf("check title = %q, should mention expert count", output.CheckRun.Output.Title)
	}

	// Verify JSON output is valid
	data, err := FormatGitHubJSON(output)
	if err != nil {
		t.Fatalf("FormatGitHubJSON failed: %v", err)
	}
	var parsed GitHubOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("output JSON is not valid: %v", err)
	}
}

// TestEndToEndBlockingReview verifies REQUEST_CHANGES flow.
func TestEndToEndBlockingReview(t *testing.T) {
	collectiveJSON := `{
		"verdict": "block",
		"blocking": true,
		"perspectives": [
			{
				"expert": "ada",
				"verdict": "block",
				"confidence": 0.95,
				"notes": ["internal/handler/export.go:12: SQL injection risk in query builder"],
				"blocking": true
			}
		],
		"agreements": [],
		"tension": "",
		"summary": "Security issue must be fixed."
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": collectiveJSON}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("GITHUB_TOKEN", "test-token")

	backend, err := newAPIBackendWithClient("github", "openai/gpt-4.1-mini", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	backend.SetBaseURL(server.URL)

	experts := testExperts()[:1]
	result, err := backend.ReviewCollective(context.Background(), experts, Submission{Content: realWorldDiff})
	if err != nil {
		t.Fatal(err)
	}

	dp := NewDiffPosition(realWorldDiff)
	output := FormatGitHubReview(result, "code", 1, dp)

	if output.Review.Event != GitHubRequestChanges {
		t.Errorf("event = %s, want REQUEST_CHANGES", output.Review.Event)
	}
	if output.CheckRun.Conclusion != "action_required" {
		t.Errorf("conclusion = %s, want action_required", output.CheckRun.Conclusion)
	}
}

// TestEndToEndChunkedReview verifies the diff splitting + merge pipeline.
// TestEndToEndLLMTimeout verifies error handling when the LLM is unreachable.
func TestEndToEndLLMTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"service unavailable"}`))
	}))
	defer server.Close()

	t.Setenv("GITHUB_TOKEN", "test-token")

	backend, err := newAPIBackendWithClient("github", "openai/gpt-4.1-mini", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	backend.SetBaseURL(server.URL)

	_, err = backend.ReviewCollective(context.Background(), testExperts(), Submission{Content: realWorldDiff})
	if err == nil {
		t.Fatal("expected error for 503 response")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention 503, got: %s", err.Error())
	}
}

// TestEndToEndMalformedLLMResponse verifies handling of invalid JSON from the LLM.
func TestEndToEndMalformedLLMResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "This is not JSON at all, just plain text review."}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("GITHUB_TOKEN", "test-token")

	backend, err := newAPIBackendWithClient("github", "openai/gpt-4.1-mini", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	backend.SetBaseURL(server.URL)

	// ReviewCollective should still return a result (parser has fallback logic)
	result, err := backend.ReviewCollective(context.Background(), testExperts(), Submission{Content: realWorldDiff})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The parser's fallback should produce something usable, not panic
	if result == nil {
		t.Fatal("result should not be nil even for malformed response")
	}
}

// TestDiffPositionRealWorldSamples covers 5 real-world diff patterns.
func TestDiffPositionRealWorldSamples(t *testing.T) {
	tests := []struct {
		name string
		diff string
		file string
		line int
		want int
		ok   bool
	}{
		{
			name: "simple addition",
			diff: `diff --git a/app.go b/app.go
--- a/app.go
+++ b/app.go
@@ -5,3 +5,4 @@
 func main() {
 	app := NewApp()
+	app.Run()
 }
`,
			file: "app.go", line: 7, want: 3, ok: true,
		},
		{
			name: "modified line",
			diff: `diff --git a/config.go b/config.go
--- a/config.go
+++ b/config.go
@@ -1,4 +1,4 @@
 package config
-const Version = "1.0.0"
+const Version = "1.1.0"
 const Name = "app"
`,
			file: "config.go", line: 2, want: 3, ok: true,
		},
		{
			name: "multi-hunk modification",
			diff: `diff --git a/server.go b/server.go
--- a/server.go
+++ b/server.go
@@ -3,4 +3,5 @@
 import "net/http"
+import "log"

 func serve() {
@@ -15,3 +16,4 @@
 func health() {
 	return "ok"
+	log.Println("health")
 }
`,
			file: "server.go", line: 18, want: 8, ok: true,
		},
		{
			name: "new file",
			diff: `diff --git a/middleware.go b/middleware.go
new file mode 100644
--- /dev/null
+++ b/middleware.go
@@ -0,0 +1,5 @@
+package main
+
+func auth(next http.Handler) http.Handler {
+	return next
+}
`,
			file: "middleware.go", line: 3, want: 3, ok: true,
		},
		{
			name: "line not in diff",
			diff: `diff --git a/utils.go b/utils.go
--- a/utils.go
+++ b/utils.go
@@ -10,3 +10,4 @@
 func helper() {
+	fmt.Println("debug")
 }
`,
			file: "utils.go", line: 1, want: 0, ok: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dp := NewDiffPosition(tt.diff)
			pos, ok := dp.Position(tt.file, tt.line)
			if ok != tt.ok || pos != tt.want {
				t.Errorf("Position(%q, %d) = (%d, %v), want (%d, %v)", tt.file, tt.line, pos, ok, tt.want, tt.ok)
			}
		})
	}
}
