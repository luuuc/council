package cmd

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(installCmd)
}

var installCmd = &cobra.Command{
	Use:   "install <url>",
	Short: "Install a persona from a URL",
	Long: `Fetches a persona from any URL and adds it to your council.

Works with any public URL hosting a valid persona markdown file:
  - GitHub raw URLs
  - Gist raw URLs
  - Any public HTTP/HTTPS URL

The file must be valid persona markdown with YAML frontmatter containing
at least: id, name, and focus fields.

GitHub Shorthand:
  user/repo/path expands to raw.githubusercontent.com URL.
  Note: Shorthand assumes the 'main' branch. For repos using 'master' or
  other branches, use the full raw URL instead.

Examples:
  council install https://raw.githubusercontent.com/user/repo/main/expert.md
  council install https://gist.githubusercontent.com/user/abc123/raw/expert.md
  council install user/repo/expert    # GitHub shorthand (assumes main branch)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council init' first")
		}

		return runInstall(args[0])
	},
}

// maxPersonaSize is the maximum size of a persona file (1MB).
// This prevents memory exhaustion from malicious or misconfigured URLs.
const maxPersonaSize = 1 << 20 // 1MB

func runInstall(input string) error {
	url := expandGitHubShorthand(input)

	// Fetch the URL
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch %s: HTTP %d", url, resp.StatusCode)
	}

	// Read body with size limit to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, maxPersonaSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if len(body) > maxPersonaSize {
		return fmt.Errorf("persona file too large (max %d bytes)", maxPersonaSize)
	}

	// Parse as expert (validates frontmatter)
	e, err := expert.Parse(body)
	if err != nil {
		return fmt.Errorf("invalid persona file: %w", err)
	}

	// Validate required fields
	if e.ID == "" {
		return fmt.Errorf("invalid persona: missing 'id' field in frontmatter")
	}
	if e.Name == "" {
		return fmt.Errorf("invalid persona: missing 'name' field in frontmatter")
	}
	if e.Focus == "" {
		return fmt.Errorf("invalid persona: missing 'focus' field in frontmatter")
	}

	// Check if already exists
	if expert.Exists(e.ID) {
		return fmt.Errorf("expert '%s' already exists - remove it first with 'council remove %s'", e.ID, e.ID)
	}

	// Save to .council/experts/
	if err := e.Save(); err != nil {
		return fmt.Errorf("failed to save persona: %w", err)
	}

	fmt.Printf("Installed: %s\n", e.Name)
	fmt.Printf("  Focus: %s\n", e.Focus)
	runAutoSync(false, nil)

	return nil
}

// expandGitHubShorthand expands user/repo/path to raw GitHub URL.
// If input looks like a URL, returns as-is.
func expandGitHubShorthand(input string) string {
	// If it looks like a URL, return as-is
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return input
	}

	// Try to parse as user/repo/path
	parts := strings.Split(input, "/")
	if len(parts) >= 3 {
		user := parts[0]
		repo := parts[1]
		path := strings.Join(parts[2:], "/")
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", user, repo, path)
	}

	// Not a shorthand, return as-is (will fail on HTTP GET)
	return input
}
