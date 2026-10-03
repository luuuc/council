package review

import (
	"testing"
)

func TestDiffPositionSingleHunk(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main

+import "fmt"
 func main() {
-	println("hello")
+	fmt.Println("hello")
 }
`

	dp := NewDiffPosition(diff)

	tests := []struct {
		file string
		line int
		pos  int
		ok   bool
	}{
		{"main.go", 3, 3, true},  // +import "fmt"
		{"main.go", 5, 6, true},  // +fmt.Println("hello")
		{"main.go", 1, 0, false}, // context line: "package main" — not an added line
		{"main.go", 99, 0, false},
		{"other.go", 1, 0, false},
	}

	for _, tt := range tests {
		pos, ok := dp.Position(tt.file, tt.line)
		if ok != tt.ok || pos != tt.pos {
			t.Errorf("Position(%q, %d) = (%d, %v), want (%d, %v)", tt.file, tt.line, pos, ok, tt.pos, tt.ok)
		}
	}
}

func TestDiffPositionMultiHunk(t *testing.T) {
	diff := `diff --git a/handler.go b/handler.go
--- a/handler.go
+++ b/handler.go
@@ -5,6 +5,7 @@
 import "net/http"

+import "log"
 func handler(w http.ResponseWriter, r *http.Request) {
 	w.WriteHeader(200)
 }
@@ -20,4 +21,5 @@
 func health(w http.ResponseWriter, r *http.Request) {
 	w.WriteHeader(200)
+	log.Println("health check")
 }
`

	dp := NewDiffPosition(diff)

	tests := []struct {
		file string
		line int
		pos  int
		ok   bool
	}{
		{"handler.go", 7, 3, true},   // +import "log" (first hunk, position 3)
		{"handler.go", 23, 10, true},  // +log.Println (second hunk)
	}

	for _, tt := range tests {
		pos, ok := dp.Position(tt.file, tt.line)
		if ok != tt.ok || pos != tt.pos {
			t.Errorf("Position(%q, %d) = (%d, %v), want (%d, %v)", tt.file, tt.line, pos, ok, tt.pos, tt.ok)
		}
	}
}

func TestDiffPositionMultipleFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,3 +1,4 @@
 package a
+func A() {}

diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,3 +1,4 @@
 package b
+func B() {}

`

	dp := NewDiffPosition(diff)

	posA, okA := dp.Position("a.go", 2)
	posB, okB := dp.Position("b.go", 2)

	if !okA || posA != 2 {
		t.Errorf("a.go:2 = (%d, %v), want (2, true)", posA, okA)
	}
	if !okB || posB != 2 {
		t.Errorf("b.go:2 = (%d, %v), want (2, true)", posB, okB)
	}
}

func TestDiffPositionNewFile(t *testing.T) {
	diff := `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.go
@@ -0,0 +1,3 @@
+package new
+
+func New() {}
`

	dp := NewDiffPosition(diff)

	pos1, ok1 := dp.Position("new.go", 1)
	pos3, ok3 := dp.Position("new.go", 3)

	if !ok1 || pos1 != 1 {
		t.Errorf("new.go:1 = (%d, %v), want (1, true)", pos1, ok1)
	}
	if !ok3 || pos3 != 3 {
		t.Errorf("new.go:3 = (%d, %v), want (3, true)", pos3, ok3)
	}
}

func TestDiffPositionDeletedLine(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,4 +1,3 @@
 package main
-import "unused"
 func main() {
 }
`

	dp := NewDiffPosition(diff)

	// Deleted lines don't produce positions for new file lines
	_, ok := dp.Position("main.go", 2)
	if ok {
		t.Error("deleted line should not have a position in new file")
	}
}

func TestDiffPositionFiles(t *testing.T) {
	diff := `diff --git a/x.go b/x.go
--- a/x.go
+++ b/x.go
@@ -1 +1,2 @@
+new line
diff --git a/y.go b/y.go
--- a/y.go
+++ b/y.go
@@ -1 +1,2 @@
+another line
`

	dp := NewDiffPosition(diff)
	files := dp.Files()
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestDiffPositionEmptyDiff(t *testing.T) {
	dp := NewDiffPosition("")
	_, ok := dp.Position("anything.go", 1)
	if ok {
		t.Error("empty diff should have no positions")
	}
}

func TestParseHunkNewStart(t *testing.T) {
	tests := []struct {
		header string
		want   int
	}{
		{"@@ -1,5 +1,6 @@", 1},
		{"@@ -20,4 +21,5 @@", 21},
		{"@@ -0,0 +1,3 @@", 1},
		{"@@ -100 +200,10 @@ func foo()", 200},
	}
	for _, tt := range tests {
		got := parseHunkNewStart(tt.header)
		if got != tt.want {
			t.Errorf("parseHunkNewStart(%q) = %d, want %d", tt.header, got, tt.want)
		}
	}
}

func TestParseDiffPath(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"diff --git a/src/main.go b/src/main.go", "src/main.go"},
		{"diff --git a/README.md b/README.md", "README.md"},
		{"diff --git a/a/b/c.txt b/a/b/c.txt", "a/b/c.txt"},
	}
	for _, tt := range tests {
		got := parseDiffPath(tt.line)
		if got != tt.want {
			t.Errorf("parseDiffPath(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestParseDiffPathEdgeCases(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"diff --git a/src/main.go b/src/main.go", "src/main.go"},
		{"diff --git a/README.md b/README.md", "README.md"},
		{"diff --git a/deep/nested/path/file.go b/deep/nested/path/file.go", "deep/nested/path/file.go"},
		{"diff --git a/b/file.go b/b/file.go", "b/file.go"},
	}
	for _, tt := range tests {
		got := parseDiffPath(tt.line)
		if got != tt.want {
			t.Errorf("parseDiffPath(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}
