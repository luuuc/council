package review

import (
	"strconv"
	"strings"
)

// DiffPosition maps a file path and line number to the diff-relative position
// required by the GitHub PR Reviews API.
type DiffPosition struct {
	positions map[string]map[int]int // file -> line -> position
}

// NewDiffPosition parses a unified diff and builds the position map.
// Only added and modified lines (lines starting with '+' that aren't
// the +++ header) get positions. Context and deleted lines are tracked
// for correct position counting but don't receive mappable positions.
func NewDiffPosition(diff string) *DiffPosition {
	dp := &DiffPosition{
		positions: make(map[string]map[int]int),
	}

	lines := strings.Split(diff, "\n")
	var currentFile string
	var position int // 1-based position relative to first @@ in this file
	var newLine int  // current line number in the new file
	var inHunk bool  // whether we've seen at least one @@ for this file

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			currentFile = parseDiffPath(line)
			position = 0
			newLine = 0
			inHunk = false
			continue
		}

		// Skip diff metadata lines that aren't part of the hunk
		if strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "old mode") ||
			strings.HasPrefix(line, "new mode") ||
			strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") ||
			strings.HasPrefix(line, "similarity index") ||
			strings.HasPrefix(line, "rename from") ||
			strings.HasPrefix(line, "rename to") ||
			strings.HasPrefix(line, "Binary files") {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			if inHunk {
				// Subsequent @@ lines count as a position
				position++
			}
			inHunk = true
			newLine = parseHunkNewStart(line)
			continue
		}

		if currentFile == "" {
			continue
		}

		if strings.HasPrefix(line, "+") {
			position++
			if dp.positions[currentFile] == nil {
				dp.positions[currentFile] = make(map[int]int)
			}
			dp.positions[currentFile][newLine] = position
			newLine++
		} else if strings.HasPrefix(line, "-") {
			position++
			// Deleted lines don't advance newLine
		} else {
			// Context line (space prefix or empty)
			position++
			newLine++
		}
	}

	return dp
}

// Position returns the diff-relative position for a file and line number.
// Returns (position, true) if the line is in the diff, (0, false) otherwise.
func (dp *DiffPosition) Position(file string, line int) (int, bool) {
	fileMap, ok := dp.positions[file]
	if !ok {
		return 0, false
	}
	pos, ok := fileMap[line]
	return pos, ok
}

// Files returns the list of files present in the diff.
func (dp *DiffPosition) Files() []string {
	files := make([]string, 0, len(dp.positions))
	for f := range dp.positions {
		files = append(files, f)
	}
	return files
}

// parseHunkNewStart extracts the new file start line from a hunk header.
// Format: @@ -old_start[,old_count] +new_start[,new_count] @@
func parseHunkNewStart(header string) int {
	// Find the +N part
	idx := strings.Index(header, "+")
	if idx < 0 {
		return 1
	}
	rest := header[idx+1:]
	// Take digits until comma or space
	end := strings.IndexAny(rest, ", @")
	if end < 0 {
		end = len(rest)
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 1
	}
	return n
}

// parseDiffPath extracts the file path from a "diff --git a/path b/path" line.
func parseDiffPath(diffLine string) string {
	// Format: "diff --git a/path/to/file b/path/to/file\n"
	diffLine = strings.TrimSpace(diffLine)
	// Split on the last " b/" to handle paths containing " b/" as a substring.
	idx := strings.LastIndex(diffLine, " b/")
	if idx >= 0 {
		return diffLine[idx+3:]
	}
	// Fallback: try to extract from a/ prefix
	parts := strings.SplitN(diffLine, " a/", 2)
	if len(parts) == 2 {
		return strings.SplitN(parts[1], " ", 2)[0]
	}
	return diffLine
}
