package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestFields(t *testing.T) {
	req := &Request{
		Method:  MethodPost,
		URL:     "http://example.com/api",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"key":"value"}`),
		Timeout: 5 * time.Second,
	}

	assert.Equal(t, MethodPost, req.Method)
	assert.Equal(t, "http://example.com/api", req.URL)
	assert.Equal(t, "application/json", req.Headers["Content-Type"])
	assert.Equal(t, `{"key":"value"}`, string(req.Body))
	assert.Equal(t, 5*time.Second, req.Timeout)
}

func TestResponseFields(t *testing.T) {
	resp := &Response{
		StatusCode: 200,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{"status":"ok"}`),
		Latency:    150 * time.Millisecond,
		Error:      nil,
	}

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Headers["Content-Type"][0])
	assert.Equal(t, `{"status":"ok"}`, string(resp.Body))
	assert.Equal(t, 150*time.Millisecond, resp.Latency)
	assert.NoError(t, resp.Error)
}

func TestResponseIsError(t *testing.T) {
	resp := &Response{StatusCode: 500}
	assert.True(t, resp.IsError())

	resp = &Response{StatusCode: 200}
	assert.False(t, resp.IsError())

	resp = &Response{StatusCode: 404}
	assert.True(t, resp.IsError())
}

func TestResponseIsSuccess(t *testing.T) {
	resp := &Response{StatusCode: 200}
	assert.True(t, resp.IsSuccess())

	resp = &Response{StatusCode: 201}
	assert.True(t, resp.IsSuccess())

	resp = &Response{StatusCode: 500}
	assert.False(t, resp.IsSuccess())
}

func TestMethodStrings(t *testing.T) {
	assert.Equal(t, Method("GET"), MethodGet)
	assert.Equal(t, Method("POST"), MethodPost)
	assert.Equal(t, Method("PUT"), MethodPut)
	assert.Equal(t, Method("DELETE"), MethodDelete)
	assert.Equal(t, Method("PATCH"), MethodPatch)
	assert.Equal(t, Method("HEAD"), MethodHead)
}

func TestProtocolInterface(t *testing.T) {
	// Verify that mockProtocol implements the Protocol interface.
	var _ Protocol = &mockProtocol{}
}

type mockProtocol struct{}

func (m *mockProtocol) Execute(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		StatusCode: 200,
		Body:       []byte(`{"mock":true}`),
		Latency:    10 * time.Millisecond,
	}, nil
}

func (m *mockProtocol) Name() string {
	return "mock"
}

func TestMockProtocolExecute(t *testing.T) {
	p := &mockProtocol{}
	req := &Request{
		Method: MethodGet,
		URL:    "http://example.com",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, `{"mock":true}`, string(resp.Body))
}

func TestRequestWithVariables(t *testing.T) {
	req := &Request{
		Method:  MethodPost,
		URL:     "http://${host}/api/${version}",
		Headers: map[string]string{"Authorization": "Bearer ${token}"},
		Body:    []byte(`{"user":"${user}"}`),
	}

	assert.Contains(t, req.URL, "${host}")
	assert.Contains(t, req.Headers["Authorization"], "${token}")
	assert.Contains(t, string(req.Body), "${user}")
}

func TestResponseBodyJSON(t *testing.T) {
	resp := &Response{
		StatusCode: 200,
		Body:       []byte(`{"name":"salvo","version":1}`),
	}

	var result map[string]any
	require.NoError(t, resp.BodyJSON(&result))
	assert.Equal(t, "salvo", result["name"])
	assert.Equal(t, float64(1), result["version"])
}

func TestResponseBodyJSONError(t *testing.T) {
	resp := &Response{
		StatusCode: 200,
		Body:       []byte(`invalid json`),
	}

	var result map[string]any
	assert.Error(t, resp.BodyJSON(&result))
}
