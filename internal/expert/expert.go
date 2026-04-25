// Package expert manages expert persona files stored as markdown with YAML frontmatter.
package expert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/luuuc/council/internal/config"
	"gopkg.in/yaml.v3"
)

// Pre-compiled regex for ID generation
var idRegexp = regexp.MustCompile(`[^a-z0-9]+`)

// Tension represents a known disagreement between two experts.
type Tension struct {
	Expert       string `yaml:"expert" json:"expert"`
	Topic        string `yaml:"topic" json:"topic"`
	Position     string `yaml:"position" json:"position"`
	Counterpoint string `yaml:"counterpoint" json:"counterpoint"`
}

// Pre-compiled template for expert body generation
var bodyTemplate = template.Must(template.New("body").Parse(`# {{.Name}} - {{.Focus}}

You are {{.Name}}, known for expertise in {{.Focus}}.
{{- if .Backstory}}

{{.Backstory}}
{{end}}
{{if .Philosophy}}## Philosophy

{{.Philosophy}}
{{end}}
{{if .Principles}}## Principles

{{range .Principles}}- {{.}}
{{end}}
{{end}}
{{if .RedFlags}}## Red Flags

Watch for these patterns:
{{range .RedFlags}}- {{.}}
{{end}}
{{end}}
{{if .Tensions}}## Tensions

{{range .Tensions}}- vs {{.Expert}} on {{.Topic}}: {{.Position}} (counterpoint: {{.Counterpoint}})
{{end}}
{{end}}## Review Style

When reviewing code, focus on your area of expertise. Be direct and specific.
Explain your reasoning. Suggest concrete improvements.
`))

// Expert represents an expert persona.
// This is the canonical type used throughout the codebase for both
// project experts and custom/installed personas.
type Expert struct {
	ID         string    `yaml:"id" json:"id"`
	Name       string    `yaml:"name" json:"name"`
	Focus      string    `yaml:"focus" json:"focus"`
	Influences []string  `yaml:"influences,omitempty" json:"influences,omitempty"`
	Backstory  string    `yaml:"backstory,omitempty" json:"backstory,omitempty"`
	Philosophy string    `yaml:"philosophy,omitempty" json:"philosophy,omitempty"`
	Principles []string  `yaml:"principles,omitempty" json:"principles,omitempty"`
	RedFlags   []string  `yaml:"red_flags,omitempty" json:"red_flags,omitempty"`
	Tensions   []Tension `yaml:"tensions,omitempty" json:"tensions,omitempty"`

	// Suggestion metadata
	Core     bool     `yaml:"core,omitempty" json:"-"`     // Always suggest for matching intention
	Triggers []string `yaml:"triggers,omitempty" json:"-"` // Only suggest when patterns detected

	// Personal council metadata (used by creator commands)
	Category string `yaml:"category,omitempty" json:"category,omitempty"` // e.g., "custom", "rails", "go"
	Priority string `yaml:"priority,omitempty" json:"priority,omitempty"` // "always", "high", "normal"

	// Body is the markdown content after frontmatter
	Body string `yaml:"-" json:"-"`

	// Source indicates where this expert came from: "", "custom", or "installed:<name>"
	Source string `yaml:"-" json:"-"`
}

// ExpertSuggestions is the expected AI response format
type ExpertSuggestions struct {
	Experts []Expert `yaml:"experts"`
}

// ListResult contains the result of listing experts, including any warnings
type ListResult struct {
	Experts  []*Expert
	Warnings []string
}

// ApplyDefaults sets default values for optional fields.
func (e *Expert) ApplyDefaults() {
	if e.Category == "" {
		e.Category = "custom"
	}
	if e.Priority == "" {
		e.Priority = "normal"
	}
}

// MarshalExpertsJSON marshals a list of experts to JSON
func MarshalExpertsJSON(experts []*Expert) ([]byte, error) {
	return json.MarshalIndent(experts, "", "  ")
}

func (e *Expert) Path() string {
	return config.Path(config.ExpertsDir, e.ID+".md")
}

// Save writes the expert to disk
func (e *Expert) Save() error {
	return SaveToPath(e, e.Path())
}

func (e *Expert) generateBody() string {
	var buf bytes.Buffer
	if err := bodyTemplate.Execute(&buf, e); err != nil {
		// Fallback to simple format if template fails
		return fmt.Sprintf("# %s - %s\n\nExpert in %s.", e.Name, e.Focus, e.Focus)
	}
	return strings.TrimSpace(buf.String())
}

// Load reads an expert from disk
func Load(id string) (*Expert, error) {
	id, _ = LegacyAlias(id)
	path := config.Path(config.ExpertsDir, id+".md")
	return LoadFile(path)
}

// LoadFile reads an expert from a specific file
func LoadFile(path string) (*Expert, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return Parse(data)
}

// Parse parses expert markdown with frontmatter
func Parse(data []byte) (*Expert, error) {
	content := string(data)

	// Split frontmatter and body
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing frontmatter: file must start with '---'")
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid frontmatter: missing closing '---'")
	}

	frontmatter := strings.TrimSpace(parts[0])
	body := strings.TrimSpace(parts[1])

	var e Expert
	if err := yaml.Unmarshal([]byte(frontmatter), &e); err != nil {
		return nil, formatYAMLError(frontmatter, err)
	}

	e.Body = body
	return &e, nil
}

// formatYAMLError provides helpful context for YAML parsing errors.
// Design decision: This function is intentionally verbose (~45 lines) because
// the enhanced error messages with line context and hints significantly improve
// the user experience when debugging malformed expert files. The UX benefit
// justifies the code complexity. If YAML error formatting is needed elsewhere,
// consider extracting to internal/yamlutil/error.go.
func formatYAMLError(content string, err error) error {
	errStr := err.Error()
	lines := strings.Split(content, "\n")

	// Try to extract line number from yaml error (format: "yaml: line N: ...")
	if strings.Contains(errStr, "line") {
		// Parse line number
		var lineNum int
		if _, scanErr := fmt.Sscanf(errStr, "yaml: line %d:", &lineNum); scanErr == nil && lineNum > 0 && lineNum <= len(lines) {
			// Show context around the error
			start := lineNum - 2
			if start < 0 {
				start = 0
			}
			end := lineNum + 1
			if end > len(lines) {
				end = len(lines)
			}

			var context strings.Builder
			fmt.Fprintf(&context, "YAML error at line %d:\n\n", lineNum)
			for i := start; i < end; i++ {
				marker := "  "
				if i == lineNum-1 {
					marker = "> "
				}
				fmt.Fprintf(&context, "  %s%d: %s\n", marker, i+1, lines[i])
			}
			fmt.Fprintf(&context, "\nError: %s", errStr)

			// Add common fix suggestions
			if strings.Contains(errStr, "did not find expected") {
				context.WriteString("\n\nHint: Check for:\n")
				context.WriteString("  - Missing or extra spaces in indentation\n")
				context.WriteString("  - Special characters that need quoting (: @ # etc)\n")
				context.WriteString("  - Missing dash (-) for list items\n")
			}

			return fmt.Errorf("%s", context.String())
		}
	}

	// Fallback to original error with generic hint
	return fmt.Errorf("failed to parse YAML: %w\n\nHint: Check indentation and special characters", err)
}

// List returns all experts in the council
func List() ([]*Expert, error) {
	result, err := ListWithWarnings()
	if err != nil {
		return nil, err
	}
	return result.Experts, nil
}

// ListWithWarnings returns all experts with any warnings about files that couldn't be loaded
func ListWithWarnings() (*ListResult, error) {
	dir := config.Path(config.ExpertsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &ListResult{}, nil
		}
		return nil, err
	}

	result := &ListResult{
		Experts:  []*Expert{},
		Warnings: []string{},
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		e, err := LoadFile(path)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not load %s: %v", entry.Name(), err))
			continue
		}
		result.Experts = append(result.Experts, e)
	}

	return result, nil
}

// Delete removes an expert from the council
func Delete(id string) error {
	path := config.Path(config.ExpertsDir, id+".md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("expert '%s' not found - run 'council list' to see available experts", id)
	}
	return os.Remove(path)
}

// Exists checks if an expert exists
func Exists(id string) bool {
	id, _ = LegacyAlias(id)
	path := config.Path(config.ExpertsDir, id+".md")
	_, err := os.Stat(path)
	return err == nil
}

// ToID converts a name to a kebab-case ID
func ToID(name string) string {
	// Convert to lowercase
	id := strings.ToLower(name)

	// Replace spaces and special chars with hyphens
	id = idRegexp.ReplaceAllString(id, "-")

	// Remove leading/trailing hyphens
	id = strings.Trim(id, "-")

	return id
}

// ParseAIResponse parses YAML response from AI into experts
func ParseAIResponse(data []byte) ([]Expert, error) {
	// Try to extract YAML from markdown code block
	content := string(data)
	if idx := strings.Index(content, "```yaml"); idx >= 0 {
		content = content[idx+7:]
		if end := strings.Index(content, "```"); end >= 0 {
			content = content[:end]
		}
	} else if idx := strings.Index(content, "```"); idx >= 0 {
		content = content[idx+3:]
		if end := strings.Index(content, "```"); end >= 0 {
			content = content[:end]
		}
	}

	var suggestions ExpertSuggestions
	if err := yaml.Unmarshal([]byte(content), &suggestions); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return suggestions.Experts, nil
}

// SourceMarker returns the display marker for an expert's source
func (e *Expert) SourceMarker() string {
	switch {
	case e.Source == "custom":
		return " [custom]"
	case strings.HasPrefix(e.Source, "installed:"):
		return " [" + e.Source + "]"
	default:
		return ""
	}
}

// ParseFrontmatter parses just the YAML frontmatter into an Expert.
// Unlike Parse, this expects only the YAML content without the --- delimiters.
func ParseFrontmatter(data []byte) (*Expert, error) {
	var e Expert
	if err := yaml.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// SaveToPath writes the expert to a specific file path.
func SaveToPath(e *Expert, path string) error {
	// Generate body if empty
	if e.Body == "" {
		e.Body = e.generateBody()
	}

	// Generate frontmatter
	fm, err := yaml.Marshal(e)
	if err != nil {
		return fmt.Errorf("failed to marshal expert: %w", err)
	}

	// Combine frontmatter and body
	content := fmt.Sprintf("---\n%s---\n\n%s", string(fm), e.Body)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write expert file: %w", err)
	}

	// Verify round-trip: ensure the saved file can be parsed back
	loaded, err := LoadFile(path)
	if err != nil {
		// Clean up the bad file
		_ = os.Remove(path)
		return fmt.Errorf("saved file is invalid: %w", err)
	}

	// Verify key fields match
	if loaded.ID != e.ID || loaded.Name != e.Name {
		_ = os.Remove(path)
		return fmt.Errorf("saved file has corrupted data: id or name mismatch")
	}

	return nil
}

