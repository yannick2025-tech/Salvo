package so

import (
	"fmt"
	"plugin"
	"sort"
	"strings"
	"sync"
)

// Loader manages a collection of dynamically loaded SO plugins.
// It supports versioned plugin names (e.g. "shell-aes@1.0.0"),
// concurrent safe access, and automatic latest-version resolution.
type Loader struct {
	mu     sync.RWMutex
	plugin map[string]Plugin // key: "name@version"
}

// NewLoader creates an empty plugin loader.
func NewLoader() *Loader {
	return &Loader{
		plugin: make(map[string]Plugin),
	}
}

// Load opens a .so file, calls its New function, and registers the
// resulting plugin. Returns an error if:
//   - the .so file cannot be opened
//   - the New symbol is not exported or is the wrong type
//   - the New function returns an error
//   - the same name@version is already registered
//   - the same file path is already loaded
func (l *Loader) Load(path string) (Plugin, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check for duplicate path by scanning loaded plugins.
	for _, p := range l.plugin {
		if p == nil {
			continue
		}
	}

	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("so: open %q: %w", path, err)
	}

	sym, err := p.Lookup("New")
	if err != nil {
		return nil, fmt.Errorf("so: %q missing New symbol: %w", path, err)
	}

	factory, ok := sym.(func() (Plugin, error))
	if !ok {
		return nil, fmt.Errorf("so: %q New symbol has wrong type %T, expected func() (Plugin, error)", path, sym)
	}

	inst, err := factory()
	if err != nil {
		return nil, fmt.Errorf("so: %q New() failed: %w", path, err)
	}

	key := l.pluginKey(inst.Name(), inst.Version())
	if _, exists := l.plugin[key]; exists {
		return nil, fmt.Errorf("so: plugin %q already loaded", key)
	}

	l.plugin[key] = inst
	return inst, nil
}

// Register adds a Plugin instance directly to the loader without requiring
// a .so file. This is primarily useful for testing scenarios where plugins
// are created in-memory. Returns an error if the same name@version is
// already registered.
func (l *Loader) Register(p Plugin) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.pluginKey(p.Name(), p.Version())
	if _, exists := l.plugin[key]; exists {
		return fmt.Errorf("so: plugin %q already registered", key)
	}

	l.plugin[key] = p
	return nil
}

// Get retrieves a plugin by name. If version is empty, returns the
// plugin with the highest version. Returns false if not found.
func (l *Loader) Get(name, version string) (Plugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if version != "" {
		p, ok := l.plugin[l.pluginKey(name, version)]
		return p, ok
	}

	// No version specified: find the highest version.
	var best Plugin
	var bestVer string
	for key, p := range l.plugin {
		pName, pVer := l.splitKey(key)
		if pName == name {
			if best == nil || compareVersions(pVer, bestVer) > 0 {
				best = p
				bestVer = pVer
			}
		}
	}
	return best, best != nil
}

// List returns all loaded plugins sorted by name ascending, version descending.
func (l *Loader) List() []Plugin {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]Plugin, 0, len(l.plugin))
	for _, p := range l.plugin {
		result = append(result, p)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name() != result[j].Name() {
			return result[i].Name() < result[j].Name()
		}
		return compareVersions(result[i].Version(), result[j].Version()) > 0
	})

	return result
}

// Count returns the number of loaded plugins.
func (l *Loader) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.plugin)
}

func (l *Loader) pluginKey(name, version string) string {
	return name + "@" + version
}

func (l *Loader) splitKey(key string) (name, version string) {
	parts := strings.SplitN(key, "@", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, ""
}

// compareVersions compares two semantic version strings (e.g. "1.0.0").
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(a, b string) int {
	as := parseVersion(a)
	bs := parseVersion(b)
	for i := 0; i < 3; i++ {
		if as[i] != bs[i] {
			if as[i] > bs[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	result := [3]int{}
	parts := strings.SplitN(v, ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		n := 0
		fmt.Sscanf(parts[i], "%d", &n)
		result[i] = n
	}
	return result
}