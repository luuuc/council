package adapter_test

import (
	"fmt"

	"github.com/luuuc/council/internal/adapter"
)

func ExampleGet() {
	// Get retrieves an adapter by name
	claude, ok := adapter.Get("claude")
	if !ok {
		fmt.Println("Claude adapter not found")
		return
	}

	fmt.Printf("Name: %s\n", claude.Name())
	fmt.Printf("Display: %s\n", claude.DisplayName())
	// Output:
	// Name: claude
	// Display: Claude Code
}

func ExampleNames() {
	// Names returns all registered adapter names in sorted order
	names := adapter.Names()
	for _, name := range names {
		fmt.Println(name)
	}
	// Output:
	// claude
	// generic
	// opencode
}

func ExampleAdapter_Paths() {
	claude, _ := adapter.Get("claude")
	paths := claude.Paths()

	fmt.Printf("Agents: %s\n", paths.Agents)
	fmt.Printf("Council skill: %s\n", claude.CommandPath("council"))
	// Output:
	// Agents: .claude/agents
	// Council skill: .claude/skills/council/SKILL.md
}
