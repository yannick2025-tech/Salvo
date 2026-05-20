package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yannick2025-tech/Salvo/internal/protocol"
)

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	mux.HandleFunc("/api/status/404", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})

	mux.HandleFunc("/api/status/500", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	})

	mux.HandleFunc("/api/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	mux.HandleFunc("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"slow":true}`))
	})

	mux.HandleFunc("/api/method", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"method":"` + r.Method + `"}`))
	})

	return httptest.NewServer(mux)
}

func asHTTPResp(t *testing.T, resp protocol.Response) *HTTPResponse {
	t.Helper()
	require.NotNil(t, resp)
	httpResp, ok := resp.(*HTTPResponse)
	require.True(t, ok, "expected *HTTPResponse, got %T", resp)
	return httpResp
}

func TestHTTPProtocolGet(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodGet,
		URL:    srv.URL + "/api/echo",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)
	assert.NoError(t, hr.Error)
}

func TestHTTPProtocolPost(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method:  MethodPost,
		URL:     srv.URL + "/api/echo",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"hello":"world"}`),
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)
	assert.Equal(t, `{"hello":"world"}`, string(hr.Body))
}

func TestHTTPProtocolPut(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodPut,
		URL:    srv.URL + "/api/method",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)

	var result map[string]string
	require.NoError(t, hr.BodyJSON(&result))
	assert.Equal(t, "PUT", result["method"])
}

func TestHTTPProtocolDelete(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodDelete,
		URL:    srv.URL + "/api/method",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)

	var result map[string]string
	require.NoError(t, hr.BodyJSON(&result))
	assert.Equal(t, "DELETE", result["method"])
}

func TestHTTPProtocolPatch(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodPatch,
		URL:    srv.URL + "/api/method",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)

	var result map[string]string
	require.NoError(t, hr.BodyJSON(&result))
	assert.Equal(t, "PATCH", result["method"])
}

func TestHTTPProtocolHeaders(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodGet,
		URL:    srv.URL + "/api/headers",
		Headers: map[string]string{
			"X-Request-ID": "test-123",
		},
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)
	assert.Contains(t, hr.Headers["X-Custom"], "value")
}

func TestHTTPProtocol404(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodGet,
		URL:    srv.URL + "/api/status/404",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 404, hr.StatusCode)
	assert.True(t, hr.IsError())
	assert.False(t, hr.IsSuccess())
}

func TestHTTPProtocol500(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodGet,
		URL:    srv.URL + "/api/status/500",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 500, hr.StatusCode)
	assert.True(t, hr.IsError())
}

func TestHTTPProtocolTimeout(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method:  MethodGet,
		URL:     srv.URL + "/api/slow",
		Timeout: 50 * time.Millisecond,
	}

	resp, err := p.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestHTTPProtocolContextCancellation(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodGet,
		URL:    srv.URL + "/api/slow",
	}

	resp, err := p.Execute(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestHTTPProtocolLatency(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodGet,
		URL:    srv.URL + "/api/echo",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.GreaterOrEqual(t, hr.Latency, time.Microsecond)
	assert.Less(t, hr.Latency, 5*time.Second)
}

func TestHTTPProtocolName(t *testing.T) {
	p := NewProtocol()
	assert.Equal(t, "http", p.Name())
}

func TestHTTPProtocolInvalidURL(t *testing.T) {
	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodGet,
		URL:    "http://invalid-host-that-does-not-exist.local/api",
	}

	resp, err := p.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestHTTPProtocolWithClient(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	client := &http.Client{Timeout: 10 * time.Second}
	p := NewProtocol(WithClient(client))

	req := &HTTPRequest{
		Method: MethodGet,
		URL:    srv.URL + "/api/echo",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)
}

func TestHTTPRequestGetTimeout(t *testing.T) {
	req := &HTTPRequest{Timeout: 5 * time.Second}
	assert.Equal(t, 5*time.Second, req.GetTimeout())
}

func TestHTTPResponseInterfaceCompliance(t *testing.T) {
	var _ protocol.Response = &HTTPResponse{}
}

func TestHTTPProtocolWrongRequestType(t *testing.T) {
	p := NewProtocol()

	resp, err := p.Execute(context.Background(), &mockBadRequest{})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

type mockBadRequest struct{}

func (m *mockBadRequest) GetTimeout() time.Duration { return 0 }
