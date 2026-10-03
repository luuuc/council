package export

import (
	"strings"
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestFormatMarkdown_SingleExpert(t *testing.T) {
	experts := []*expert.Expert{
		{
			ID:         "ben",
			Name:       "Ben",
			Focus:      "Rails doctrine, conventions, architecture",
			Philosophy: "Convention over configuration is not just a technical choice.",
			Principles: []string{"RESTful routes only", "Fat models, skinny controllers"},
			RedFlags:   []string{"Service objects everywhere", "Abstract base classes"},
		},
	}

	result := FormatMarkdown(experts)

	// Check header
	if !strings.Contains(result, "# Expert Council") {
		t.Error("Missing main header")
	}

	if !strings.Contains(result, "Use these expert perspectives") {
		t.Error("Missing intro text")
	}

	// Check expert section
	if !strings.Contains(result, "## Ben") {
		t.Error("Missing expert header")
	}

	if !strings.Contains(result, "**Focus**: Rails doctrine") {
		t.Error("Missing focus")
	}

	if !strings.Contains(result, "Convention over configuration") {
		t.Error("Missing philosophy")
	}

	if !strings.Contains(result, "**Principles**:") {
		t.Error("Missing principles header")
	}

	if !strings.Contains(result, "- RESTful routes only") {
		t.Error("Missing principle")
	}

	if !strings.Contains(result, "**Watch for**:") {
		t.Error("Missing red flags header")
	}

	if !strings.Contains(result, "- Service objects everywhere") {
		t.Error("Missing red flag")
	}

	// Single expert should not have trailing separator
	if strings.HasSuffix(strings.TrimSpace(result), "---") {
		t.Error("Should not have trailing separator for single expert")
	}
}

func TestFormatMarkdown_MultipleExperts(t *testing.T) {
	experts := []*expert.Expert{
		{
			ID:    "ben",
			Name:  "Ben",
			Focus: "Rails",
		},
		{
			ID:    "kent",
			Name:  "Ada",
			Focus: "Testing",
		},
	}

	result := FormatMarkdown(experts)

	// Check both experts present
	if !strings.Contains(result, "## Ben") {
		t.Error("Missing first expert")
	}

	if !strings.Contains(result, "## Ada") {
		t.Error("Missing second expert")
	}

	// Check separator between experts
	if !strings.Contains(result, "---") {
		t.Error("Missing separator between experts")
	}

	// Check separator is between experts, not at end
	benIdx := strings.Index(result, "## Ben")
	kentIdx := strings.Index(result, "## Ada")
	sepIdx := strings.Index(result, "---")

	if sepIdx < benIdx || sepIdx > kentIdx {
		t.Error("Separator should be between experts")
	}
}

func TestFormatMarkdown_EmptyExperts(t *testing.T) {
	experts := []*expert.Expert{}

	result := FormatMarkdown(experts)

	// Should still have header
	if !strings.Contains(result, "# Expert Council") {
		t.Error("Missing main header even with no experts")
	}
}

func TestFormatMarkdown_MinimalExpert(t *testing.T) {
	experts := []*expert.Expert{
		{
			ID:    "minimal",
			Name:  "Minimal Expert",
			Focus: "Minimalism",
		},
	}

	result := FormatMarkdown(experts)

	if !strings.Contains(result, "## Minimal Expert") {
		t.Error("Missing expert name")
	}

	if !strings.Contains(result, "**Focus**: Minimalism") {
		t.Error("Missing focus")
	}

	// Should not contain optional sections
	if strings.Contains(result, "**Principles**:") {
		t.Error("Should not have principles section when empty")
	}

	if strings.Contains(result, "**Watch for**:") {
		t.Error("Should not have red flags section when empty")
	}
}
