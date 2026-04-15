package pack

import (
	_ "embed"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed builtins.yaml
var builtinsYAML []byte

var (
	builtinPacks     map[string]*Pack
	builtinPacksOnce sync.Once
)

// Builtins returns the built-in packs, lazily loaded from embedded YAML.
func Builtins() map[string]*Pack {
	builtinPacksOnce.Do(func() {
		// Parse as map of raw pack data (yaml tags match Pack struct)
		var raw map[string]Pack
		if err := yaml.Unmarshal(builtinsYAML, &raw); err != nil {
			panic(fmt.Sprintf("failed to parse builtins.yaml: %v", err))
		}
		builtinPacks = make(map[string]*Pack, len(raw))
		for name, p := range raw {
			p.Source = "builtin"
			builtinPacks[name] = &p
		}
	})
	return builtinPacks
}
