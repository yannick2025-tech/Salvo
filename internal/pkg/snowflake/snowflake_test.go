package snowflake

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewNode(t *testing.T) {
	tests := []struct {
		name    string
		nodeID  int64
		wantErr bool
	}{
		{name: "valid node 0", nodeID: 0, wantErr: false},
		{name: "valid node 1", nodeID: 1, wantErr: false},
		{name: "valid max node 1023", nodeID: 1023, wantErr: false},
		{name: "invalid negative node", nodeID: -1, wantErr: true},
		{name: "invalid node over max", nodeID: 1024, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := NewNode(tt.nodeID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, node)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, node)
			}
		})
	}
}

func TestGenerateUniqueIDs(t *testing.T) {
	node, err := NewNode(1)
	assert.NoError(t, err)

	ids := make(map[ID]bool)
	count := 10000

	for i := 0; i < count; i++ {
		id := node.Generate()
		assert.False(t, ids[id], "duplicate ID generated: %d", id)
		ids[id] = true
	}
	assert.Equal(t, count, len(ids))
}

func TestGenerateUniqueIDsAcrossGoroutines(t *testing.T) {
	node, err := NewNode(1)
	assert.NoError(t, err)

	var mu sync.Mutex
	ids := make(map[ID]bool)
	var wg sync.WaitGroup

	goroutines := 10
	idsPerGoroutine := 1000

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < idsPerGoroutine; i++ {
				id := node.Generate()
				mu.Lock()
				ids[id] = true
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, goroutines*idsPerGoroutine, len(ids))
}

func TestIDMarshalJSONAsString(t *testing.T) {
	node, err := NewNode(1)
	assert.NoError(t, err)

	id := node.Generate()

	data, err := json.Marshal(id)
	assert.NoError(t, err)

	expected := `"` + id.String() + `"`
	assert.Equal(t, expected, string(data))
}

func TestIDUnmarshalJSONFromString(t *testing.T) {
	node, err := NewNode(1)
	assert.NoError(t, err)

	original := node.Generate()

	data, err := json.Marshal(original)
	assert.NoError(t, err)

	var decoded ID
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestIDUnmarshalJSONFromNumber(t *testing.T) {
	// Test unmarshaling from JSON number (not string)
	// This is important for backward compatibility with old data
	jsonWithNumber := `1234567890123456789`

	var decoded ID
	err := json.Unmarshal([]byte(jsonWithNumber), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, ID(1234567890123456789), decoded)
}

func TestIDUnmarshalJSONInStructWithNumber(t *testing.T) {
	// Test unmarshaling struct with numeric ID fields
	type Sample struct {
		ID   ID     `json:"id"`
		Name string `json:"name"`
	}

	// JSON with numeric ID (not string)
	jsonData := `{"id":1234567890123456789,"name":"test"}`

	var s Sample
	err := json.Unmarshal([]byte(jsonData), &s)
	assert.NoError(t, err)
	assert.Equal(t, ID(1234567890123456789), s.ID)
	assert.Equal(t, "test", s.Name)
}

func TestIDMarshalInStruct(t *testing.T) {
	node, err := NewNode(1)
	assert.NoError(t, err)

	type Sample struct {
		ID   ID        `json:"id"`
		Name string    `json:"name"`
		Time time.Time `json:"time"`
	}

	s := Sample{
		ID:   node.Generate(),
		Name: "test",
		Time: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(s)
	assert.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)

	idStr, ok := result["id"].(string)
	assert.True(t, ok, "ID should be serialized as string, got %T", result["id"])
	assert.Equal(t, s.ID.String(), idStr)
}

func TestIDComponents(t *testing.T) {
	node, err := NewNode(1)
	assert.NoError(t, err)

	id := node.Generate()

	assert.True(t, id.Time() > 0)
	assert.Equal(t, int64(1), id.NodeID())
	assert.True(t, id.Sequence() >= 0)
}

func TestIDString(t *testing.T) {
	node, err := NewNode(1)
	assert.NoError(t, err)

	id := node.Generate()
	str := id.String()
	assert.NotEmpty(t, str)

	var parsed ID
	err = parsed.Parse(str)
	assert.NoError(t, err)
	assert.Equal(t, id, parsed)
}

func TestIDParseInvalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "non-numeric", input: "abc"},
		{name: "negative number", input: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var id ID
			err := id.Parse(tt.input)
			assert.Error(t, err)
		})
	}
}
