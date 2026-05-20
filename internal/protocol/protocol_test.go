package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockRequest struct {
	timeout time.Duration
}

func (r *mockRequest) GetTimeout() time.Duration {
	return r.timeout
}

type mockResponse struct {
	statusCode int
	latency    time.Duration
	err        error
}

func (r *mockResponse) GetStatusCode() int {
	return r.statusCode
}

func (r *mockResponse) GetLatency() time.Duration {
	return r.latency
}

func (r *mockResponse) GetError() error {
	return r.err
}

func TestMockRequestImplementsInterface(t *testing.T) {
	var _ Request = &mockRequest{}
}

func TestMockResponseImplementsInterface(t *testing.T) {
	var _ Response = &mockResponse{}
}

func TestRequestGetTimeout(t *testing.T) {
	req := &mockRequest{timeout: 5 * time.Second}
	assert.Equal(t, 5*time.Second, req.GetTimeout())
}

func TestRequestZeroTimeout(t *testing.T) {
	req := &mockRequest{timeout: 0}
	assert.Equal(t, time.Duration(0), req.GetTimeout())
}

func TestResponseGetStatusCode(t *testing.T) {
	resp := &mockResponse{statusCode: 200}
	assert.Equal(t, 200, resp.GetStatusCode())
}

func TestResponseGetLatency(t *testing.T) {
	resp := &mockResponse{latency: 150 * time.Millisecond}
	assert.Equal(t, 150*time.Millisecond, resp.GetLatency())
}

func TestResponseGetError(t *testing.T) {
	resp := &mockResponse{err: nil}
	assert.NoError(t, resp.GetError())
}

type mockProtocol struct{}

func (m *mockProtocol) Execute(ctx context.Context, req Request) (Response, error) {
	return &mockResponse{statusCode: 200, latency: 10 * time.Millisecond}, nil
}

func (m *mockProtocol) Name() string {
	return "mock"
}

func TestProtocolInterface(t *testing.T) {
	var _ Protocol = &mockProtocol{}
}

func TestMockProtocolExecute(t *testing.T) {
	p := &mockProtocol{}
	req := &mockRequest{timeout: 5 * time.Second}

	resp, err := p.Execute(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.GetStatusCode())
	assert.Equal(t, 10*time.Millisecond, resp.GetLatency())
}
