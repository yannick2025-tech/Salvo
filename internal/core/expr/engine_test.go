package expr

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFunc returns a fixed value, useful for deterministic tests.
func stubFunc(value string) FunctionHandler {
	return func(args []string) (string, error) {
		return value, nil
	}
}

// echoFunc returns all args joined by comma.
func echoFunc() FunctionHandler {
	return func(args []string) (string, error) {
		return strings.Join(args, ","), nil
	}
}

// errorFunc always returns an error.
func errorFunc(msg string) FunctionHandler {
	return func(args []string) (string, error) {
		return "", errors.New(msg)
	}
}

func TestResolveVariableReference(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "simple variable",
			input:     "Hello ${name}",
			variables: map[string]any{"name": "World"},
			want:      "Hello World",
		},
		{
			name:      "variable at start",
			input:     "${greeting}, user!",
			variables: map[string]any{"greeting": "Hi"},
			want:      "Hi, user!",
		},
		{
			name:      "variable at end",
			input:     "Hello ${name}",
			variables: map[string]any{"name": "World"},
			want:      "Hello World",
		},
		{
			name:      "multiple variables",
			input:     "${a} and ${b}",
			variables: map[string]any{"a": "foo", "b": "bar"},
			want:      "foo and bar",
		},
		{
			name:      "integer variable",
			input:     "count=${n}",
			variables: map[string]any{"n": 42},
			want:      "count=42",
		},
		{
			name:      "float variable",
			input:     "rate=${r}",
			variables: map[string]any{"r": 3.14},
			want:      "rate=3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewFunctionRegistry()
			got, err := Resolve(tt.input, tt.variables, r)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveFunctionCall(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]any
		register  func(r *FunctionRegistry)
		want      string
		wantErr   bool
	}{
		{
			name:      "function no args",
			input:     "${__ping()}",
			variables: nil,
			register: func(r *FunctionRegistry) {
				_ = r.Register("__ping", stubFunc("pong"))
			},
			want: "pong",
		},
		{
			name:      "function with args",
			input:     "${__echo(hello,world)}",
			variables: nil,
			register: func(r *FunctionRegistry) {
				_ = r.Register("__echo", echoFunc())
			},
			want: "hello,world",
		},
		{
			name:      "function with quoted args containing comma",
			input:     `${__echo("a,b",c)}`,
			variables: nil,
			register: func(r *FunctionRegistry) {
				_ = r.Register("__echo", echoFunc())
			},
			want: "a,b,c",
		},
		{
			name:      "function in middle of text",
			input:     "result=${__ping()}, done",
			variables: nil,
			register: func(r *FunctionRegistry) {
				_ = r.Register("__ping", stubFunc("pong"))
			},
			want: "result=pong, done",
		},
		{
			name:      "function returns error",
			input:     "${__fail()}",
			variables: nil,
			register: func(r *FunctionRegistry) {
				_ = r.Register("__fail", errorFunc("boom"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewFunctionRegistry()
			if tt.register != nil {
				tt.register(r)
			}
			got, err := Resolve(tt.input, tt.variables, r)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveMathExpression(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "simple multiplication and division",
			input:     "${chargeTime} * ${ranking} / 100",
			variables: map[string]any{"chargeTime": 60, "ranking": 50},
			want:      "30",
		},
		{
			name:      "addition",
			input:     "${a} + ${b}",
			variables: map[string]any{"a": 10, "b": 20},
			want:      "30",
		},
		{
			name:      "subtraction",
			input:     "${a} - ${b}",
			variables: map[string]any{"a": 50, "b": 20},
			want:      "30",
		},
		{
			name:      "parentheses",
			input:     "(${a} + ${b}) * 3",
			variables: map[string]any{"a": 10, "b": 20},
			want:      "90",
		},
		{
			name:      "complex expression",
			input:     "${a} * (${b} + 5) / 3",
			variables: map[string]any{"a": 12, "b": 10},
			want:      "60",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewFunctionRegistry()
			got, err := Resolve(tt.input, tt.variables, r)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveNestedExpression(t *testing.T) {
	r := NewFunctionRegistry()
	_ = r.Register("__add", func(args []string) (string, error) {
		if len(args) != 2 {
			return "", errors.New("need 2 args")
		}
		a, _ := strconv.Atoi(args[0])
		b, _ := strconv.Atoi(args[1])
		return strconv.Itoa(a + b), nil
	})

	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "nested variable in function args",
			input:     "${__add(${x}, ${y})}",
			variables: map[string]any{"x": 3, "y": 4},
			want:      "7",
		},
		{
			name:      "nested function in function args",
			input:     "${__add(${__add(1, 2)}, 10)}",
			variables: nil,
			want:      "13",
		},
		{
			name:      "nested variable in math context",
			input:     "${${x}} * 2",
			variables: map[string]any{"x": "5"},
			want:      "10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.input, tt.variables, r)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveMixedExpression(t *testing.T) {
	r := NewFunctionRegistry()
	_ = r.Register("__snowflakeId", stubFunc("1234567890123456789"))
	_ = r.Register("__email", stubFunc("user@example.com"))

	t.Run("text with variable and function", func(t *testing.T) {
		vars := map[string]any{"name": "Alice"}
		got, err := Resolve("用户:${name}, ID:${__snowflakeId()}", vars, r)
		require.NoError(t, err)
		assert.Equal(t, "用户:Alice, ID:1234567890123456789", got)
	})

	t.Run("multiple functions and variables", func(t *testing.T) {
		vars := map[string]any{"role": "admin"}
		got, err := Resolve("${__email()} [${role}]", vars, r)
		require.NoError(t, err)
		assert.Equal(t, "user@example.com [admin]", got)
	})
}

func TestResolveNoExpression(t *testing.T) {
	r := NewFunctionRegistry()
	vars := map[string]any{"name": "World"}

	got, err := Resolve("plain text without expressions", vars, r)
	require.NoError(t, err)
	assert.Equal(t, "plain text without expressions", got)
}

func TestResolveEmptyString(t *testing.T) {
	r := NewFunctionRegistry()
	got, err := Resolve("", nil, r)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestResolveCircularReference(t *testing.T) {
	r := NewFunctionRegistry()
	vars := map[string]any{
		"a": "${b}",
		"b": "${a}",
	}

	_, err := Resolve("${a}", vars, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth")
}

func TestResolveMaxDepthExceeded(t *testing.T) {
	r := NewFunctionRegistry()
	// Build a chain: v0 -> v1 -> v2 -> ... -> v11 (exceeds max depth of 10)
	vars := make(map[string]any)
	for i := 0; i < 12; i++ {
		vars["v"+strconv.Itoa(i)] = "${v" + strconv.Itoa(i+1) + "}"
	}
	vars["v12"] = "end"

	_, err := Resolve("${v0}", vars, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth")
}

func TestResolveUnregisteredFunction(t *testing.T) {
	r := NewFunctionRegistry()
	got, err := Resolve("${__unknown()}", nil, r)
	require.NoError(t, err)
	// Unregistered functions should be left as-is (original text preserved).
	assert.Equal(t, "${__unknown()}", got)
}

func TestResolveFunctionWithNoArgs(t *testing.T) {
	r := NewFunctionRegistry()
	_ = r.Register("__random", func(args []string) (string, error) {
		if len(args) == 0 {
			return "", errors.New("requires at least 2 args")
		}
		return "42", nil
	})

	_, err := Resolve("${__random()}", nil, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires")
}

func TestResolveVariableNotFound(t *testing.T) {
	r := NewFunctionRegistry()
	_, err := Resolve("Hello ${missing}", nil, r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveConcurrent(t *testing.T) {
	r := NewFunctionRegistry()
	_ = r.Register("__ping", stubFunc("pong"))
	vars := map[string]any{"name": "World"}

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			got, err := Resolve("Hi ${name}=${__ping()}", vars, r)
			assert.NoError(t, err)
			assert.Equal(t, "Hi World=pong", got)
			done <- true
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
