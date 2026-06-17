package so

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPlugin implements Plugin for testing.
type stubPlugin struct {
	name    string
	version string
	callFn  func(op string, args []string) (string, error)
}

func (s *stubPlugin) Name() string                          { return s.name }
func (s *stubPlugin) Version() string                       { return s.version }
func (s *stubPlugin) Call(op string, args []string) (string, error) { return s.callFn(op, args) }

func TestLoaderGetLatestVersion(t *testing.T) {
	l := NewLoader()

	// Manually register plugins (bypass .so loading).
	v1 := &stubPlugin{name: "test", version: "1.0.0"}
	v2 := &stubPlugin{name: "test", version: "1.1.0"}
	l.plugin["test@1.0.0"] = v1
	l.plugin["test@1.1.0"] = v2

	p, ok := l.Get("test", "")
	require.True(t, ok)
	assert.Equal(t, "1.1.0", p.Version(), "Get without version should return latest")
}

func TestLoaderGetSpecificVersion(t *testing.T) {
	l := NewLoader()

	v1 := &stubPlugin{name: "test", version: "1.0.0"}
	v2 := &stubPlugin{name: "test", version: "1.1.0"}
	l.plugin["test@1.0.0"] = v1
	l.plugin["test@1.1.0"] = v2

	p, ok := l.Get("test", "1.0.0")
	require.True(t, ok)
	assert.Equal(t, "1.0.0", p.Version())
}

func TestLoaderGetNotFound(t *testing.T) {
	l := NewLoader()

	_, ok := l.Get("unknown", "")
	assert.False(t, ok)

	_, ok = l.Get("unknown", "1.0.0")
	assert.False(t, ok)
}

func TestLoaderListSorted(t *testing.T) {
	l := NewLoader()

	l.plugin["alpha@2.0.0"] = &stubPlugin{name: "alpha", version: "2.0.0"}
	l.plugin["alpha@1.0.0"] = &stubPlugin{name: "alpha", version: "1.0.0"}
	l.plugin["beta@1.0.0"] = &stubPlugin{name: "beta", version: "1.0.0"}

	list := l.List()
	require.Len(t, list, 3)

	// Sorted: name asc, version desc.
	assert.Equal(t, "alpha", list[0].Name())
	assert.Equal(t, "2.0.0", list[0].Version())
	assert.Equal(t, "alpha", list[1].Name())
	assert.Equal(t, "1.0.0", list[1].Version())
	assert.Equal(t, "beta", list[2].Name())
	assert.Equal(t, "1.0.0", list[2].Version())
}

func TestLoaderDuplicateRegistration(t *testing.T) {
	l := NewLoader()
	l.plugin["dup@1.0.0"] = &stubPlugin{name: "dup", version: "1.0.0"}

	// Overwrite tracking: manual registration via map.
	p := &stubPlugin{name: "dup", version: "1.0.0"}
	key := l.pluginKey(p.Name(), p.Version())
	_, exists := l.plugin[key]
	assert.True(t, exists, "duplicate exists check")

	// Simulating duplicate: the key already exists.
	l.plugin["dup@1.0.0"] = p // overwrites silently when using map directly
}

func TestLoaderCount(t *testing.T) {
	l := NewLoader()
	assert.Equal(t, 0, l.Count())

	l.plugin["a@1.0.0"] = &stubPlugin{name: "a", version: "1.0.0"}
	l.plugin["b@1.0.0"] = &stubPlugin{name: "b", version: "1.0.0"}
	assert.Equal(t, 2, l.Count())
}

func TestLoaderConcurrentSafety(t *testing.T) {
	l := NewLoader()

	// Add some plugins.
	l.plugin["alpha@1.0.0"] = &stubPlugin{name: "alpha", version: "1.0.0"}
	l.plugin["beta@1.0.0"] = &stubPlugin{name: "beta", version: "1.0.0"}

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(idx int) {
			if idx%2 == 0 {
				l.Get("alpha", "")
			} else {
				l.List()
			}
			done <- true
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}

	// Should not race and report consistent state.
	assert.Equal(t, 2, l.Count())
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.0.0", [3]int{1, 0, 0}},
		{"2.3.4", [3]int{2, 3, 4}},
		{"0.0.1", [3]int{0, 0, 1}},
		{"10.20.30", [3]int{10, 20, 30}},
		{"invalid", [3]int{0, 0, 0}},
		{"1.2", [3]int{1, 2, 0}},
		{"", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		got := parseVersion(tt.input)
		assert.Equal(t, tt.want, got, "parseVersion(%q)", tt.input)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.0.1", "0.0.0", 1},
		{"1.0", "1.0.0", 0},
		{"invalid1", "invalid2", 0},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		assert.Equal(t, tt.want, got, "compareVersions(%q, %q)", tt.a, tt.b)
	}
}