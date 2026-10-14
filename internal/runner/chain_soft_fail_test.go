package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/trace"
)

// chainServer is an httptest server whose response depends on the request
// path: paths in failSet return HTTP 200 with a failing business body
// (errorCode != 0), all other paths return HTTP 200 with a success body.
// Per-path hit counts let tests verify which nodes actually executed.
type chainServer struct {
	*httptest.Server
	mu      sync.Mutex
	hits    map[string]int
	failSet map[string]bool
}

func newChainServer(failSet map[string]bool) *chainServer {
	cs := &chainServer{hits: map[string]int{}, failSet: failSet}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		cs.hits[r.URL.Path]++
		fail := cs.failSet[r.URL.Path]
		cs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if fail {
			_, _ = w.Write([]byte(`{"errorCode": 50028, "msg": "biz error"}`))
		} else {
			_, _ = w.Write([]byte(`{"errorCode": 0, "msg": "ok", "data": {"value": 1}}`))
		}
	}))
	return cs
}

func (cs *chainServer) hit(path string) int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.hits[path]
}

// chainHTTPNode builds a plain HTTP sceneNode for chain-level tests.
// withAssertion controls whether expect_body {errorCode: 0} is configured.
func chainHTTPNode(id, url string, blockOnError bool, withAssertion bool) *sceneNode {
	cfg := map[string]any{"method": "GET", "url": url}
	if withAssertion {
		cfg["expect_body"] = map[string]any{"errorCode": 0}
	}
	cfgJSON, _ := json.Marshal(cfg)
	return &sceneNode{
		id:            id,
		name:          id,
		nodeType:      "http",
		config:        string(cfgJSON),
		loopCount:     1,
		mode:          dag.ExecSync,
		blockOnError:  blockOnError,
		stats:         &Stats{},
		httpOnlyStats: &Stats{},
		nodeStats:     NewNodeStats(10000),
		log:           newTestLogger(),
	}
}

// chainGroupNode builds a group (LOOPS) sceneNode wrapping child nodes.
func chainGroupNode(id string, blockOnError bool, children ...dag.Node) *sceneNode {
	ids := make([]string, 0, len(children))
	for _, c := range children {
		ids = append(ids, c.ID())
	}
	cfgJSON, _ := json.Marshal(map[string]any{
		"node_ids":   ids,
		"loop_count": 1,
		"async":      false,
	})
	return &sceneNode{
		id:           id,
		name:         id,
		nodeType:     "group",
		config:       string(cfgJSON),
		loopCount:    1,
		mode:         dag.ExecSync,
		blockOnError: blockOnError,
		childNodes:   children,
		nodeStats:    NewNodeStats(10000),
		log:          newTestLogger(),
	}
}

// buildChainDAG registers nodes and normal edges A→B→LOOPS→F style.
func buildChainDAG(t *testing.T, nodes []dag.Node, edges [][2]string) *dag.DAG {
	t.Helper()
	d := dag.New()
	for _, n := range nodes {
		require.NoError(t, d.AddNode(n))
	}
	for _, e := range edges {
		require.NoError(t, d.AddEdge(e[0], e[1], dag.EdgeNormal, ""))
	}
	return d
}

// runTracedChain executes the DAG under the tracing executor and returns
// the execution result and the tracer for span/trace assertions.
func runTracedChain(t *testing.T, d *dag.DAG) (error, *trace.Tracer, snowflake.ID) {
	t.Helper()
	tracer, err := trace.NewTracer(trace.Config{BufferSize: 32})
	require.NoError(t, err)
	sceneID := snowflake.ID(101)
	runID := snowflake.ID(202)
	hook := &dag.HookAdapter{Tracer: tracer}
	exec := dag.NewExecutor(d, dag.WithTraceHook(hook, sceneID, runID))
	_, execErr := exec.ExecuteWithTrace(context.Background(), map[string]any{})
	return execErr, tracer, runID
}

// waitTraceStatus polls the tracer until the trace reaches the expected
// status (async goroutines may still be writing spans after return).
func waitTraceStatus(t *testing.T, tracer *trace.Tracer, runID snowflake.ID, want trace.SpanStatus) *trace.Trace {
	t.Helper()
	var got *trace.Trace
	require.Eventually(t, func() bool {
		tr, ok := tracer.ByRunID(runID)
		if !ok {
			return false
		}
		got = tr
		return tr.Status == want
	}, 3*time.Second, 20*time.Millisecond, "trace status should be %s", want)
	return got
}

func spanOf(t *testing.T, tr *trace.Trace, nodeID string) *trace.Span {
	t.Helper()
	for _, s := range tr.Spans {
		if s.NodeID == nodeID {
			return s
		}
	}
	t.Fatalf("span for node %s not found", nodeID)
	return nil
}

// Scenario 1: A hard-fails (HTTP 200, body error, assertion, block_on_error=true)
// → downstream nodes SKIP, trace fails.
func TestChainTrace_AHardFail_SkipsDownstreamAndTraceError(t *testing.T) {
	cs := newChainServer(map[string]bool{"/a": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", true, true)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", false, false)
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", false, C, D, E)
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.Error(t, execErr, "block_on_error=true assertion failure must abort the chain")

	tr := waitTraceStatus(t, tracer, runID, trace.SpanStatusError)
	sA := spanOf(t, tr, "A")
	assert.Equal(t, trace.SpanStatusError, sA.Status)
	assert.Contains(t, sA.Error, "50028")

	// Downstream nodes are skipped, not executed.
	require.Eventually(t, func() bool {
		return spanOf(t, tr, "F").Status == trace.SpanStatusSkip
	}, 2*time.Second, 20*time.Millisecond, "F span should be skip")
	assert.Equal(t, trace.SpanStatusSkip, spanOf(t, tr, "B").Status)
	assert.Equal(t, trace.SpanStatusSkip, spanOf(t, tr, "LOOPS").Status)

	for _, p := range []string{"/b", "/c", "/d", "/e", "/f"} {
		assert.Zero(t, cs.hit(p), "node %s must not execute (skipped)", p)
	}
}

// Scenario 2: A soft-fails (HTTP 200, body error, assertion, block_on_error=false)
// → downstream nodes still execute, trace fails.
func TestChainTrace_ASoftFail_ContinuesAndTraceError(t *testing.T) {
	cs := newChainServer(map[string]bool{"/a": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", false, true)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", false, false)
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", false, C, D, E)
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.NoError(t, execErr, "soft failure must not abort the chain")

	tr := waitTraceStatus(t, tracer, runID, trace.SpanStatusError)
	assert.Equal(t, trace.SpanStatusError, spanOf(t, tr, "A").Status,
		"A span must reflect the soft failure")
	assert.Contains(t, spanOf(t, tr, "A").Error, "50028")
	assert.Equal(t, trace.SpanStatusOK, spanOf(t, tr, "B").Status)
	assert.Equal(t, trace.SpanStatusOK, spanOf(t, tr, "LOOPS").Status)
	assert.Equal(t, trace.SpanStatusOK, spanOf(t, tr, "F").Status)

	// All downstream nodes executed exactly once — none skipped.
	for _, p := range []string{"/a", "/b", "/c", "/d", "/e", "/f"} {
		assert.Equal(t, 1, cs.hit(p), "node %s must execute normally", p)
	}
}

// Scenario 3: A has NO assertion (HTTP 200, body error is invisible to the
// platform) → all nodes execute, trace succeeds — block_on_error is
// irrelevant without an assertion because no failure signal exists.
func TestChainTrace_ANoAssertion_ContinuesAndTraceOk(t *testing.T) {
	for _, block := range []bool{false, true} {
		cs := newChainServer(map[string]bool{"/a": true})

		A := chainHTTPNode("A", cs.URL+"/a", block, false)
		B := chainHTTPNode("B", cs.URL+"/b", false, false)
		C := chainHTTPNode("C", cs.URL+"/c", false, false)
		D := chainHTTPNode("D", cs.URL+"/d", false, false)
		E := chainHTTPNode("E", cs.URL+"/e", false, false)
		LOOPS := chainGroupNode("LOOPS", false, C, D, E)
		F := chainHTTPNode("F", cs.URL+"/f", false, false)

		d := buildChainDAG(t,
			[]dag.Node{A, B, LOOPS, F},
			[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
		)

		execErr, tracer, runID := runTracedChain(t, d)
		require.NoError(t, execErr, "no assertion → no failure signal → chain must succeed (block=%v)", block)

		tr := waitTraceStatus(t, tracer, runID, trace.SpanStatusOK)
		assert.Equal(t, trace.SpanStatusOK, spanOf(t, tr, "A").Status,
			"without assertion the platform cannot perceive the business failure (block=%v)", block)

		for _, p := range []string{"/a", "/b", "/c", "/d", "/e", "/f"} {
			assert.Equal(t, 1, cs.hit(p), "node %s must execute (block=%v)", p, block)
		}
		cs.Close()
	}
}

// Scenario 4a: D (inside LOOPS) soft-fails (assertion, block_on_error=false)
// → E still executes within the group, and the trace must fail because the
// chain outcome follows the node's real result.
func TestChainTrace_GroupChildSoftFail_ChildContinuesButTraceShouldFail(t *testing.T) {
	cs := newChainServer(map[string]bool{"/d": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", false, false)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", false, true)
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", false, C, D, E)
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.NoError(t, execErr, "soft failure inside group must not abort the chain")

	// D's soft failure is real: its node stats recorded the failure.
	assert.EqualValues(t, 1, D.nodeStats.FailedReqs.Load(),
		"D nodeStats must record the soft failure")

	// E and F still execute — the failure must not interrupt the flow.
	assert.Equal(t, 1, cs.hit("/c"))
	assert.Equal(t, 1, cs.hit("/d"))
	assert.Equal(t, 1, cs.hit("/e"), "E must execute after D's soft failure")
	assert.Equal(t, 1, cs.hit("/f"))

	// The trace must reflect the real outcome: failure.
	waitTraceStatus(t, tracer, runID, trace.SpanStatusError)
}

// Scenario 4b: D (inside LOOPS) hard-fails (assertion, block_on_error=true)
// → the group aborts (E never runs) and the whole chain fails.
func TestChainTrace_GroupChildHardFail_AbortsGroupAndChain(t *testing.T) {
	cs := newChainServer(map[string]bool{"/d": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", false, false)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", true, true)
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", false, C, D, E)
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.Error(t, execErr, "hard failure inside group must abort the chain")

	tr := waitTraceStatus(t, tracer, runID, trace.SpanStatusError)
	assert.Equal(t, trace.SpanStatusError, spanOf(t, tr, "LOOPS").Status,
		"LOOPS span must reflect the child hard failure")

	assert.Equal(t, 1, cs.hit("/c"), "C executes before D")
	assert.Equal(t, 1, cs.hit("/d"))
	assert.Zero(t, cs.hit("/e"), "E must not execute after D's hard failure")
	assert.Zero(t, cs.hit("/f"), "F must not execute")
}

// Scenario 4c: D (inside LOOPS) has NO assertion → business body failure is
// invisible → everything executes, trace succeeds.
func TestChainTrace_GroupChildNoAssertion_AllOk(t *testing.T) {
	cs := newChainServer(map[string]bool{"/d": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", false, false)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", false, false) // no assertion
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", false, C, D, E)
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.NoError(t, execErr)

	tr := waitTraceStatus(t, tracer, runID, trace.SpanStatusOK)
	assert.Equal(t, trace.SpanStatusOK, spanOf(t, tr, "LOOPS").Status)

	for _, p := range []string{"/a", "/b", "/c", "/d", "/e", "/f"} {
		assert.Equal(t, 1, cs.hit(p), "node %s must execute", p)
	}
	assert.EqualValues(t, 0, D.nodeStats.FailedReqs.Load(),
		"no assertion → no failure signal → D counts as success")
}

// Scenario 4d: block_on_error is set on A (which succeeds), D soft-fails.
// A's block_on_error is a per-node property and must NOT change D's
// behavior: D's soft failure neither aborts the flow nor is masked.
func TestChainTrace_BlockOnASuccess_DoesNotAffectGroupChild(t *testing.T) {
	cs := newChainServer(map[string]bool{"/d": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", true, false) // block=true, but A succeeds
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", false, true) // soft failure
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", false, C, D, E)
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.NoError(t, execErr, "A succeeded; D's soft failure must not abort the chain")

	assert.Equal(t, 1, cs.hit("/e"), "E must execute after D's soft failure")
	assert.Equal(t, 1, cs.hit("/f"), "F must execute")
	assert.EqualValues(t, 1, D.nodeStats.FailedReqs.Load())

	// The chain outcome follows D's real result regardless of A's config.
	waitTraceStatus(t, tracer, runID, trace.SpanStatusError)
}

// Scenario 4e: block_on_error=true on LOOPS (the group) itself, D soft-fails.
// Trace must still follow the node's real result: failure. This verifies
// whether the group honors its own block_on_error for child soft failures.
func TestChainTrace_BlockOnGroup_IgnoredForChildSoftFail(t *testing.T) {
	cs := newChainServer(map[string]bool{"/d": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", false, false)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", false, true) // soft failure
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", true, C, D, E) // block=true on the group
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.NoError(t, execErr, "group block_on_error only applies when the group itself fails")

	// Soft failure semantics: flow continues (E, F execute) …
	assert.Equal(t, 1, cs.hit("/e"))
	assert.Equal(t, 1, cs.hit("/f"))

	// … and the trace must still fail — the node's real outcome wins.
	waitTraceStatus(t, tracer, runID, trace.SpanStatusError)
}

// Scenario 4f: block_on_error=true on LOOPS, D HARD-fails (block_on_error=true
// on D) → group returns an error → chain fails, E and F never execute.
func TestChainTrace_BlockOnGroup_HardChildFail_FailsChain(t *testing.T) {
	cs := newChainServer(map[string]bool{"/d": true})
	defer cs.Close()

	A := chainHTTPNode("A", cs.URL+"/a", false, false)
	B := chainHTTPNode("B", cs.URL+"/b", false, false)
	C := chainHTTPNode("C", cs.URL+"/c", false, false)
	D := chainHTTPNode("D", cs.URL+"/d", true, true) // hard failure
	E := chainHTTPNode("E", cs.URL+"/e", false, false)
	LOOPS := chainGroupNode("LOOPS", true, C, D, E) // block=true on the group
	F := chainHTTPNode("F", cs.URL+"/f", false, false)

	d := buildChainDAG(t,
		[]dag.Node{A, B, LOOPS, F},
		[][2]string{{"A", "B"}, {"B", "LOOPS"}, {"LOOPS", "F"}},
	)

	execErr, tracer, runID := runTracedChain(t, d)
	require.Error(t, execErr, "child hard failure makes the group fail; with block_on_error=true the chain must fail")

	waitTraceStatus(t, tracer, runID, trace.SpanStatusError)

	assert.Equal(t, 1, cs.hit("/c"))
	assert.Equal(t, 1, cs.hit("/d"))
	assert.Zero(t, cs.hit("/e"), "E must not execute")
	assert.Zero(t, cs.hit("/f"), "F must not execute")
}
