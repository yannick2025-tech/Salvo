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

func TestResolveBareVariables(t *testing.T) {
	r := NewFunctionRegistry()
	_ = r.Register("__add", func(args []string) (string, error) {
		if len(args) != 2 {
			return "", errors.New("need 2 args")
		}
		a, _ := strconv.Atoi(args[0])
		b, _ := strconv.Atoi(args[1])
		return strconv.Itoa(a + b), nil
	})
	_ = r.Register("__echo", echoFunc())

	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "bare variable in function arg",
			input:     "${__echo(${name})}",
			variables: map[string]any{"name": "Alice"},
			want:      "Alice",
		},
		{
			name:      "nested variable in math expression",
			input:     "${${base}}",
			variables: map[string]any{"base": "x", "x": "42"},
			want:      "42",
		},
		{
			name:      "nested variable in function args",
			input:     "${__add(${x}, ${y})}",
			variables: map[string]any{"x": 5, "y": 7},
			want:      "12",
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

func TestResolveMathExpressionEdgeCases(t *testing.T) {
	r := NewFunctionRegistry()

	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "negative result",
			input:     "${a} - ${b}",
			variables: map[string]any{"a": 5, "b": 15},
			want:      "-10",
		},
		{
			name:      "decimal result",
			input:     "${a} / ${b}",
			variables: map[string]any{"a": 7, "b": 2},
			want:      "3.5",
		},
		{
			name:      "complex nested parentheses",
			input:     "((${a} + ${b}) * (${c} - ${d})) / 2",
			variables: map[string]any{"a": 10, "b": 5, "c": 8, "d": 3},
			want:      "37.5",
		},
		{
			name:      "power of two",
			input:     "${a} * ${a}",
			variables: map[string]any{"a": 4},
			want:      "16",
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

func TestResolveNestedInArgs(t *testing.T) {
	r := NewFunctionRegistry()
	_ = r.Register("__concat", func(args []string) (string, error) {
		return strings.Join(args, ""), nil
	})

	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "nested variable in function arg",
			input:     "${__concat(${prefix}, ${suffix})}",
			variables: map[string]any{"prefix": "Hello", "suffix": "World"},
			want:      "HelloWorld",
		},
		{
			name:      "multiple nested variables",
			input:     "${__concat(${a}, ${b}, ${c})}",
			variables: map[string]any{"a": "1", "b": "2", "c": "3"},
			want:      "123",
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

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, "hello"},
		{`'world'`, "world"},
		{`no quotes`, "no quotes"},
		{`"`, `"`},
		{`""`, ""},
		{`''`, ""},
		{`"unclosed`, `"unclosed`},
		{`unclosed"`, `unclosed"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimQuotes(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(42), 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"int32", int32(50), 50.0, true},
		{"int16", int16(25), 25.0, true},
		{"int8", int8(10), 10.0, true},
		{"uint", uint(30), 30.0, true},
		{"uint64", uint64(60), 60.0, true},
		{"uint32", uint32(40), 40.0, true},
		{"uint16", uint16(20), 20.0, true},
		{"uint8", uint8(15), 15.0, true},
		{"string valid", "3.14", 3.14, true},
		{"string invalid", "abc", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat(tt.input)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.InDelta(t, tt.want, got, 0.001)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"zero int", int(0), true},
		{"non-zero int", int(5), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEmpty(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveMathExpressionPaths(t *testing.T) {
	r := NewFunctionRegistry()

	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "simple math with variables",
			input:     "${a} + ${b}",
			variables: map[string]any{"a": 10, "b": 20},
			want:      "30",
		},
		{
			name:      "math with division",
			input:     "${a} / ${b}",
			variables: map[string]any{"a": 100, "b": 4},
			want:      "25",
		},
		{
			name:      "complex expression",
			input:     "(${a} + ${b}) * ${c}",
			variables: map[string]any{"a": 5, "b": 3, "c": 2},
			want:      "16",
		},
		{
			name:      "invalid math returns as-is",
			input:     "${a} + ${b}",
			variables: map[string]any{"a": "x", "b": "y"},
			want:      "x + y",
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

func TestReplaceBareVariables(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]any
		want      string
	}{
		{
			name:      "replace single variable",
			input:     "x + 5",
			variables: map[string]any{"x": 10},
			want:      "10 + 5",
		},
		{
			name:      "replace multiple variables",
			input:     "a + b",
			variables: map[string]any{"a": 5, "b": 3},
			want:      "5 + 3",
		},
		{
			name:      "nil variables",
			input:     "x + 5",
			variables: nil,
			want:      "x + 5",
		},
		{
			name:      "no match",
			input:     "y + 5",
			variables: map[string]any{"x": 10},
			want:      "y + 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceBareVariables(tt.input, tt.variables)
			assert.Equal(t, tt.want, got)
		})
	}
}
