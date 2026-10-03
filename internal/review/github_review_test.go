package review

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMapVerdictToEvent(t *testing.T) {
	tests := []struct {
		verdict  Verdict
		blocking bool
		want     GitHubReviewEvent
	}{
		{VerdictPass, false, GitHubApprove},
		{VerdictComment, false, GitHubComment},
		{VerdictBlock, false, GitHubRequestChanges},
		{VerdictEscalate, false, GitHubRequestChanges},
		{VerdictPass, true, GitHubRequestChanges},
		{VerdictComment, true, GitHubRequestChanges},
	}

	for _, tt := range tests {
		got := MapVerdictToEvent(tt.verdict, tt.blocking)
		if got != tt.want {
			t.Errorf("MapVerdictToEvent(%s, %v) = %s, want %s", tt.verdict, tt.blocking, got, tt.want)
		}
	}
}

func TestFormatGitHubReviewAllPass(t *testing.T) {
	result := &SynthesizedResult{
		Verdict: VerdictPass,
		Perspectives: []ExpertVerdict{
			{Expert: "ada", Verdict: VerdictPass, Notes: []string{"Clean code"}},
			{Expert: "kai", Verdict: VerdictPass, Notes: []string{"Ship it"}},
		},
		Agreements: []string{"Well-structured implementation"},
	}

	output := FormatGitHubReview(result, "rails", 2, nil)

	if output.Review.Event != GitHubApprove {
		t.Errorf("event = %s, want APPROVE", output.Review.Event)
	}
	if !strings.Contains(output.Review.Body, "**Votes:** 2 pass.") {
		t.Error("body should contain verdict")
	}
	if !strings.Contains(output.Review.Body, "2 pass") {
		t.Error("body should mention pass count")
	}
	if !strings.Contains(output.Review.Body, "Well-structured implementation") {
		t.Error("body should contain agreements")
	}
	if !strings.Contains(output.Review.Body, "pack: rails") {
		t.Error("body should contain pack name")
	}
	if output.CheckRun.Conclusion != "success" {
		t.Errorf("check conclusion = %s, want success", output.CheckRun.Conclusion)
	}
}

func TestFormatGitHubReviewAnyBlock(t *testing.T) {
	result := &SynthesizedResult{
		Verdict:  VerdictBlock,
		Blocking: true,
		Perspectives: []ExpertVerdict{
			{Expert: "sentinel", Verdict: VerdictBlock, Notes: []string{"SQL injection risk"}},
			{Expert: "ada", Verdict: VerdictPass, Notes: []string{"Code looks fine"}},
		},
		Tension: "sentinel sees security risk, ada disagrees",
	}

	output := FormatGitHubReview(result, "code", 2, nil)

	if output.Review.Event != GitHubRequestChanges {
		t.Errorf("event = %s, want REQUEST_CHANGES", output.Review.Event)
	}
	if !strings.Contains(output.Review.Body, "1 block") {
		t.Error("body should mention block count")
	}
	if !strings.Contains(output.Review.Body, "### Where they disagree") {
		t.Error("body should contain tension section")
	}
	if output.CheckRun.Conclusion != "action_required" {
		t.Errorf("check conclusion = %s, want action_required", output.CheckRun.Conclusion)
	}
}

func TestFormatGitHubReviewMixedComment(t *testing.T) {
	result := &SynthesizedResult{
		Verdict: VerdictComment,
		Perspectives: []ExpertVerdict{
			{Expert: "ada", Verdict: VerdictComment, Notes: []string{"Add test for empty state"}},
			{Expert: "kai", Verdict: VerdictPass},
		},
	}

	output := FormatGitHubReview(result, "", 2, nil)

	if output.Review.Event != GitHubComment {
		t.Errorf("event = %s, want COMMENT", output.Review.Event)
	}
	if !strings.Contains(output.Review.Body, "1 comment") {
		t.Error("body should mention comment count")
	}
}

func TestFormatGitHubReviewWithInlineComments(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,5 @@
 package main
+import "fmt"
+func run() { fmt.Println("hello") }
 func main() {}
`
	dp := NewDiffPosition(diff)

	result := &SynthesizedResult{
		Verdict: VerdictComment,
		Perspectives: []ExpertVerdict{
			{Expert: "ada", Verdict: VerdictComment, Notes: []string{"main.go:3: Consider extracting this into a separate function"}},
		},
	}

	output := FormatGitHubReview(result, "code", 1, dp)

	if len(output.Review.Comments) != 1 {
		t.Fatalf("expected 1 inline comment, got %d", len(output.Review.Comments))
	}
	c := output.Review.Comments[0]
	if c.Path != "main.go" {
		t.Errorf("comment path = %q, want main.go", c.Path)
	}
	if c.Position != 3 {
		t.Errorf("comment position = %d, want 3", c.Position)
	}
	if !strings.Contains(c.Body, "**ada**") {
		t.Error("comment should include expert attribution")
	}
	if !strings.Contains(c.Body, "extracting this") {
		t.Error("comment should include the note text")
	}

	if len(output.CheckRun.Output.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(output.CheckRun.Output.Annotations))
	}
	a := output.CheckRun.Output.Annotations[0]
	if a.Path != "main.go" || a.StartLine != 3 {
		t.Errorf("annotation = %s:%d, want main.go:3", a.Path, a.StartLine)
	}
	if a.AnnotationLevel != "warning" {
		t.Errorf("annotation level = %s, want warning", a.AnnotationLevel)
	}
}

func TestFormatGitHubReviewFallbackComment(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -5,3 +5,4 @@
 func main() {
+	doSomething()
 }
`
	dp := NewDiffPosition(diff)

	result := &SynthesizedResult{
		Verdict: VerdictComment,
		Perspectives: []ExpertVerdict{
			{Expert: "ada", Verdict: VerdictComment, Notes: []string{"main.go:1: Package-level comment missing"}},
		},
	}

	output := FormatGitHubReview(result, "", 1, dp)

	// Line 1 is not in the diff, so it should fall back to body
	if len(output.Review.Comments) != 0 {
		t.Errorf("expected 0 mapped comments (line not in diff), got %d", len(output.Review.Comments))
	}
	if !strings.Contains(output.Review.Body, "Additional Comments") {
		t.Error("body should contain fallback section")
	}
}

func TestFormatGitHubReviewPerspectivesTable(t *testing.T) {
	result := &SynthesizedResult{
		Verdict: VerdictComment,
		Perspectives: []ExpertVerdict{
			{Expert: "kent-beck", Verdict: VerdictComment, Notes: []string{"No test for empty-state CSV"}},
			{Expert: "sentinel-nyx", Verdict: VerdictPass},
			{Expert: "kai-westbrook", Verdict: VerdictComment, Notes: []string{"Skip the column picker for now"}},
		},
	}

	output := FormatGitHubReview(result, "rails", 3, nil)

	if !strings.Contains(output.Review.Body, "| kent-beck | comment |") {
		t.Error("body should contain perspectives table with ada")
	}
	if !strings.Contains(output.Review.Body, "| sentinel-nyx | pass | — |") {
		t.Error("body should contain sentinel row with dash for no notes")
	}
}

func TestFormatGitHubJSON(t *testing.T) {
	output := GitHubOutput{
		Review: GitHubReview{
			Event: GitHubApprove,
			Body:  "LGTM",
		},
		CheckRun: GitHubCheckRun{
			Name:       "Council Review",
			Status:     "completed",
			Conclusion: "success",
		},
	}

	data, err := FormatGitHubJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed GitHubOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if parsed.Review.Event != GitHubApprove {
		t.Errorf("parsed event = %s, want APPROVE", parsed.Review.Event)
	}
}

func TestParseNoteFileRef(t *testing.T) {
	tests := []struct {
		note     string
		wantFile string
		wantLine int
		wantText string
	}{
		{"main.go:42: Missing error check", "main.go", 42, "Missing error check"},
		{"src/handler.go:10 No validation", "src/handler.go", 10, "No validation"},
		{"No file reference here", "", 0, "No file reference here"},
		{"noext:42: something", "", 0, "noext:42: something"},
		{"file.go:abc: not a line", "", 0, "file.go:abc: not a line"},
	}

	for _, tt := range tests {
		file, line, text := parseNoteFileRef(tt.note)
		if file != tt.wantFile || line != tt.wantLine || text != tt.wantText {
			t.Errorf("parseNoteFileRef(%q) = (%q, %d, %q), want (%q, %d, %q)",
				tt.note, file, line, text, tt.wantFile, tt.wantLine, tt.wantText)
		}
	}
}

func TestFormatGitHubReviewCheckRunTitle(t *testing.T) {
	result := &SynthesizedResult{
		Verdict: VerdictPass,
		Perspectives: []ExpertVerdict{
			{Expert: "a", Verdict: VerdictPass},
			{Expert: "b", Verdict: VerdictPass},
			{Expert: "c", Verdict: VerdictPass},
		},
	}

	output := FormatGitHubReview(result, "code", 3, nil)

	if output.CheckRun.Name != "Council Review" {
		t.Errorf("check name = %q, want 'Council Review'", output.CheckRun.Name)
	}
	if !strings.Contains(output.CheckRun.Output.Title, "3 experts reviewed") {
		t.Errorf("check title = %q, should mention expert count", output.CheckRun.Output.Title)
	}
}
