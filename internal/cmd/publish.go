package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/luuuc/council-cli/internal/config"
	"github.com/luuuc/council-cli/internal/expert"
	"github.com/spf13/cobra"
)

var publishAll bool

func init() {
	rootCmd.AddCommand(publishCmd)
	publishCmd.Flags().BoolVar(&publishAll, "all", false, "Include all personas (not just custom)")
}

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Export personas for sharing",
	Long: `Creates a git-ready directory with your custom personas.

By default, only exports custom personas (not curated library ones).
Use --all to include all personas in your council.

Output structure:
  council-personas/
  ├── README.md           # Auto-generated index with install instructions
  ├── my-cto.md
  └── custom-persona.md

After publishing:
  1. Push to GitHub: git add council-personas/ && git commit && git push
  2. Share install URL: council install user/repo/council-personas/my-cto`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !config.Exists() {
			return fmt.Errorf("council not initialized: run 'council start' first")
		}

		return runPublish(publishAll)
	},
}

func runPublish(includeAll bool) error {
	experts, err := expert.List()
	if err != nil {
		return fmt.Errorf("failed to load experts: %w", err)
	}

	if len(experts) == 0 {
		return fmt.Errorf("no experts to publish - add some with 'council add' first")
	}

	// Filter to custom personas unless --all
	var toPublish []*expert.Expert
	if includeAll {
		toPublish = experts
	} else {
		toPublish = filterCustomExperts(experts)
	}

	if len(toPublish) == 0 {
		return fmt.Errorf("no custom personas to publish\n\nYour council only contains curated library personas.\nUse 'council publish --all' to include them, or\ncreate custom personas with 'council add \"Name\"'")
	}

	// Create output directory
	outputDir := "council-personas"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Copy expert files
	for _, e := range toPublish {
		srcPath := e.Path()
		dstPath := filepath.Join(outputDir, e.ID+".md")

		if err := copyFile(srcPath, dstPath); err != nil {
			fmt.Printf("Warning: could not copy %s: %v\n", e.ID, err)
			continue
		}
	}

	// Generate README
	readme := generatePublishReadme(toPublish)
	readmePath := filepath.Join(outputDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	fmt.Printf("Published %d personas to %s/\n", len(toPublish), outputDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  git add council-personas/")
	fmt.Println("  git commit -m 'Add council personas'")
	fmt.Println("  git push")

	return nil
}

// filterCustomExperts returns experts that are not from the curated library.
// A custom expert is one whose ID is not found in the suggestion bank.
func filterCustomExperts(experts []*expert.Expert) []*expert.Expert {
	var custom []*expert.Expert
	for _, e := range experts {
		if !isFromCuratedLibrary(e.ID) {
			custom = append(custom, e)
		}
	}
	return custom
}

// isFromCuratedLibrary checks if an expert ID exists in the suggestion bank.
func isFromCuratedLibrary(id string) bool {
	for _, experts := range loadSuggestionBank() {
		for _, e := range experts {
			if e.ID == id {
				return true
			}
		}
	}
	return false
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// generatePublishReadme creates a README.md for the published personas.
func generatePublishReadme(experts []*expert.Expert) string {
	var sb strings.Builder

	sb.WriteString("# Council Personas\n\n")
	sb.WriteString("Expert personas for [council-cli](https://github.com/luuuc/council-cli).\n\n")

	sb.WriteString("## Install\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("council install <raw-url>\n")
	sb.WriteString("```\n\n")

	sb.WriteString("Or using GitHub shorthand:\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("council install user/repo/council-personas/<persona-id>\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Personas\n\n")
	sb.WriteString("| Name | Focus |\n")
	sb.WriteString("|------|-------|\n")

	for _, e := range experts {
		sb.WriteString(fmt.Sprintf("| [%s](%s.md) | %s |\n", e.Name, e.ID, e.Focus))
	}

	return sb.String()
}
