// Package api tests for the unified composite child-reference model YAML
// import/export behavior (group/while/loop node_ids).
package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
)

// TestYAMLImportWhileNodeIDs verifies that a while node referencing children
// via node_ids (by name) imports successfully, resolves names to IDs, and
// retains its skeleton config (exit conditions, max_iterations).
func TestYAMLImportWhileNodeIDs(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: while-ref-import-test
nodes:
  - name: PollStatus
    type: http
    config:
      method: GET
      url: /api/status
      extract:
        chargingStatus: $.result.status
  - name: ParseResult
    type: http
    config:
      method: GET
      url: /api/parse
  - name: PollLoop
    type: while
    config:
      node_ids:
        - PollStatus
        - ParseResult
      exit_conditions:
        - variable: chargingStatus
          operator: equals
          value: "4"
      max_iterations: 10
      interval_seconds: 1
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "While Ref Import",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "import failed: %s", result.Message)

	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// List nodes and verify while config.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))

	var pollLoop *dto.NodeDTO
	nameToID := make(map[string]string)
	for i, n := range listResp.Items {
		nameToID[n.Name] = n.ID.String()
		if n.Type == "while" {
			pollLoop = &listResp.Items[i]
		}
	}
	require.NotNil(t, pollLoop, "while node must exist")

	var cfg struct {
		NodeIDs         []string         `json:"node_ids"`
		ExitConditions  []map[string]any `json:"exit_conditions"`
		MaxIterations   int              `json:"max_iterations"`
		IntervalSeconds int              `json:"interval_seconds"`
	}
	require.NoError(t, json.Unmarshal([]byte(pollLoop.Config), &cfg))
	require.Len(t, cfg.NodeIDs, 2, "while node_ids must resolve to 2 children")
	// Names must be resolved to the children's node IDs.
	assert.Equal(t, nameToID["PollStatus"], cfg.NodeIDs[0])
	assert.Equal(t, nameToID["ParseResult"], cfg.NodeIDs[1])
	// Skeleton config must survive the name→ID resolution pass.
	require.Len(t, cfg.ExitConditions, 1)
	assert.Equal(t, "chargingStatus", cfg.ExitConditions[0]["variable"])
	assert.Equal(t, 10, cfg.MaxIterations)
	assert.Equal(t, 1, cfg.IntervalSeconds)
}

// TestYAMLImportLoopNodeIDs verifies loop node_ids import and name→ID resolution.
func TestYAMLImportLoopNodeIDs(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: loop-ref-import-test
nodes:
  - name: StepA
    type: http
    config:
      url: /api/a
  - name: RepeatBlock
    type: loop
    config:
      node_ids:
        - StepA
      loop_count: 2
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Loop Ref Import",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "import failed: %s", result.Message)

	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))

	var loopNode *dto.NodeDTO
	stepAID := ""
	for i, n := range listResp.Items {
		if n.Name == "StepA" {
			stepAID = n.ID.String()
		}
		if n.Type == "loop" {
			loopNode = &listResp.Items[i]
		}
	}
	require.NotNil(t, loopNode)

	var cfg struct {
		NodeIDs   []string `json:"node_ids"`
		LoopCount int      `json:"loop_count"`
	}
	require.NoError(t, json.Unmarshal([]byte(loopNode.Config), &cfg))
	require.Len(t, cfg.NodeIDs, 1)
	assert.Equal(t, stepAID, cfg.NodeIDs[0])
	assert.Equal(t, 2, cfg.LoopCount)
}

// TestYAMLImportRejectsCompositeChildInWhile verifies the nesting rule:
// while/loop node_ids referencing a composite node (group/while/loop) is
// rejected with 400.
func TestYAMLImportRejectsCompositeChildInWhile(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: while-nested-forbidden
nodes:
  - name: InnerGroup
    type: group
    config:
      node_ids: []
      loop_count: 1
  - name: OuterWhile
    type: while
    config:
      node_ids:
        - InnerGroup
      max_iterations: 3
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "While Nested Forbidden",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code, "while referencing a group child must be rejected")
	assert.Contains(t, result.Message, "composite child")
}

// TestYAMLImportRejectsGroupInGroup verifies the nesting rule for group
// containing another group still rejects with 400 (extended from group-only
// validation to the unified composite validation).
func TestYAMLImportRejectsGroupInGroup(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: group-in-group-forbidden
nodes:
  - name: InnerGroup
    type: group
    config:
      node_ids: []
      loop_count: 1
  - name: OuterGroup
    type: group
    config:
      node_ids:
        - InnerGroup
      loop_count: 1
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Group In Group Forbidden",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code, "group referencing a group child must be rejected")
	assert.Contains(t, result.Message, "cannot contain another group")
}

// TestYAMLExportNodeIDsAsNames verifies the export→import round trip for all
// three composite types: exported node_ids must be node names (not snowflake
// IDs), and re-importing the exported YAML must succeed with child references
// intact.
func TestYAMLExportNodeIDsAsNames(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: composite-export-roundtrip
nodes:
  - name: ChildA
    type: http
    config:
      url: /api/a
  - name: ChildB
    type: http
    config:
      url: /api/b
  - name: GroupBlock
    type: group
    config:
      node_ids:
        - ChildA
        - ChildB
      loop_count: 1
  - name: WhileBlock
    type: while
    config:
      node_ids:
        - ChildA
      exit_conditions:
        - variable: done
          operator: equals
          value: "1"
      max_iterations: 5
  - name: LoopBlock
    type: loop
    config:
      node_ids:
        - ChildB
      loop_count: 3
`

	// Step 1: Import.
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Composite Export Roundtrip",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "import failed: %s", result.Message)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Step 2: Export — node_ids must come back as names.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/export", dto.IDRequest{ID: scene.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "export failed: %s", result.Message)
	exportData, _ := json.Marshal(result.Data)
	var exportResp dto.ExportYAMLResponse
	require.NoError(t, json.Unmarshal(exportData, &exportResp))
	require.NotEmpty(t, exportResp.YAML)

	assert.Contains(t, exportResp.YAML, "- ChildA", "group node_ids must export as node names")
	assert.Contains(t, exportResp.YAML, "- ChildB", "group node_ids must export as node names")

	// The exported YAML must not contain snowflake ID references inside the
	// composite configs — crude check: no long digit strings in node_ids lines.
	for _, line := range strings.Split(exportResp.YAML, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") && len(trimmed) > 3 && isAllDigits(trimmed[2:]) && len(trimmed[2:]) > 5 {
			t.Fatalf("exported YAML contains a numeric ID reference where a name was expected: %q", trimmed)
		}
	}

	// Step 3: Re-import the exported YAML into a new scene (round trip).
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Composite Roundtrip Re-import",
		YAML: exportResp.YAML,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "re-import failed: %s", result.Message)
	sceneData2, _ := json.Marshal(result.Data)
	var scene2 dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData2, &scene2))

	// Step 4: Verify re-imported composite configs resolved children again.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene2.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))

	nameToID2 := make(map[string]string)
	for _, n := range listResp.Items {
		nameToID2[n.Name] = n.ID.String()
	}

	for _, n := range listResp.Items {
		switch n.Type {
		case "group":
			var cfg struct {
				NodeIDs []string `json:"node_ids"`
			}
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			require.Len(t, cfg.NodeIDs, 2)
			assert.Equal(t, nameToID2["ChildA"], cfg.NodeIDs[0])
			assert.Equal(t, nameToID2["ChildB"], cfg.NodeIDs[1])
		case "while":
			var cfg struct {
				NodeIDs       []string `json:"node_ids"`
				MaxIterations int      `json:"max_iterations"`
			}
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			require.Len(t, cfg.NodeIDs, 1)
			assert.Equal(t, nameToID2["ChildA"], cfg.NodeIDs[0])
			assert.Equal(t, 5, cfg.MaxIterations)
		case "loop":
			var cfg struct {
				NodeIDs   []string `json:"node_ids"`
				LoopCount int      `json:"loop_count"`
			}
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			require.Len(t, cfg.NodeIDs, 1)
			assert.Equal(t, nameToID2["ChildB"], cfg.NodeIDs[0])
			assert.Equal(t, 3, cfg.LoopCount)
		}
	}
}

// isAllDigits reports whether s consists only of ASCII digits.
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}
