package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
)

func TestSnowflakeIDJSON(t *testing.T) {
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)
	id := node.Generate()

	data, err := json.Marshal(id)
	require.NoError(t, err)

	var decoded string
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, id.String(), decoded)
}

func TestSnowflakeIDUnmarshal(t *testing.T) {
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)
	id := node.Generate()

	data, err := json.Marshal(id)
	require.NoError(t, err)

	var parsed snowflake.ID
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)
	assert.Equal(t, id, parsed)
}

func TestModelIsDeleted(t *testing.T) {
	m := Model{}
	assert.False(t, m.IsDeleted())

	now := time.Now()
	m.DeletedAt = &now
	assert.True(t, m.IsDeleted())
}

func TestSceneJSON(t *testing.T) {
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	s := Scene{
		Model: Model{
			ID:        node.Generate(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		Name:    "test-scene",
		Status:  SceneStatusDraft,
		DAGJSON: `{"nodes":[]}`,
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	idStr, ok := parsed["id"].(string)
	assert.True(t, ok, "id should be a string in JSON")
	assert.Equal(t, s.ID.String(), idStr)
	assert.Equal(t, "test-scene", parsed["name"])
	assert.Equal(t, "draft", parsed["status"])
}

func TestRunRecordJSON(t *testing.T) {
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	rec := RunRecord{
		Model: Model{
			ID:        node.Generate(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		SceneID:     node.Generate(),
		Status:      RunStatusCompleted,
		WorkerCount: 10,
		RunMode:     "count",
		TotalReqs:   1000,
		SuccessReqs: 990,
		FailedReqs:  10,
		AvgLatency:  45.5,
		P50Latency:  40.0,
		P95Latency:  80.0,
		P99Latency:  120.0,
	}

	data, err := json.Marshal(rec)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	sceneIDStr, ok := parsed["scene_id"].(string)
	assert.True(t, ok, "scene_id should be a string in JSON")
	assert.Equal(t, rec.SceneID.String(), sceneIDStr)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "draft", SceneStatusDraft)
	assert.Equal(t, "ready", SceneStatusReady)
	assert.Equal(t, "running", SceneStatusRunning)
	assert.Equal(t, "completed", SceneStatusCompleted)
	assert.Equal(t, "failed", SceneStatusFailed)

	assert.Equal(t, "http", NodeTypeHTTP)
	assert.Equal(t, "delay", NodeTypeDelay)
	assert.Equal(t, "condition", NodeTypeCondition)
	assert.Equal(t, "loop", NodeTypeLoop)
	assert.Equal(t, "group", NodeTypeGroup)

	assert.Equal(t, "global", VariableScopeGlobal)
	assert.Equal(t, "scene", VariableScopeScene)
	assert.Equal(t, "api", VariableScopeAPI)

	assert.Equal(t, "before", PluginPhaseBefore)
	assert.Equal(t, "after", PluginPhaseAfter)

	assert.Equal(t, "success", ReportStatusSuccess)
	assert.Equal(t, "failed", ReportStatusFailed)
	assert.Equal(t, "partial", ReportStatusPartial)

	assert.Equal(t, "running", RunStatusRunning)
	assert.Equal(t, "completed", RunStatusCompleted)
	assert.Equal(t, "failed", RunStatusFailed)
	assert.Equal(t, "cancelled", RunStatusCancelled)
}
