package provider

import "fmt"

// Factory creates a Provider from a data directory (where secrets.toml lives).
type Factory func(dataDir string) (Provider, error)

var registry = map[string]Factory{}

// Register adds a provider factory. Called from adapter package init().
func Register(name string, f Factory) {
	registry[name] = f
}

// Get returns a provider by name, constructing it from the given data directory.
func Get(name, dataDir string) (Provider, error) {
	if name == "" {
		name = "composio"
	}
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q (registered: %v)", name, Names())
	}
	return f(dataDir)
}

// Names returns all registered provider names.
func Names() []string {
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	return names
}
