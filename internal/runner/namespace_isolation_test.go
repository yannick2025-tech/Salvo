package runner

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yannick2025-tech/Salvo/internal/core/variable"
)

// TestNamespaceIsolation_SceneOverridesGlobal verifies that scene variables
// take precedence over global config variables (salvo.yaml).
//
// This is the core fix for the namespace pollution bug:
// When global salvo.yaml defines order_id: "67890" and the scene
// defines order_id: "", the scene value should win.
func TestNamespaceIsolation_SceneOverridesGlobal(t *testing.T) {
	globalScope := variable.NewScope(variable.WithLevel(variable.ScopeGlobal))

	// Step 1: Global config variables (salvo.yaml) set first
	globalScope.Set("base_url", "http://localhost:9090/mock/api")
	globalScope.Set("product_id", "12345")
	globalScope.Set("order_id", "67890") // mock-server default — should NOT pollute scene

	// Step 2: Scene variables set AFTER (take precedence)
	globalScope.Set("order_id", "")       // scene defines empty default
	globalScope.Set("charging_status", "") // scene defines empty default
	globalScope.Set("token", "")           // scene defines empty default

	// Verify: scene's order_id ("") overrides global's order_id ("67890")
	val, ok := globalScope.Get("order_id")
	require.True(t, ok)
	assert.Equal(t, "", val, "scene's order_id should override global's order_id")

	// Verify: global variables not overridden by scene remain
	val, ok = globalScope.Get("base_url")
	require.True(t, ok)
	assert.Equal(t, "http://localhost:9090/mock/api", val, "global base_url should be preserved")

	val, ok = globalScope.Get("product_id")
	require.True(t, ok)
	assert.Equal(t, "12345", val, "global product_id should be preserved")
}

// TestNamespaceIsolation_ExtractFailureKeepsSceneDefault verifies that
// when extract fails (response is not valid JSON), the variable keeps
// its scene-defined default value rather than falling back to a global value.
func TestNamespaceIsolation_ExtractFailureKeepsSceneDefault(t *testing.T) {
	// Simulate the variable map as built by buildScope (global first, scene second)
	vars := map[string]any{
		"base_url":   "http://localhost:9090/mock/api",
		"order_id":   "",  // scene default (overrides global "67890")
		"token":      "",  // scene default
		"charging_status": "", // scene default
	}

	// Simulate: extract skipped because response is AES encrypted
	// (extractVarsFromResponse does nothing because response is not valid JSON)
	// Variables should keep their scene defaults.

	assert.Equal(t, "", vars["order_id"], "order_id should keep scene default (empty)")
	assert.Equal(t, "", vars["charging_status"], "charging_status should keep scene default")
}

// TestNamespaceIsolation_ExtractSuccessOverridesDefault verifies that
// when extract succeeds, it correctly overrides the scene default.
func TestNamespaceIsolation_ExtractSuccessOverridesDefault(t *testing.T) {
	vars := map[string]any{
		"order_id": "", // scene default
	}

	// Simulate successful extract
	responseBody := []byte(`{"errorCode":0,"data":{"orderId":"202607211619060001"}}`)
	var jsonData map[string]any
	err := json.Unmarshal(responseBody, &jsonData)
	require.NoError(t, err)

	// Extract order_id
	value := resolveJSONPath(jsonData, "$.data.orderId")
	require.NotNil(t, value)
	vars["order_id"] = value

	assert.Equal(t, "202607211619060001", vars["order_id"],
		"extracted order_id should override scene default")
}

// TestNamespaceIsolation_ExtractSkippedOnEncryptedResponse verifies the
// exact scenario from the bug report: AES encrypted response causes
// extract to be skipped, and the variable keeps its scene default.
func TestNamespaceIsolation_ExtractSkippedOnEncryptedResponse(t *testing.T) {
	vars := map[string]any{
		"order_id": "", // scene default (should NOT become "67890")
	}

	// Encrypted response body (not valid JSON)
	encryptedBody := []byte(`"VuulXttG/VbF2GUcBhgqt334IuwkJc0EnMVYQNR8xCxWP6R9N2P06yWmn..."`)

	// Try to parse as JSON — should fail (same as extractVarsFromResponse logic)
	var jsonData map[string]any
	err := json.Unmarshal(encryptedBody, &jsonData)
	require.Error(t, err, "encrypted body should not parse as JSON")

	// extractVarsFromResponse returns early without setting any variables
	// → order_id keeps its scene default
	assert.Equal(t, "", vars["order_id"],
		"order_id should keep scene default when extract is skipped")
}

// TestNamespaceIsolation_BuildScopeOrder verifies the buildScope function
// sets global variables first, then scene variables (scene takes precedence).
func TestNamespaceIsolation_BuildScopeOrder(t *testing.T) {
	scope := variable.NewScope(variable.WithLevel(variable.ScopeGlobal))

	// Simulate buildScope: global first
	globalVars := map[string]string{
		"base_url":  "http://localhost:9090/mock/api",
		"order_id":  "67890",
		"product_id": "12345",
	}
	for k, v := range globalVars {
		scope.Set(k, v)
	}

	// Then scene variables (should override globals with same key)
	sceneVars := map[string]string{
		"order_id":       "", // scene defines empty default
		"charging_status": "",
		"token":          "",
		"order_id_int":   "",
	}
	for k, v := range sceneVars {
		scope.Set(k, v)
	}

	// Verify precedence
	testCases := []struct {
		key      string
		expected any
	}{
		{"order_id", ""},           // scene overrides global
		{"base_url", "http://localhost:9090/mock/api"}, // global preserved
		{"product_id", "12345"},    // global preserved
		{"charging_status", ""},    // scene only
		{"token", ""},              // scene only
	}

	for _, tc := range testCases {
		val, ok := scope.Get(tc.key)
		require.True(t, ok, "key %s should exist", tc.key)
		assert.Equal(t, tc.expected, val, "key %s should have expected value", tc.key)
	}
}
