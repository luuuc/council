// Package pack manages reusable expert groupings for council reviews.
package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/luuuc/council-cli/internal/config"
	"gopkg.in/yaml.v3"
)

// Member represents an expert in a pack with their blocking status.
type Member struct {
	ID       string `yaml:"id" json:"id"`
	Blocking bool   `yaml:"blocking,omitempty" json:"blocking,omitempty"`
}

// Pack represents a reusable group of experts.
type Pack struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Members     []Member `yaml:"members" json:"members"`
	Source      string   `yaml:"-" json:"source,omitempty"` // "builtin" or ""
}

// Validate checks that a pack has required fields.
func (p *Pack) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("pack name is required")
	}
	if strings.ContainsAny(p.Name, " /\\") {
		return fmt.Errorf("pack name must not contain spaces or slashes")
	}
	return nil
}

// HasMember checks if an expert ID is already in the pack.
func (p *Pack) HasMember(id string) bool {
	for _, m := range p.Members {
		if m.ID == id {
			return true
		}
	}
	return false
}

// AddMember adds an expert to the pack. Returns an error if already present.
func (p *Pack) AddMember(id string, blocking bool) error {
	if p.HasMember(id) {
		return fmt.Errorf("expert '%s' is already in pack '%s'", id, p.Name)
	}
	p.Members = append(p.Members, Member{ID: id, Blocking: blocking})
	return nil
}

// RemoveMember removes an expert from the pack. Returns an error if not found.
func (p *Pack) RemoveMember(id string) error {
	for i, m := range p.Members {
		if m.ID == id {
			p.Members = append(p.Members[:i], p.Members[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("expert '%s' not found in pack '%s'", id, p.Name)
}

// Parse parses a pack from YAML bytes.
func Parse(data []byte) (*Pack, error) {
	var p Pack
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse pack YAML: %w", err)
	}
	return &p, nil
}

// path returns the file path for a custom pack.
func path(name string) string {
	return config.Path(config.PacksDir, name+".yaml")
}

// Load reads a custom pack from .council/packs/<name>.yaml.
func Load(name string) (*Pack, error) {
	data, err := os.ReadFile(path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("pack '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read pack: %w", err)
	}
	return Parse(data)
}

// Save writes a pack to .council/packs/<name>.yaml.
func Save(p *Pack) error {
	if err := p.Validate(); err != nil {
		return err
	}

	dir := config.Path(config.PacksDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create packs directory: %w", err)
	}

	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal pack: %w", err)
	}

	if err := os.WriteFile(path(p.Name), data, 0644); err != nil {
		return fmt.Errorf("failed to write pack: %w", err)
	}

	return nil
}

// Delete removes a custom pack file.
func Delete(name string) error {
	p := path(name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return fmt.Errorf("pack '%s' not found", name)
	}
	return os.Remove(p)
}

// ListResult contains the result of listing packs, including any warnings.
type ListResult struct {
	Packs    []*Pack
	Warnings []string
}

// List returns all custom packs from .council/packs/.
func List() ([]*Pack, error) {
	result, err := ListWithWarnings()
	if err != nil {
		return nil, err
	}
	return result.Packs, nil
}

// ListWithWarnings returns all custom packs with any warnings about files that couldn't be loaded.
func ListWithWarnings() (*ListResult, error) {
	dir := config.Path(config.PacksDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &ListResult{}, nil
		}
		return nil, fmt.Errorf("failed to read packs directory: %w", err)
	}

	result := &ListResult{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", entry.Name(), err))
			continue
		}

		p, err := Parse(data)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse %s: %v", entry.Name(), err))
			continue
		}

		result.Packs = append(result.Packs, p)
	}

	return result, nil
}
