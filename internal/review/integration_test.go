package review

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

// TestEndToEndPRReview runs the unattended path: room prompt → model API →
// checked answer → GitHub PR review with an inline comment.
func TestEndToEndPRReview(t *testing.T) {
	answer := `{
		"reviews": [
			{"expert":"ada","verdict":"comment","confidence":0.8,"notes":["internal/handler/export.go:24: No error handling on writer.Write — CSV write errors are silently dropped"]},
			{"expert":"ben","verdict":"pass","confidence":0.9,"notes":["Ship it, the test covers the happy path"],"replies":[{"to":"ada","stance":"disagree","note":"a CSV write to an HTTP response won't fail in practice"}]}
		],
		"disagreements": [{"topic":"Handle writer.Write errors?","sides":[{"experts":["ada"],"position":"yes"},{"experts":["ben"],"position":"no need"}]}],
		"decisions": ["Add error handling now, or ship and watch?"]
	}`
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prompt = body.Messages[0].Content
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": answer}}},
		})
	}))
	defer server.Close()
	t.Setenv("GITHUB_TOKEN", "test-token")

	backend, err := newAPIBackendWithClient("github", "openai/gpt-4.1-mini", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	backend.SetBaseURL(server.URL)

	councils := []Council{roomCouncil("code", "ada", "ben")}
	text, err := backend.Complete(context.Background(), BuildRoomPrompt(councils, Submission{Content: realWorldDiff}))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(prompt, "writer.Flush()") {
		t.Error("the diff should be in the room prompt")
	}
	out, err := ParseRoom(text, councils)
	if err != nil {
		t.Fatalf("ParseRoom: %v", err)
	}
	result := out.Councils[0].Result

	output := FormatGitHubReview(result, "code", 2, NewDiffPosition(realWorldDiff))
	if output.Review.Event != GitHubComment {
		t.Errorf("review event = %s, want COMMENT", output.Review.Event)
	}
	if len(output.Review.Comments) != 1 {
		t.Fatalf("expected 1 inline comment, got %d", len(output.Review.Comments))
	}
	c := output.Review.Comments[0]
	if c.Path != "internal/handler/export.go" || c.Position <= 0 || !strings.Contains(c.Body, "Virtual Ada") {
		t.Errorf("inline comment = %+v", c)
	}
	if !strings.Contains(output.Review.Body, "Add error handling now, or ship and watch?") {
		t.Error("the review body should carry the decisions")
	}
	if output.CheckRun.Conclusion != "success" {
		t.Errorf("check conclusion = %s, want success (comment is not a failure)", output.CheckRun.Conclusion)
	}
	if _, err := FormatGitHubJSON(output); err != nil {
		t.Fatalf("FormatGitHubJSON: %v", err)
	}
}

// TestEndToEndBlockingReview: a blocking member's block requests changes.
func TestEndToEndBlockingReview(t *testing.T) {
	councils := []Council{roomCouncil("code", "ada", "ben")}
	councils[0].Inputs[0].Blocking = true
	out, err := ParseRoom(`{"reviews":[
		{"expert":"ada","verdict":"block","notes":["SQL injection in the export query"]},
		{"expert":"ben","verdict":"pass","notes":["fine by me"]}]}`, councils)
	if err != nil {
		t.Fatal(err)
	}
	output := FormatGitHubReview(out.Councils[0].Result, "code", 2, nil)
	if output.Review.Event != GitHubRequestChanges || output.CheckRun.Conclusion != "action_required" {
		t.Errorf("event = %s, conclusion = %s; want REQUEST_CHANGES, action_required", output.Review.Event, output.CheckRun.Conclusion)
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
