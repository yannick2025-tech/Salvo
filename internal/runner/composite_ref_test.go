package runner

// composite_ref_test.go — TDD tests for the unified "node_ids reference" model
// of composite nodes (group/while/loop). Children are real scene nodes
// referenced by ID/name; while/loop execute them via runChildChain with
// variable-scope refresh (SnapshotVariables) so extracts performed by one child
// drive exit conditions and downstream children.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

// --- minimal repo fakes for buildDAG tests ---

type fakeNodeRepo struct{ nodes []*model.Node }

func (f *fakeNodeRepo) Create(context.Context, *model.Node) error { return nil }
func (f *fakeNodeRepo) GetByID(context.Context, snowflake.ID) (*model.Node, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeNodeRepo) List(context.Context, repo.Filter) ([]*model.Node, error) {
	return f.nodes, nil
}
func (f *fakeNodeRepo) Update(context.Context, *model.Node) error  { return nil }
func (f *fakeNodeRepo) Delete(context.Context, snowflake.ID) error { return nil }

type fakeEdgeRepo struct{ edges []*model.Edge }

func (f *fakeEdgeRepo) Create(context.Context, *model.Edge) error { return nil }
func (f *fakeEdgeRepo) GetByID(context.Context, snowflake.ID) (*model.Edge, error) {
	return nil, fmt.Errorf("not implemented")
}
func (f *fakeEdgeRepo) List(context.Context, repo.Filter) ([]*model.Edge, error) {
	return f.edges, nil
}
func (f *fakeEdgeRepo) Update(context.Context, *model.Edge) error  { return nil }
func (f *fakeEdgeRepo) Delete(context.Context, snowflake.ID) error { return nil }

func newRefTestRunner(nodes []*model.Node, edges []*model.Edge) *Runner {
	return &Runner{
		cfg:           Config{SceneID: 101},
		ctx:           context.Background(),
		log:           newTestLogger(),
		nodes:         &fakeNodeRepo{nodes: nodes},
		edges:         &fakeEdgeRepo{edges: edges},
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		nodeStats:     map[string]*NodeStats{},
		runID:         202,
	}
}

// TestExecuteWhileNodeIDsRefMode verifies the reference mode end-to-end: the
// while node runs its child chain each iteration, the child's extract writes
// into the shared variable scope, and the refreshed variables drive the exit
// condition (this fails without the SnapshotVariables refresh).
func TestExecuteWhileNodeIDsRefMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		status := "1"
		if calls >= 2 {
			status = "4"
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"status":"%s"}}`, status)))
	}))
	defer server.Close()

	// Child HTTP node whose extract writes chargingStatus into the shared scope.
	childCfg, err := json.Marshal(map[string]any{
		"method":  "GET",
		"url":     server.URL + "/status",
		"extract": map[string]string{"chargingStatus": "$.result.status"},
	})
	require.NoError(t, err)
	child := &sceneNode{
		id:            "child-http",
		name:          "child-http",
		nodeType:      "http",
		config:        string(childCfg),
		loopCount:     1,
		mode:          dag.ExecSync,
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		nodeStats:     NewNodeStats(10000),
		log:           newTestLogger(),
	}

	whileCfg, err := json.Marshal(map[string]any{
		"exit_conditions": []map[string]string{{"variable": "chargingStatus", "operator": "equals", "value": "4"}},
		"max_iterations":  5,
	})
	require.NoError(t, err)
	wn := newTestSceneNode()
	wn.id = "while-ref"
	wn.nodeType = "while"
	wn.config = string(whileCfg)
	wn.childNodes = []dag.Node{child}

	exec := dag.NewExecutor(dag.New(), dag.WithInitialVars(map[string]any{}))
	input := &dag.Input{Variables: map[string]any{}, Executor: exec}

	output, err := wn.executeWhile(ctx, input, wn.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	assert.Equal(t, 2, gotCalls, "loop must exit once the child extracts status=4 (2 iterations)")

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "while", resp["type"])
	assert.Equal(t, 2, resp["iterations"])
}

// TestExecuteWhileRefModeTakesPrecedenceOverSteps verifies that when both
// node_ids children and embedded steps are configured, the reference mode wins.
func TestExecuteWhileRefModeTakesPrecedenceOverSteps(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mu sync.Mutex
	refHits, stepsHits := 0, 0
	refServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		refHits++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"status":"4"}}`))
	}))
	defer refServer.Close()
	stepsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		stepsHits++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"status":"4"}}`))
	}))
	defer stepsServer.Close()

	childCfg, err := json.Marshal(map[string]any{
		"method":  "GET",
		"url":     refServer.URL + "/status",
		"extract": map[string]string{"chargingStatus": "$.result.status"},
	})
	require.NoError(t, err)
	child := &sceneNode{
		id:            "child-http",
		nodeType:      "http",
		config:        string(childCfg),
		loopCount:     1,
		mode:          dag.ExecSync,
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		nodeStats:     NewNodeStats(10000),
		log:           newTestLogger(),
	}

	// Config carries BOTH embedded steps and node_ids children.
	whileCfg, err := json.Marshal(whileConfig{
		ExitConditions: []exitCondition{{Variable: "chargingStatus", Operator: "equals", Value: "4"}},
		MaxIterations:  5,
		Steps: []stepConfig{
			{
				Name:    "legacy step",
				Request: &stepRequestConfig{Method: "GET", URL: stepsServer.URL + "/status"},
			},
		},
	})
	require.NoError(t, err)
	wn := newTestSceneNode()
	wn.id = "while-both"
	wn.nodeType = "while"
	wn.config = string(whileCfg)
	wn.childNodes = []dag.Node{child}

	exec := dag.NewExecutor(dag.New(), dag.WithInitialVars(map[string]any{}))
	input := &dag.Input{Variables: map[string]any{}, Executor: exec}

	output, err := wn.executeWhile(ctx, input, wn.log)
	require.NoError(t, err)
	require.NotNil(t, output)

	mu.Lock()
	gotRef, gotSteps := refHits, stepsHits
	mu.Unlock()
	assert.Equal(t, 1, gotRef, "reference-mode child must execute")
	assert.Equal(t, 0, gotSteps, "embedded steps must NOT execute when node_ids children exist")
}

// TestExecuteLoopNodeIDsRefMode verifies the loop reference mode: loop_count
// iterations of the child chain, with group-style soft failure semantics.
func TestExecuteLoopNodeIDsRefMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cs := newChainServer(map[string]bool{"/d": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", false, true) // soft failure (assertion, block_on_error=false)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)

	loopCfg, err := json.Marshal(map[string]any{
		"loop_count": 3,
		"node_ids":   []string{"A", "D", "B"},
	})
	require.NoError(t, err)
	ln := newTestSceneNode()
	ln.id = "loop-ref"
	ln.nodeType = "loop"
	ln.config = string(loopCfg)
	ln.childNodes = []dag.Node{A, D, B}

	output, err := ln.executeLoop(ctx, &dag.Input{}, ln.log)
	require.NoError(t, err, "soft failure inside loop must not abort")
	require.NotNil(t, output)

	// All children executed exactly loop_count times, including after D's soft failure.
	assert.Equal(t, 3, cs.hit("/a"))
	assert.Equal(t, 3, cs.hit("/d"))
	assert.Equal(t, 3, cs.hit("/b"), "B must execute after D's soft failure in every iteration")

	resp, ok := output.Response.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "loop", resp["type"])
	assert.Equal(t, 3, resp["iterations"])

	// Group-style semantics: the first child soft failure surfaces on Output.Error.
	assert.Error(t, output.Error, "loop must surface the child soft failure")
}

// TestBuildDAGCompositeChildren verifies buildDAG for the reference model:
// while/loop children are excluded from the main DAG topology, mounted on the
// composite node's childNodes, and nesting of composite types inside a loop
// body is rejected.
func TestBuildDAGCompositeChildren(t *testing.T) {
	sceneID := snowflake.ID(101)
	mkNode := func(id int64, name, ntype, config string) *model.Node {
		return &model.Node{
			Model:   model.Model{ID: snowflake.ID(id)},
			SceneID: sceneID,
			Name:    name,
			Type:    ntype,
			Config:  config,
		}
	}
	httpCfg := `{"method":"GET","url":"http://example.com/api"}`

	whileCfg := fmt.Sprintf(`{"node_ids":["%s","%s"],"exit_conditions":[{"variable":"x","operator":"equals","value":"1"}],"max_iterations":3}`,
		"child-a", "child-b")
	loopCfg := `{"node_ids":["child-c"],"loop_count":2}`

	nodes := []*model.Node{
		mkNode(1, "start", model.NodeTypeHTTP, httpCfg),
		mkNode(2, "child-a", model.NodeTypeHTTP, httpCfg),
		mkNode(3, "child-b", model.NodeTypeHTTP, httpCfg),
		mkNode(4, "child-c", model.NodeTypeHTTP, httpCfg),
		mkNode(5, "poll", model.NodeTypeWhile, whileCfg),
		mkNode(6, "repeat", model.NodeTypeLoop, loopCfg),
	}
	edges := []*model.Edge{
		{Model: model.Model{ID: 10}, SceneID: sceneID, FromNode: snowflake.ID(1), ToNode: snowflake.ID(5)},
		{Model: model.Model{ID: 11}, SceneID: sceneID, FromNode: snowflake.ID(5), ToNode: snowflake.ID(6)},
	}

	r := newRefTestRunner(nodes, edges)
	d, err := r.buildDAG(&model.Scene{Model: model.Model{ID: sceneID}})
	require.NoError(t, err)

	// Composite children are excluded from the main DAG topology.
	for _, excluded := range []string{"2", "3", "4"} {
		_, ok := d.Node(excluded)
		assert.False(t, ok, "composite child %s must not be an independent DAG node", excluded)
	}
	// Composite nodes themselves remain in the DAG.
	for _, included := range []string{"1", "5", "6"} {
		_, ok := d.Node(included)
		assert.True(t, ok, "node %s must be in the DAG", included)
	}

	// Children are mounted on the composite nodes.
	whileDagNode, ok := d.Node("5")
	require.True(t, ok)
	wn, ok := whileDagNode.(*sceneNode)
	require.True(t, ok)
	require.Len(t, wn.childNodes, 2)
	assert.Equal(t, "child-a", wn.childNodes[0].(*sceneNode).name)
	assert.Equal(t, "child-b", wn.childNodes[1].(*sceneNode).name)

	loopDagNode, ok := d.Node("6")
	require.True(t, ok)
	ln, ok := loopDagNode.(*sceneNode)
	require.True(t, ok)
	require.Len(t, ln.childNodes, 1)
	assert.Equal(t, "child-c", ln.childNodes[0].(*sceneNode).name)
}

// TestBuildDAGWhileChildNestingForbidden verifies that while/loop nodes cannot
// reference composite (group/while/loop) children.
func TestBuildDAGWhileChildNestingForbidden(t *testing.T) {
	sceneID := snowflake.ID(101)
	mkNode := func(id int64, name, ntype, config string) *model.Node {
		return &model.Node{
			Model:   model.Model{ID: snowflake.ID(id)},
			SceneID: sceneID,
			Name:    name,
			Type:    ntype,
			Config:  config,
		}
	}

	whileCfg := `{"node_ids":["inner-group"],"exit_conditions":[{"variable":"x","operator":"equals","value":"1"}],"max_iterations":3}`
	groupCfg := `{"node_ids":[],"loop_count":1}`

	nodes := []*model.Node{
		mkNode(1, "outer-while", model.NodeTypeWhile, whileCfg),
		mkNode(2, "inner-group", model.NodeTypeGroup, groupCfg),
	}

	r := newRefTestRunner(nodes, nil)
	_, err := r.buildDAG(&model.Scene{Model: model.Model{ID: sceneID}})
	require.Error(t, err, "while node referencing a group child must fail DAG construction")
	assert.Contains(t, err.Error(), "cannot contain")
}
