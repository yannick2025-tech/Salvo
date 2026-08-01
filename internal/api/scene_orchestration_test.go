package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
)

// --- 11.1 Scene Variables Integration ---

func TestSceneVariablesBatchSetAndList(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "var-integration-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Batch-set variables with nested references (stored in scene.Variables JSON)
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/variables/batch-set", dto.BatchSetVariablesRequest{
		SceneID: scene.ID,
		Variables: map[string]string{
			"base_url":  "http://localhost:8080",
			"api_path":  "/api/v1/users",
			"full_url":  "${base_url}${api_path}",
			"token":     "Bearer abc123",
			"body_json": `{"name":"test","count":42}`,
		},
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "batch-set failed: %s", result.Message)

	// Verify batch-set response includes variables
	bsData, _ := json.Marshal(result.Data)
	var bsResp dto.BatchSetVariablesResponse
	require.NoError(t, json.Unmarshal(bsData, &bsResp))
	assert.Equal(t, "http://localhost:8080", bsResp.Variables["base_url"])
	assert.Equal(t, "${base_url}${api_path}", bsResp.Variables["full_url"])

	// Verify variables persisted: get scene and check Variables field
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/get", dto.IDRequest{ID: scene.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	sceneData2, _ := json.Marshal(result.Data)
	var loadedScene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData2, &loadedScene))

	var varsMap map[string]string
	require.NoError(t, json.Unmarshal([]byte(loadedScene.Variables), &varsMap))
	assert.Equal(t, 5, len(varsMap), "expected 5 variables in scene")
	assert.Equal(t, "http://localhost:8080", varsMap["base_url"])
	assert.Equal(t, "${base_url}${api_path}", varsMap["full_url"])
	assert.Equal(t, "Bearer abc123", varsMap["token"])
	assert.Equal(t, `{"name":"test","count":42}`, varsMap["body_json"])
}

func TestSceneVariableNestedReferences(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "nested-var-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Set up nested reference chain: A → B → C
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/variables/batch-set", dto.BatchSetVariablesRequest{
		SceneID: scene.ID,
		Variables: map[string]string{
			"C":    "final-value",
			"B":    "${C}",
			"A":    "${B}",
			"url":  "http://${C}/api",
			"deep": "${A}-suffix",
		},
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "batch-set failed: %s", result.Message)

	// Verify variables persisted: check scene Variables JSON field
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/get", dto.IDRequest{ID: scene.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	sceneData2, _ := json.Marshal(result.Data)
	var loadedScene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData2, &loadedScene))

	var varsMap map[string]string
	require.NoError(t, json.Unmarshal([]byte(loadedScene.Variables), &varsMap))
	assert.Equal(t, "final-value", varsMap["C"])
	assert.Equal(t, "${C}", varsMap["B"])
	assert.Equal(t, "${B}", varsMap["A"])
	assert.Equal(t, "http://${C}/api", varsMap["url"])
	assert.Equal(t, "${A}-suffix", varsMap["deep"])
}

func TestSceneVariableScopedSupport(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "scoped-var-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Set variables with different scopes
	for _, v := range []struct {
		scope, key, value string
	}{
		{"global", "env", "production"},
		{"global", "timeout", "30"},
		{"scene", "base_path", "/api/v2"},
		{"scene", "retries", "3"},
	} {
		resp = postJSONAuth(t, srv, token, "/api/v1/scenes/variables/set", dto.SetVariableRequest{
			SceneID: scene.ID,
			Scope:   v.scope,
			Key:     v.key,
			Value:   v.value,
		})
		result = decodeResponse(t, resp)
		assert.Equal(t, 0, result.Code, "set %s.%s failed", v.scope, v.key)
	}

	// List and verify scope separation
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/variables/list", dto.ListVariablesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.VariableDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))
	assert.Equal(t, 4, len(listResp.Items))

	scopeCount := make(map[string]int)
	for _, v := range listResp.Items {
		scopeCount[v.Scope]++
	}
	assert.Equal(t, 2, scopeCount["global"])
	assert.Equal(t, 2, scopeCount["scene"])
}

// --- 11.2 Data Source Integration ---

func TestDataSourceUploadAndPreview(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "ds-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Upload CSV data source
	uploadResp := postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/upload", dto.UploadDataSourceRequest{
		SceneID:  scene.ID,
		FileName: "users.csv",
		Content:  "id,name,role\n1,Alice,admin\n2,Bob,user\n3,Charlie,user\n",
	})
	result = decodeResponse(t, uploadResp)
	assert.Equal(t, 0, result.Code, "upload failed: %s", result.Message)

	dsData, _ := json.Marshal(result.Data)
	var ds dto.DataSourceDTO
	require.NoError(t, json.Unmarshal(dsData, &ds))
	assert.Equal(t, "users.csv", ds.FileName)
	assert.Equal(t, 3, ds.RowCount)
	assert.Contains(t, ds.Columns, "id")
	assert.Contains(t, ds.Columns, "name")
	assert.Contains(t, ds.Columns, "role")

	// List data sources
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/list", dto.SceneIDRequest{
		SceneID: scene.ID,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	listData, _ := json.Marshal(result.Data)
	var dsList []dto.DataSourceDTO
	require.NoError(t, json.Unmarshal(listData, &dsList))
	assert.Equal(t, 1, len(dsList))
	assert.Equal(t, "users.csv", dsList[0].FileName)

	// Preview data source
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/preview", dto.IDRequest{
		ID: ds.ID,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "preview failed: %s", result.Message)

	prevData, _ := json.Marshal(result.Data)
	var preview dto.DataSourcePreviewDTO
	require.NoError(t, json.Unmarshal(prevData, &preview))
	assert.Equal(t, 3, len(preview.Rows), "expected 3 rows in preview")
	assert.Equal(t, "Alice", preview.Rows[0]["name"])
	assert.Equal(t, "admin", preview.Rows[0]["role"])
}

func TestDataSourceDelete(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "ds-delete-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Upload data source
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/upload", dto.UploadDataSourceRequest{
		SceneID:  scene.ID,
		FileName: "temp.csv",
		Content:  "a,b\n1,2\n",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	dsData, _ := json.Marshal(result.Data)
	var ds dto.DataSourceDTO
	require.NoError(t, json.Unmarshal(dsData, &ds))

	// Delete data source
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/delete", dto.IDRequest{ID: ds.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "delete failed: %s", result.Message)

	// Verify list is empty
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/list", dto.SceneIDRequest{
		SceneID: scene.ID,
	})
	result = decodeResponse(t, resp)
	listData, _ := json.Marshal(result.Data)
	var dsList []dto.DataSourceDTO
	require.NoError(t, json.Unmarshal(listData, &dsList))
	assert.Equal(t, 0, len(dsList))
}

func TestDataSourceEmptyCSV(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "ds-empty-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Upload CSV with headers only (no data rows)
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/upload", dto.UploadDataSourceRequest{
		SceneID:  scene.ID,
		FileName: "empty.csv",
		Content:  "col1,col2,col3\n",
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "upload empty CSV failed: %s", result.Message)

	dsData, _ := json.Marshal(result.Data)
	var ds dto.DataSourceDTO
	require.NoError(t, json.Unmarshal(dsData, &ds))
	assert.Equal(t, 0, ds.RowCount, "expected 0 rows for header-only CSV")
	assert.Equal(t, 3, len(ds.Columns))
}

// --- 10.4 DataSource source field: CSV not overwritten by YAML import ---

func TestYAMLImportDoesNotOverwriteCSVDataSource(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Step 1: Import YAML with a data source named "user_charger"
	yamlContent := `
name: source-override-test
description: Test YAML import does not overwrite CSV data source
data_sources:
  - name: user_charger
    columns:
      - phone
      - pointId
      - postId
    rows:
      - phone: "11111111111"
        pointId: "2200000001"
        postId: "22000000"
nodes:
  - name: Setup
    type: setup
    config:
      url: http://localhost/init
  - name: Teardown
    type: teardown
    config:
      url: http://localhost/cleanup
`
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Source Override Test",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "YAML import failed: %s", result.Message)

	importData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(importData, &scene))

	// Verify the YAML-imported data source has source=yaml
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/list", dto.SceneIDRequest{
		SceneID: scene.ID,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	dsListData, _ := json.Marshal(result.Data)
	var dsList []dto.DataSourceDTO
	require.NoError(t, json.Unmarshal(dsListData, &dsList))
	require.Equal(t, 1, len(dsList), "expected 1 data source after YAML import")
	assert.Equal(t, "yaml", dsList[0].Source, "YAML-imported data source should have source=yaml")
	assert.Equal(t, "user_charger", dsList[0].Name)

	// Step 2: Upload CSV with the same name "user_charger" — YAML and CSV now coexist
	csvResp := postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/upload", dto.UploadDataSourceRequest{
		SceneID:  scene.ID,
		FileName: "user_charger.csv",
		Content:  "phone,pointId,postId\n15550000000,1100000001,11000000\n16660000000,2200000002,22000000\n",
	})
	result = decodeResponse(t, csvResp)
	assert.Equal(t, 0, result.Code, "CSV upload failed: %s", result.Message)

	// Verify both YAML and CSV data sources coexist with the same name
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/list", dto.SceneIDRequest{
		SceneID: scene.ID,
	})
	result = decodeResponse(t, resp)
	dsListData, _ = json.Marshal(result.Data)
	require.NoError(t, json.Unmarshal(dsListData, &dsList))
	require.Equal(t, 2, len(dsList), "expected 2 data sources (yaml + csv) after CSV upload")
	// Find the CSV and YAML entries
	var csvDS, yamlDS *dto.DataSourceDTO
	for i := range dsList {
		if dsList[i].Source == "csv" {
			csvDS = &dsList[i]
		} else if dsList[i].Source == "yaml" {
			yamlDS = &dsList[i]
		}
	}
	require.NotNil(t, csvDS, "CSV data source should exist")
	assert.Equal(t, 2, csvDS.RowCount, "CSV should have 2 data rows")
	require.NotNil(t, yamlDS, "YAML data source should still exist")
	assert.Equal(t, 1, yamlDS.RowCount, "YAML should have 1 data row")

	// Step 3: Re-import YAML with same data source name — CSV should NOT be overwritten
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Source Override Test V2",
		YAML: yamlContent,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "second YAML import failed: %s", result.Message)

	// Verify both CSV and YAML data sources still exist after re-import
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/list", dto.SceneIDRequest{
		SceneID: scene.ID,
	})
	result = decodeResponse(t, resp)
	dsListData, _ = json.Marshal(result.Data)
	require.NoError(t, json.Unmarshal(dsListData, &dsList))
	require.Equal(t, 2, len(dsList), "expected 2 data sources after re-import (yaml + csv coexist)")
	// Verify CSV is still intact
	for i := range dsList {
		if dsList[i].Source == "csv" {
			assert.Equal(t, 2, dsList[i].RowCount, "CSV data should still have 2 rows")
			assert.Equal(t, "user_charger", dsList[i].Name)
		}
	}
}

// --- 11.3 Group Node Integration ---

func TestGroupNodeCreateAndRetrieve(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "group-node-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Add child nodes first
	childIDs := make([]string, 3)
	for i, name := range []string{"Child1", "Child2", "Child3"} {
		resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
			SceneID: scene.ID,
			Name:    name,
			Type:    "http",
			Config:  `{"url":"/api/test"}`,
		})
		result = decodeResponse(t, resp)
		assert.Equal(t, 0, result.Code)
		nodeData, _ := json.Marshal(result.Data)
		var node dto.NodeDTO
		require.NoError(t, json.Unmarshal(nodeData, &node))
		childIDs[i] = node.ID.String()
	}

	// Create group node with child references and loop_count
	groupConfig := map[string]any{
		"node_ids":   childIDs,
		"loop_count": 5,
	}
	configJSON, _ := json.Marshal(groupConfig)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID:   scene.ID,
		Name:      "Login Flow Group",
		Type:      "group",
		Config:    string(configJSON),
		LoopCount: 5,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "create group node failed: %s", result.Message)

	gnData, _ := json.Marshal(result.Data)
	var groupNode dto.NodeDTO
	require.NoError(t, json.Unmarshal(gnData, &groupNode))
	assert.Equal(t, "group", groupNode.Type)
	assert.Equal(t, "Login Flow Group", groupNode.Name)
	assert.Equal(t, 5, groupNode.LoopCount)

	// List nodes and verify
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)

	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))
	assert.Equal(t, 4, len(listResp.Items), "expected 4 nodes (3 children + 1 group)")

	// Verify group node config
	var foundGroup bool
	for _, n := range listResp.Items {
		if n.Type == "group" {
			foundGroup = true
			var cfg struct {
				NodeIDs   []string `json:"node_ids"`
				LoopCount int      `json:"loop_count"`
			}
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			assert.Equal(t, 3, len(cfg.NodeIDs))
			assert.Equal(t, 5, cfg.LoopCount)
		}
	}
	assert.True(t, foundGroup, "group node should be in list")
}

func TestGroupNodeUpdateLoopCount(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "group-update-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Create group node
	groupConfig := map[string]any{
		"node_ids":   []string{},
		"loop_count": 1,
	}
	configJSON, _ := json.Marshal(groupConfig)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID,
		Name:    "Group Node",
		Type:    "group",
		Config:  string(configJSON),
	})
	result = decodeResponse(t, resp)
	nodeData, _ := json.Marshal(result.Data)
	var node dto.NodeDTO
	require.NoError(t, json.Unmarshal(nodeData, &node))

	// Update loop_count
	groupConfig["loop_count"] = 10
	newConfigJSON, _ := json.Marshal(groupConfig)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/update", dto.UpdateNodeRequest{
		ID:        node.ID,
		LoopCount: 10,
		Config:    string(newConfigJSON),
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "update failed: %s", result.Message)

	// Verify update
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   10,
	})
	result = decodeResponse(t, resp)
	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))
	assert.Equal(t, 10, listResp.Items[0].LoopCount)
}

// --- 11.4 & 11.5 Timer Node Integration ---

func TestTimerNodeDelayMode(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "timer-delay-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Create timer node with delay mode (5 seconds)
	timerConfig := map[string]any{
		"mode":  "delay",
		"delay": 5000, // milliseconds
	}
	configJSON, _ := json.Marshal(timerConfig)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID,
		Name:    "Wait 5 Seconds",
		Type:    "timer",
		Config:  string(configJSON),
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "create timer delay node failed: %s", result.Message)

	nodeData, _ := json.Marshal(result.Data)
	var node dto.NodeDTO
	require.NoError(t, json.Unmarshal(nodeData, &node))
	assert.Equal(t, "timer", node.Type)
	assert.Equal(t, "Wait 5 Seconds", node.Name)

	// Verify config
	var cfg struct {
		Mode  string `json:"mode"`
		Delay int    `json:"delay"`
	}
	require.NoError(t, json.Unmarshal([]byte(node.Config), &cfg))
	assert.Equal(t, "delay", cfg.Mode)
	assert.Equal(t, 5000, cfg.Delay)
}

func TestTimerNodeIntervalMode(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// Create scene
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/create", dto.CreateSceneRequest{
		Name: "timer-interval-test", Status: "draft",
	})
	result := decodeResponse(t, resp)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Create timer node with interval mode (2 second interval, 10 seconds total)
	timerConfig := map[string]any{
		"mode":     "interval",
		"interval": 2000,  // milliseconds
		"duration": 10000, // milliseconds
	}
	configJSON, _ := json.Marshal(timerConfig)

	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/add", dto.AddNodeRequest{
		SceneID: scene.ID,
		Name:    "Heartbeat Timer",
		Type:    "timer",
		Config:  string(configJSON),
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "create timer interval node failed: %s", result.Message)

	nodeData, _ := json.Marshal(result.Data)
	var node dto.NodeDTO
	require.NoError(t, json.Unmarshal(nodeData, &node))

	// Verify config
	var cfg struct {
		Mode     string `json:"mode"`
		Interval int    `json:"interval"`
		Duration int    `json:"duration"`
	}
	require.NoError(t, json.Unmarshal([]byte(node.Config), &cfg))
	assert.Equal(t, "interval", cfg.Mode)
	assert.Equal(t, 2000, cfg.Interval)
	assert.Equal(t, 10000, cfg.Duration)
}

// --- 11.6 YAML Import/Export Integration ---

func TestYAMLImportWithVariablesAndDataSources(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: yaml-scene-test
description: Scene imported from YAML with variables and data sources
variables:
  - key: base_url
    value: http://localhost:8080
  - key: api_token
    value: test-token-123
data_sources:
  - name: test_users
    columns:
      - id
      - name
      - email
    rows:
      - id: "1"
        name: Alice
        email: alice@test.com
      - id: "2"
        name: Bob
        email: bob@test.com
nodes:
  - name: Setup
    type: setup
    config:
      url: ${base_url}/init
  - name: Get Users
    type: http
    config:
      url: ${base_url}/users
      headers:
        Authorization: Bearer ${api_token}
  - name: Verify
    type: http
    config:
      url: ${base_url}/verify
  - name: Cleanup
    type: teardown
    config:
      url: ${base_url}/cleanup
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "YAML Import Test",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "YAML import failed: %s", result.Message)

	importData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(importData, &scene))
	assert.Equal(t, "YAML Import Test", scene.Name)

	// Verify data sources were imported
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/datasources/list", dto.SceneIDRequest{
		SceneID: scene.ID,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	dsListData, _ := json.Marshal(result.Data)
	var dsList []dto.DataSourceDTO
	require.NoError(t, json.Unmarshal(dsListData, &dsList))
	assert.Equal(t, 1, len(dsList), "expected 1 data source")
	assert.Equal(t, "test_users", dsList[0].Name)

	// Verify nodes were imported
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	nodeListData, _ := json.Marshal(result.Data)
	var nodeListResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(nodeListData, &nodeListResp))
	assert.Equal(t, 4, len(nodeListResp.Items), "expected 4 nodes")
}

func TestYAMLImportWithGroupAndTimerNodes(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: group-timer-yaml-test
nodes:
  - name: Start
    type: setup
    config:
      url: /api/start
  - name: Heartbeat
    type: timer
    config:
      mode: interval
      interval: 3000
      duration: 15000
  - name: Sub Step 1
    type: http
    config:
      url: /api/step1
  - name: Sub Step 2
    type: http
    config:
      url: /api/step2
  - name: Login Flow
    type: group
    config:
      node_ids:
        - Sub Step 1
        - Sub Step 2
      loop_count: 3
  - name: Wait
    type: timer
    config:
      mode: delay
      delay: 5000
  - name: End
    type: teardown
    config:
      url: /api/end
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Group Timer YAML Test",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "YAML import failed: %s", result.Message)

	importData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(importData, &scene))

	// Verify all 7 nodes exist
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	nodeListData, _ := json.Marshal(result.Data)
	var nodeListResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(nodeListData, &nodeListResp))
	assert.Equal(t, 7, len(nodeListResp.Items), "expected 7 nodes")

	// Verify node types
	typeCounts := make(map[string]int)
	for _, n := range nodeListResp.Items {
		typeCounts[n.Type]++
	}
	assert.Equal(t, 1, typeCounts["setup"])
	assert.Equal(t, 1, typeCounts["teardown"])
	assert.Equal(t, 2, typeCounts["http"], "expected 2 http nodes")
	assert.Equal(t, 2, typeCounts["timer"], "expected 2 timer nodes")
	assert.Equal(t, 1, typeCounts["group"], "expected 1 group node")

	// Verify group node config has resolved node_ids
	for _, n := range nodeListResp.Items {
		if n.Type == "group" {
			var cfg struct {
				NodeIDs   []string `json:"node_ids"`
				LoopCount int      `json:"loop_count"`
			}
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			assert.Equal(t, 2, len(cfg.NodeIDs), "group should have 2 children")
			assert.Equal(t, 3, cfg.LoopCount, "group loop_count should be 3")
		}
	}

	// Verify timer nodes
	timerDelay := 0
	timerInterval := 0
	for _, n := range nodeListResp.Items {
		if n.Type == "timer" {
			var cfg struct {
				Mode     string `json:"mode"`
				Delay    int    `json:"delay,omitempty"`
				Interval int    `json:"interval,omitempty"`
				Duration int    `json:"duration,omitempty"`
			}
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			if cfg.Mode == "delay" {
				timerDelay++
				assert.Equal(t, 5000, cfg.Delay)
			} else if cfg.Mode == "interval" {
				timerInterval++
				assert.Equal(t, 3000, cfg.Interval)
				assert.Equal(t, 15000, cfg.Duration)
			}
		}
	}
	assert.Equal(t, 1, timerDelay, "expected 1 delay timer")
	assert.Equal(t, 1, timerInterval, "expected 1 interval timer")
}

func TestYAMLImportInvalidGroupChild(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// YAML with group referencing non-existent child
	yamlContent := `
name: invalid-group-test
nodes:
  - name: Bad Group
    type: group
    config:
      node_ids:
        - NonExistentChild
      loop_count: 1
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Invalid Group Test",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code, "should reject import with non-existent child")
	assert.Contains(t, result.Message, "not found")
}

func TestYAMLImportEmptyName(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	// YAML without name and no name in request
	yamlContent := `
description: No name scene
nodes:
  - name: Only Node
    type: http
    config:
      url: /api/test
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 400, result.Code, "should reject YAML without name")
	assert.Contains(t, result.Message, "name is required")
}

// --- 12. Backend YAML Import/Export Extension ---

func TestYAMLImportWithWhileParallelSubFlowLoop(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: yaml-all-types-test
nodes:
  - name: Start
    type: http
    config:
      url: /api/start
  - name: DataFetch
    type: sub_flow
    config:
      scene_id: "sub-scene-123"
  - name: LoopBlock
    type: loop
    config:
      loop_count: 3
  - name: WhileBlock
    type: while
    config:
      exit_conditions:
        - field: $.status
          operator: eq
          value: success
  - name: ParallelBlock
    type: parallel
    config:
      async: true
setup:
  - name: SetupInit
    type: setup
    config:
      url: /api/init
teardown:
  - name: TeardownClean
    type: teardown
    config:
      url: /api/clean
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "YAML All Types",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "import failed: %s", result.Message)

	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))
	assert.Equal(t, "YAML All Types", scene.Name)
	assert.NotZero(t, scene.ID)

	// List nodes and verify all types exist.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))
	nodes := listResp.Items
	assert.Equal(t, 7, len(nodes), "expected 7 nodes (setup + 5 + teardown)")

	typeCount := make(map[string]int)
	for _, n := range nodes {
		typeCount[n.Type]++
	}
	assert.Equal(t, 1, typeCount["http"], "http")
	assert.Equal(t, 1, typeCount["sub_flow"], "sub_flow")
	assert.Equal(t, 1, typeCount["loop"], "loop")
	assert.Equal(t, 1, typeCount["while"], "while")
	assert.Equal(t, 1, typeCount["parallel"], "parallel")
	assert.Equal(t, 1, typeCount["setup"], "setup")
	assert.Equal(t, 1, typeCount["teardown"], "teardown")

	// Verify sub_flow config has scene_id.
	for _, n := range nodes {
		if n.Type == "sub_flow" {
			var cfg map[string]any
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			assert.Equal(t, "sub-scene-123", cfg["scene_id"])
		}
		if n.Type == "loop" {
			var cfg map[string]any
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			// JSON numbers decode as float64
			assert.Equal(t, float64(3), cfg["loop_count"])
		}
		if n.Type == "parallel" {
			var cfg map[string]any
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			assert.Equal(t, true, cfg["async"])
		}
		if n.Type == "while" {
			var cfg map[string]any
			require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))
			conditions, ok := cfg["exit_conditions"].([]any)
			require.True(t, ok, "exit_conditions should be an array")
			require.Equal(t, 1, len(conditions))
			cond := conditions[0].(map[string]any)
			assert.Equal(t, "$.status", cond["field"])
			assert.Equal(t, "eq", cond["operator"])
			assert.Equal(t, "success", cond["value"])
		}
	}
}

func TestYAMLImportWithConfigAndDerivedParams(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: cfg-derived-params-test
config_params:
  cfg_base_url: "http://localhost:8080"
  cfg_timeout: "30s"
derived_params:
  full_url: "${cfg_base_url}/api/v1/users"
  request_id: "${__uuid()}"
nodes:
  - name: GetUsers
    type: http
    config:
      url: "${full_url}"
      method: GET
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Config Derived Params",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "import failed: %s", result.Message)

	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Verify variables stored include config_params and derived_params.
	// Variables are stored as JSON object (map[string]string).
	var varKeys map[string]string
	require.NoError(t, json.Unmarshal([]byte(scene.Variables), &varKeys))
	assert.Equal(t, "http://localhost:8080", varKeys["cfg_base_url"])
	assert.Equal(t, "30s", varKeys["cfg_timeout"])
	assert.Equal(t, "${cfg_base_url}/api/v1/users", varKeys["full_url"])
	assert.Equal(t, "${__uuid()}", varKeys["request_id"])
}

func TestYAMLImportWithThinkTimeRetryConditionTimedTrigger(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	yamlContent := `
name: thinktime-retry-test
nodes:
  - name: GetUsers
    type: http
    config:
      url: /api/users
      method: GET
    think_time: "1000ms"
    retry:
      count: 3
      interval: "500ms"
    condition: "${cfg_retry_count} > 0"
  - name: HealthCheck
    type: http
    config:
      url: /api/health
    timed_trigger: "@every 10s"
`

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "ThinkTime Retry",
		YAML: yamlContent,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "import failed: %s", result.Message)

	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// List nodes and verify merged config.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	listData, _ := json.Marshal(result.Data)
	var listResp dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp))
	nodes := listResp.Items
	assert.Equal(t, 2, len(nodes))

	for _, n := range nodes {
		var cfg map[string]any
		require.NoError(t, json.Unmarshal([]byte(n.Config), &cfg))

		if n.Name == "GetUsers" {
			assert.Equal(t, "1000ms", cfg["think_time"])
			assert.Equal(t, "${cfg_retry_count} > 0", cfg["condition"])

			retry, ok := cfg["retry"].(map[string]any)
			require.True(t, ok, "retry should be an object")
			// JSON numbers decode as float64
			assert.Equal(t, float64(3), retry["count"])
			assert.Equal(t, "500ms", retry["interval"])
		}
		if n.Name == "HealthCheck" {
			assert.Equal(t, "@every 10s", cfg["timed_trigger"])
		}
	}
}

func TestYAMLExportRoundTrip(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	originalYAML := `
name: round-trip-test
config_params:
  cfg_host: "localhost"
derived_params:
  url_template: "${cfg_host}:8080/api"
nodes:
  - name: Start
    type: http
    config:
      url: "${url_template}"
      method: GET
    think_time: "500ms"
    retry:
      count: 2
      interval: "1s"
  - name: LoopStep
    type: loop
    config:
      loop_count: 5
  - name: WhileStep
    type: while
    config:
      exit_conditions:
        - field: $.code
          operator: eq
          value: "200"
  - name: ParallelStep
    type: parallel
    config:
      async: true
  - name: SubFlowStep
    type: sub_flow
    config:
      scene_id: "my-sub-flow"
edges:
  - from: Start
    to: LoopStep
  - from: LoopStep
    to: ParallelStep
  - from: ParallelStep
    to: SubFlowStep
`

	// Step 1: Import.
	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Round Trip Test",
		YAML: originalYAML,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "import failed: %s", result.Message)
	sceneData, _ := json.Marshal(result.Data)
	var scene dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData, &scene))

	// Step 2: Export.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/export", dto.IDRequest{ID: scene.ID})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "export failed: %s", result.Message)

	var exportResp dto.ExportYAMLResponse
	exportData, _ := json.Marshal(result.Data)
	require.NoError(t, json.Unmarshal(exportData, &exportResp))
	assert.NotEmpty(t, exportResp.YAML)

	// Step 3: Re-import the exported YAML into a new scene.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/import", dto.ImportYAMLRequest{
		Name: "Round Trip Re-import",
		YAML: exportResp.YAML,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code, "re-import failed: %s", result.Message)
	sceneData2, _ := json.Marshal(result.Data)
	var scene2 dto.SceneDTO
	require.NoError(t, json.Unmarshal(sceneData2, &scene2))

	// Verify re-imported scene has same nodes.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/nodes/list", dto.ListNodesRequest{
		SceneID: scene2.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	listData, _ := json.Marshal(result.Data)
	var listResp2 dto.ListResponse[[]dto.NodeDTO]
	require.NoError(t, json.Unmarshal(listData, &listResp2))
	nodes2 := listResp2.Items
	assert.Equal(t, 5, len(nodes2), "expected 5 nodes in round-tripped scene")

	typeCount := make(map[string]int)
	for _, n := range nodes2 {
		typeCount[n.Type]++
	}
	assert.Equal(t, 1, typeCount["http"])
	assert.Equal(t, 1, typeCount["loop"])
	assert.Equal(t, 1, typeCount["while"])
	assert.Equal(t, 1, typeCount["parallel"])
	assert.Equal(t, 1, typeCount["sub_flow"])

	// Verify edges are also exported and re-imported.
	resp = postJSONAuth(t, srv, token, "/api/v1/scenes/edges/list", dto.ListEdgesRequest{
		SceneID: scene2.ID,
		Limit:   20,
	})
	result = decodeResponse(t, resp)
	assert.Equal(t, 0, result.Code)
	edgesData, _ := json.Marshal(result.Data)
	var edgesResp dto.ListResponse[[]dto.EdgeDTO]
	require.NoError(t, json.Unmarshal(edgesData, &edgesResp))
	edges2 := edgesResp.Items
	assert.Equal(t, 3, len(edges2), "expected 3 edges in round-tripped scene")
}

func TestYAMLExportSceneNotFound(t *testing.T) {
	srv := newTestServer(t)
	token := getAdminToken(t, srv)

	resp := postJSONAuth(t, srv, token, "/api/v1/scenes/export", dto.IDRequest{
		ID: 99999999,
	})
	result := decodeResponse(t, resp)
	assert.Equal(t, 404, result.Code, "should return 404 for non-existent scene")
	assert.Contains(t, result.Message, "not found")
}
