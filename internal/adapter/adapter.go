// Package adapter provides tool-specific integrations for AI coding assistants.
// Each supported tool (Claude Code, OpenCode) implements the Adapter interface
// to handle its particular file formats and directory structures.
package adapter

import (
	"sort"
	"strconv"

	"github.com/luuuc/council/internal/expert"
	"github.com/luuuc/council/internal/fs"
)

// Adapter defines the interface for tool-specific behavior.
// Each AI tool (Claude Code, OpenCode, etc.) has its own adapter
// that handles paths, templates, and output formatting.
type Adapter interface {
	// Identity
	Name() string        // "claude", "opencode", "generic"
	DisplayName() string // "Claude Code", "OpenCode", "Generic"

	// Detection
	Detect() bool // Does this tool exist in current project?

	// Paths returns the directory structure for this tool
	Paths() Paths

	// Templates returns embedded templates for this tool
	Templates() Templates

	// Generation
	FormatAgent(e *expert.Expert) string
	FormatCommand(name, description, body string) string

	// CommandPath is where the named command or skill is written, e.g.
	// ".claude/skills/council/SKILL.md" or ".opencode/commands/council.md".
	CommandPath(name string) string
}

// Paths contains the directory structure for a tool
type Paths struct {
	Agents     string   // Directory for agent files (e.g., ".claude/agents")
	Deprecated []string // Old paths: files are removed on sync, folders with --clean
}

// CommandNames are the commands (or skills) every tool gets: /council and
// the ones from the adapter's templates.
var CommandNames = []string{"council", "council-assemble", "council-add", "council-remove"}

// skillPath is where a skill lives under the Agent Skills standard.
func skillPath(root, name string) string {
	return root + "/" + name + "/SKILL.md"
}

// skillFrontmatter starts a SKILL.md file. Skills with side effects are
// only run when the user asks for them.
func skillFrontmatter(name, description string) string {
	fm := "---\nname: " + name + "\ndescription: " + strconv.Quote(description) + "\n"
	if name == "council-remove" {
		fm += "disable-model-invocation: true\n"
	}
	return fm + "---\n\n"
}

// Templates contains embedded template content for a tool
type Templates struct {
	Commands map[string]string // name -> template content (e.g., "council-add" -> content)
}

// registry holds all registered adapters
var registry = make(map[string]Adapter)

// Register adds an adapter to the registry.
// Called by each adapter's init() function.
func Register(a Adapter) {
	registry[a.Name()] = a
}

// Get returns an adapter by name.
// Returns (adapter, true) if found, (nil, false) if not.
func Get(name string) (Adapter, bool) {
	a, ok := registry[name]
	return a, ok
}

// All returns a copy of all registered adapters.
// Returns a copy to prevent external mutation of the registry.
func All() map[string]Adapter {
	copy := make(map[string]Adapter, len(registry))
	for k, v := range registry {
		copy[k] = v
	}
	return copy
}

// Detect returns all adapters that detect their tool in the current project.
// Results are filtered to only include adapters where Detect() returns true.
// The generic adapter is excluded from detection (it's a fallback).
// Results are sorted by name for deterministic ordering.
func Detect() []Adapter {
	var detected []Adapter
	// Get sorted names for deterministic order
	names := Names()
	for _, name := range names {
		if name == "generic" {
			continue // Generic is always available, not detected
		}
		a := registry[name]
		if a.Detect() {
			detected = append(detected, a)
		}
	}
	return detected
}

// Names returns all registered adapter names in sorted order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResetRegistry clears the adapter registry.
// This is intended for testing purposes only to enable test isolation.
func ResetRegistry() {
	registry = make(map[string]Adapter)
}

func DirExists(path string) bool {
	return fs.DirExists(path)
}

func FileExists(path string) bool {
	return fs.FileExists(path)
}
