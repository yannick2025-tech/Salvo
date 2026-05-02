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

func TestHTTPProtocolGet(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodGet,
		URL:    srv.URL + "/api/echo",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.NoError(t, resp.Error)
}

func TestHTTPProtocolPost(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method:  protocol.MethodPost,
		URL:     srv.URL + "/api/echo",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"hello":"world"}`),
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, `{"hello":"world"}`, string(resp.Body))
}

func TestHTTPProtocolPut(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodPut,
		URL:    srv.URL + "/api/method",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]string
	require.NoError(t, resp.BodyJSON(&result))
	assert.Equal(t, "PUT", result["method"])
}

func TestHTTPProtocolDelete(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodDelete,
		URL:    srv.URL + "/api/method",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]string
	require.NoError(t, resp.BodyJSON(&result))
	assert.Equal(t, "DELETE", result["method"])
}

func TestHTTPProtocolPatch(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodPatch,
		URL:    srv.URL + "/api/method",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]string
	require.NoError(t, resp.BodyJSON(&result))
	assert.Equal(t, "PATCH", result["method"])
}

func TestHTTPProtocolHeaders(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodGet,
		URL:    srv.URL + "/api/headers",
		Headers: map[string]string{
			"X-Request-ID": "test-123",
		},
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Headers["X-Custom"], "value")
}

func TestHTTPProtocol404(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodGet,
		URL:    srv.URL + "/api/status/404",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	assert.True(t, resp.IsError())
	assert.False(t, resp.IsSuccess())
}

func TestHTTPProtocol500(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodGet,
		URL:    srv.URL + "/api/status/500",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.True(t, resp.IsError())
}

func TestHTTPProtocolTimeout(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	p := NewProtocol()
	req := &protocol.Request{
		Method:  protocol.MethodGet,
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
	req := &protocol.Request{
		Method: protocol.MethodGet,
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
	req := &protocol.Request{
		Method: protocol.MethodGet,
		URL:    srv.URL + "/api/echo",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Latency, time.Microsecond)
	assert.Less(t, resp.Latency, 5*time.Second)
}

func TestHTTPProtocolName(t *testing.T) {
	p := NewProtocol()
	assert.Equal(t, "http", p.Name())
}

func TestHTTPProtocolInvalidURL(t *testing.T) {
	p := NewProtocol()
	req := &protocol.Request{
		Method: protocol.MethodGet,
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

	req := &protocol.Request{
		Method: protocol.MethodGet,
		URL:    srv.URL + "/api/echo",
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
