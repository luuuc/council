package review

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/luuuc/council/internal/config"
)

var idChars = regexp.MustCompile(`[^a-z0-9]+`)

// ReviewsDir is where recorded reviews are kept, under .council/.
const ReviewsDir = "reviews"

// Save writes a rendered review to .council/reviews/ so the human can look
// back at it, and returns the file's path. label names the council(s).
func Save(text, label string, now time.Time) (string, error) {
	dir := config.Path(ReviewsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create reviews folder: %w", err)
	}
	name := now.Format("2006-01-02-150405")
	if label = strings.Trim(idChars.ReplaceAllString(strings.ToLower(label), "-"), "-"); label != "" {
		name += "-" + label
	}
	path := config.Path(ReviewsDir, name+".txt")
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return "", fmt.Errorf("save review: %w", err)
	}
	return path, nil
}
