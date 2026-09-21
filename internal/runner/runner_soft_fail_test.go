package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/core/dag"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
)

// newHTTPTestNode builds a sceneNode of type http pointing at ts.
func newHTTPTestNode(config string, blockOnError bool, nodeStat *NodeStats) *sceneNode {
	if nodeStat == nil {
		nodeStat = &NodeStats{}
	}
	return &sceneNode{
		id:           "test-node",
		name:         "TestNode",
		nodeType:     model.NodeTypeHTTP,
		config:       config,
		loopCount:    1,
		mode:         dag.ExecSync,
		blockOnError: blockOnError,
		nodeStats:    nodeStat,
		log:          newTestLogger(),
	}
}

// TestExecuteHTTP_AssertionSoftFail verifies that an expect_body assertion
// failure with block_on_error=false is a soft failure: no error returned,
// Output.Error carries the assertion detail, Response is preserved.
func TestExecuteHTTP_AssertionSoftFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorCode": 50028, "msg": "biz error"}`))
	}))
	defer ts.Close()

	node := newHTTPTestNode(
		`{"method":"POST","url":"`+ts.URL+`","expect_body":{"errorCode":0}}`,
		false, nil,
	)

	out, err := node.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err, "soft failure must not return an error")
	require.NotNil(t, out)
	require.Error(t, out.Error, "Output.Error must carry the assertion failure")
	assert.Contains(t, out.Error.Error(), "errorCode")
	assert.Contains(t, out.Error.Error(), "50028")
	assert.NotNil(t, out.Response, "Response must be preserved for downstream variables")
}

// TestExecuteHTTP_AssertionHardFail verifies that with block_on_error=true
// an assertion failure remains a hard failure (Execute returns error).
func TestExecuteHTTP_AssertionHardFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorCode": 50028}`))
	}))
	defer ts.Close()

	node := newHTTPTestNode(
		`{"method":"POST","url":"`+ts.URL+`","expect_body":{"errorCode":0}}`,
		true, nil,
	)

	out, err := node.Execute(context.Background(), &dag.Input{})
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "errorCode")
}

// TestExecuteHTTP_Non2xxSoftFailCarriesError verifies that a non-2xx response
// with block_on_error=false continues the flow but carries Output.Error.
func TestExecuteHTTP_Non2xxSoftFailCarriesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal error`))
	}))
	defer ts.Close()

	node := newHTTPTestNode(
		`{"method":"GET","url":"`+ts.URL+`"}`,
		false, nil,
	)

	out, err := node.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err, "non-2xx soft failure must not return an error")
	require.NotNil(t, out)
	require.Error(t, out.Error, "Output.Error must carry the HTTP error")
	assert.True(t, strings.HasPrefix(out.Error.Error(), "HTTP 5"), "error should start with HTTP status, got: %s", out.Error.Error())
}

// TestExecuteHTTP_AssertionSoftFailRecordsNodeStats verifies the node-level
// stats record a failure for a soft-failed assertion (aligned with trace).
func TestExecuteHTTP_AssertionSoftFailRecordsNodeStats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorCode": 50028}`))
	}))
	defer ts.Close()

	stat := &NodeStats{}
	node := newHTTPTestNode(
		`{"method":"POST","url":"`+ts.URL+`","expect_body":{"errorCode":0}}`,
		false, stat,
	)

	_, err := node.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err)

	assert.Equal(t, int64(1), stat.TotalReqs.Load(), "total requests should be recorded")
	assert.Equal(t, int64(1), stat.FailedReqs.Load(), "assertion soft-failure must be counted as failed")
	assert.Equal(t, int64(0), stat.SuccessReqs.Load(), "assertion soft-failure must not count as success")
}

// TestExecuteHTTP_AssertionPassKeepsNoError guards the happy path: passed
// assertions leave Output.Error empty.
func TestExecuteHTTP_AssertionPassKeepsNoError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorCode": 0}`))
	}))
	defer ts.Close()

	node := newHTTPTestNode(
		`{"method":"POST","url":"`+ts.URL+`","expect_body":{"errorCode":0}}`,
		false, nil,
	)

	out, err := node.Execute(context.Background(), &dag.Input{})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.NoError(t, out.Error)
}

var _ = logger.Logger(nil) // keep logger import for consistency with other tests
