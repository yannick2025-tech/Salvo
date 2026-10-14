package so

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/expr"
)

func TestRegisterSO_LatestVersionCall(t *testing.T) {
	reg := expr.NewFunctionRegistry()
	loader := NewLoader()

	// Register two versions.
	loader.plugin["test@1.0.0"] = &stubPlugin{
		name:    "test",
		version: "1.0.0",
		callFn:  func(op string, args []string) (string, error) { return "v1-" + op, nil },
	}
	loader.plugin["test@1.1.0"] = &stubPlugin{
		name:    "test",
		version: "1.1.0",
		callFn:  func(op string, args []string) (string, error) { return "v2-" + op, nil },
	}

	err := RegisterSO(reg, loader)
	require.NoError(t, err)

	result, err := expr.Resolve(`${__so("test", "encrypt", "data")}`, nil, reg)
	require.NoError(t, err)
	assert.Equal(t, "v2-encrypt", result, "should call latest version 1.1.0")
}

func TestRegisterSO_SpecificVersionCall(t *testing.T) {
	reg := expr.NewFunctionRegistry()
	loader := NewLoader()

	loader.plugin["test@1.0.0"] = &stubPlugin{
		name:    "test",
		version: "1.0.0",
		callFn:  func(op string, args []string) (string, error) { return "v1-" + op, nil },
	}
	loader.plugin["test@1.1.0"] = &stubPlugin{
		name:    "test",
		version: "1.1.0",
		callFn:  func(op string, args []string) (string, error) { return "v2-" + op, nil },
	}

	err := RegisterSO(reg, loader)
	require.NoError(t, err)

	result, err := expr.Resolve(`${__so("test@1.0.0", "encrypt", "data")}`, nil, reg)
	require.NoError(t, err)
	assert.Equal(t, "v1-encrypt", result, "should call version 1.0.0")
}

func TestRegisterSO_PluginNotFound(t *testing.T) {
	reg := expr.NewFunctionRegistry()
	loader := NewLoader()

	err := RegisterSO(reg, loader)
	require.NoError(t, err)

	result, err := expr.Resolve(`${__so("nonexistent", "encrypt", "data")}`, nil, reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Empty(t, result)
}

func TestRegisterSO_CallFailure(t *testing.T) {
	reg := expr.NewFunctionRegistry()
	loader := NewLoader()

	loader.plugin["fail@1.0.0"] = &stubPlugin{
		name:    "fail",
		version: "1.0.0",
		callFn:  func(op string, args []string) (string, error) { return "", errors.New("operation failed") },
	}

	err := RegisterSO(reg, loader)
	require.NoError(t, err)

	_, err = expr.Resolve(`${__so("fail", "encrypt", "data")}`, nil, reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation failed")
}

func TestRegisterSO_EmptyArgs(t *testing.T) {
	reg := expr.NewFunctionRegistry()
	loader := NewLoader()

	loader.plugin["test@1.0.0"] = &stubPlugin{
		name:    "test",
		version: "1.0.0",
		callFn:  func(op string, args []string) (string, error) { return fmt.Sprintf("op=%s,args=%d", op, len(args)), nil },
	}

	err := RegisterSO(reg, loader)
	require.NoError(t, err)

	result, err := expr.Resolve(`${__so("test", "op")}`, nil, reg)
	require.NoError(t, err)
	assert.Equal(t, "op=op,args=0", result)
}

func TestRegisterSO_QuotedArgs(t *testing.T) {
	reg := expr.NewFunctionRegistry()
	loader := NewLoader()

	loader.plugin["test@1.0.0"] = &stubPlugin{
		name:    "test",
		version: "1.0.0",
		callFn:  func(op string, args []string) (string, error) { return fmt.Sprintf("op=%s,args0=%s", op, args[0]), nil },
	}

	err := RegisterSO(reg, loader)
	require.NoError(t, err)

	result, err := expr.Resolve("${__so(\"test\", \"op\", \"arg with space\")}", nil, reg)
	require.NoError(t, err)
	assert.Equal(t, "op=op,args0=arg with space", result)
}

func TestRegisterSO_TooFewArgs(t *testing.T) {
	reg := expr.NewFunctionRegistry()
	loader := NewLoader()

	err := RegisterSO(reg, loader)
	require.NoError(t, err)

	_, err = expr.Resolve(`${__so("test")}`, nil, reg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires at least 2 arguments")
}

func TestParseNameVersion(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{"shell-aes@1.0.0", "shell-aes", "1.0.0"},
		{"plugin", "plugin", ""},
		{"a@b@c", "a", "b@c"},
		{"", "", ""},
	}
	for _, tt := range tests {
		name, version := parseNameVersion(tt.input)
		assert.Equal(t, tt.wantName, name, "parseNameVersion(%q).name", tt.input)
		assert.Equal(t, tt.wantVersion, version, "parseNameVersion(%q).version", tt.input)
	}
}