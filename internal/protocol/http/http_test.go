package http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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

// --- multipart/form-data tests ---

// writeMultipartServer returns a test server that parses multipart/form-data
// requests and echoes back the received fields and file names.
func writeMultipartServer(t *testing.T, captured *multipartCapture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("parse multipart: " + err.Error()))
			return
		}
		captured.mu.Lock()
		defer captured.mu.Unlock()
		captured.contentType = r.Header.Get("Content-Type")
		for k, vs := range r.MultipartForm.Value {
			if len(vs) > 0 {
				captured.fields[k] = vs[0]
			}
		}
		for field, headers := range r.MultipartForm.File {
			if len(headers) > 0 {
				captured.files[field] = headers[0].Filename
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
}

type multipartCapture struct {
	mu          sync.Mutex
	contentType string
	fields      map[string]string
	files       map[string]string
}

func newMultipartCapture() *multipartCapture {
	return &multipartCapture{
		fields: make(map[string]string),
		files:  make(map[string]string),
	}
}

func TestHTTPProtocolMultipartFieldsOnly(t *testing.T) {
	cap := newMultipartCapture()
	srv := writeMultipartServer(t, cap)
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodPost,
		URL:    srv.URL + "/api/upload",
		Form: &FormData{
			Fields: map[string]string{
				"chargeSeq": "CS-001",
				"comment":   "good service",
			},
		},
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Contains(t, cap.contentType, "multipart/form-data")
	assert.Equal(t, "CS-001", cap.fields["chargeSeq"])
	assert.Equal(t, "good service", cap.fields["comment"])
	assert.Empty(t, cap.files)
}

func TestHTTPProtocolMultipartFileUpload(t *testing.T) {
	// Create a temp file mimicking assets/comments.jpg
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "comments.jpg")
	imgContent := []byte("FAKE-JPEG-DATA")
	require.NoError(t, os.WriteFile(imgPath, imgContent, 0644))

	cap := newMultipartCapture()
	srv := writeMultipartServer(t, cap)
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodPost,
		URL:    srv.URL + "/api/upload",
		Form: &FormData{
			Fields: map[string]string{
				"chargeSeq": "CS-002",
			},
			Files: map[string]string{
				"photo": imgPath,
			},
		},
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Contains(t, cap.contentType, "multipart/form-data")
	assert.Equal(t, "CS-002", cap.fields["chargeSeq"])
	assert.Equal(t, "comments.jpg", cap.files["photo"])
}

func TestHTTPProtocolMultipartFormOverridesBody(t *testing.T) {
	cap := newMultipartCapture()
	srv := writeMultipartServer(t, cap)
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodPost,
		URL:    srv.URL + "/api/upload",
		Body:   []byte(`{"should":"be ignored"}`),
		Form: &FormData{
			Fields: map[string]string{"used": "yes"},
		},
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	hr := asHTTPResp(t, resp)
	assert.Equal(t, 200, hr.StatusCode)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Contains(t, cap.contentType, "multipart/form-data")
	assert.Equal(t, "yes", cap.fields["used"])
}

func TestHTTPProtocolMultipartFileMissing(t *testing.T) {
	srv := writeMultipartServer(t, newMultipartCapture())
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodPost,
		URL:    srv.URL + "/api/upload",
		Form: &FormData{
			Files: map[string]string{
				"photo": "/no/such/file.jpg",
			},
		},
	}

	resp, err := p.Execute(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "no such file")
}

func TestMultipartMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.txt")
	fileB := filepath.Join(tmpDir, "b.jpg")
	require.NoError(t, os.WriteFile(fileA, []byte("AAA"), 0644))
	require.NoError(t, os.WriteFile(fileB, []byte("BBB"), 0644))

	cap := newMultipartCapture()
	srv := writeMultipartServer(t, cap)
	defer srv.Close()

	p := NewProtocol()
	req := &HTTPRequest{
		Method: MethodPost,
		URL:    srv.URL + "/api/upload",
		Form: &FormData{
			Fields: map[string]string{"seq": "M-1"},
			Files: map[string]string{
				"doc":   fileA,
				"image": fileB,
			},
		},
	}

	resp, err := p.Execute(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, asHTTPResp(t, resp).StatusCode)

	cap.mu.Lock()
	defer cap.mu.Unlock()
	assert.Equal(t, "a.txt", cap.files["doc"])
	assert.Equal(t, "b.jpg", cap.files["image"])
	assert.Equal(t, "M-1", cap.fields["seq"])
}

// Ensure body bytes are readable when Form is nil (no regression).
func TestHTTPProtocolBodyNoFormRegression(t *testing.T) {
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

// Verify FormData struct can be serialized (sanity check for type stability).
func TestFormDataZeroValue(t *testing.T) {
	var f FormData
	assert.Nil(t, f.Fields)
	assert.Nil(t, f.Files)
}

// Helper used by tests above to read the body bytes via bytes.Reader
// (kept here to justify the bytes import when not otherwise referenced).
var _ = bytes.NewReader

