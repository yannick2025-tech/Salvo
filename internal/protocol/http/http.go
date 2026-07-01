// Package http implements the Protocol interface for HTTP-based
// performance testing. It defines HTTP-specific request and response
// types and supports all standard HTTP methods, custom headers,
// request bodies, and per-request timeouts.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/protocol"
)

// Method represents an HTTP method.
type Method string

const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodDelete Method = "DELETE"
	MethodPatch  Method = "PATCH"
	MethodHead   Method = "HEAD"
)

// FormData represents a multipart/form-data request body. It carries
// simple string fields and one or more file paths that the protocol
// layer will open and stream into the multipart writer.
type FormData struct {
	// Fields are simple text form values.
	Fields map[string]string
	// Files maps field name → file path on disk.
	Files map[string]string
}

// HTTPRequest is the HTTP-specific request type. It implements the
// protocol.Request interface.
type HTTPRequest struct {
	Method  Method
	URL     string
	Headers map[string]string
	Body    []byte
	// Form, when non-nil, builds a multipart/form-data request body.
	// It takes precedence over Body when both are set.
	Form    *FormData
	Timeout time.Duration
}

// GetTimeout implements protocol.Request.
func (r *HTTPRequest) GetTimeout() time.Duration {
	return r.Timeout
}

// HTTPResponse is the HTTP-specific response type. It implements the
// protocol.Response interface.
type HTTPResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	Latency    time.Duration
	Error      error
}

// GetStatusCode implements protocol.Response.
func (r *HTTPResponse) GetStatusCode() int {
	return r.StatusCode
}

// GetLatency implements protocol.Response.
func (r *HTTPResponse) GetLatency() time.Duration {
	return r.Latency
}

// GetError implements protocol.Response.
func (r *HTTPResponse) GetError() error {
	return r.Error
}

// IsError returns true if the status code indicates a client or server
// error (4xx or 5xx).
func (r *HTTPResponse) IsError() bool {
	return r.StatusCode >= 400
}

// IsSuccess returns true if the status code is in the 2xx range.
func (r *HTTPResponse) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// BodyJSON unmarshals the response body into the provided pointer.
func (r *HTTPResponse) BodyJSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Protocol executes HTTP requests and returns HTTPResponse results.
type Protocol struct {
	client *http.Client
}

// Option configures a Protocol during construction.
type Option func(*Protocol)

// WithClient sets a custom http.Client for the Protocol.
func WithClient(client *http.Client) Option {
	return func(p *Protocol) {
		p.client = client
	}
}

// NewProtocol creates an HTTP Protocol with the given options.
// By default, it uses an http.Client with no timeout (timeouts are
// handled per-request via context).
func NewProtocol(opts ...Option) *Protocol {
	p := &Protocol{
		client: &http.Client{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the protocol identifier "http".
func (p *Protocol) Name() string {
	return "http"
}

// Execute sends the HTTP request described by req and returns the
// response. The context controls cancellation and timeout.
func (p *Protocol) Execute(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	httpReq, ok := req.(*HTTPRequest)
	if !ok {
		return nil, fmt.Errorf("http: expected *HTTPRequest, got %T", req)
	}

	if httpReq.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, httpReq.Timeout)
		defer cancel()
	}

	// Ensure query parameters are properly percent-encoded. Variable
	// interpolation can inject raw values (e.g. "Widget B") into the query
	// string; unencoded spaces break the HTTP request line and cause
	// servers to return 400 Bad Request before any handler runs.
	requestURL := httpReq.URL
	if u, perr := url.Parse(httpReq.URL); perr == nil && u.RawQuery != "" {
		if values, verr := url.ParseQuery(u.RawQuery); verr == nil {
			u.RawQuery = values.Encode()
			requestURL = u.String()
		}
	}

	// Build request body. multipart/form-data (Form) takes precedence over
	// raw Body when both are set, because mixing the two on the same request
	// is undefined behavior.
	var bodyReader io.Reader
	if httpReq.Form != nil {
		buf, contentType, err := buildMultipartBody(httpReq.Form)
		if err != nil {
			return nil, fmt.Errorf("http: build multipart body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
		// Auto-set Content-Type unless caller already provided one, so
		// the boundary reaches the server intact.
		if httpReq.Headers == nil {
			httpReq.Headers = make(map[string]string)
		}
		if _, exists := httpReq.Headers["Content-Type"]; !exists {
			httpReq.Headers["Content-Type"] = contentType
		}
	} else if httpReq.Body != nil {
		bodyReader = strings.NewReader(string(httpReq.Body))
	}

	goReq, err := http.NewRequestWithContext(ctx, string(httpReq.Method), requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http: create request: %w", err)
	}

	for k, v := range httpReq.Headers {
		goReq.Header.Set(k, v)
	}

	start := time.Now()
	httpResp, err := p.client.Do(goReq)
	if err != nil {
		return nil, fmt.Errorf("http: execute request: %w", err)
	}
	latency := time.Since(start)

	respBody, err := io.ReadAll(httpResp.Body)
	closeErr := httpResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("http: read response body: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("http: close response body: %w", closeErr)
	}

	respHeaders := make(map[string][]string)
	for k, v := range httpResp.Header {
		respHeaders[k] = v
	}

	return &HTTPResponse{
		StatusCode: httpResp.StatusCode,
		Headers:    respHeaders,
		Body:       respBody,
		Latency:    latency,
	}, nil
}

// buildMultipartBody constructs a multipart/form-data body from form fields
// and file paths. It returns the full body bytes and the Content-Type
// header value (including the generated boundary).
//
// Files are read from disk and streamed into a writer; missing files cause
// an error that propagates up so callers can distinguish "file not found"
// from a transport error.
func buildMultipartBody(form *FormData) (body []byte, contentType string, err error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	// Text fields (sorted for deterministic output, helps with replay/debugging).
	for _, k := range sortedKeys(form.Fields) {
		if wErr := w.WriteField(k, form.Fields[k]); wErr != nil {
			return nil, "", fmt.Errorf("write field %q: %w", k, wErr)
		}
	}

	// Files.
	for _, field := range sortedFileKeys(form.Files) {
		path := form.Files[field]
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil, "", fmt.Errorf("open file for field %q: %w", field, openErr)
		}
		part, createErr := w.CreateFormFile(field, filepath.Base(path))
		if createErr != nil {
			_ = f.Close()
			return nil, "", fmt.Errorf("create form file %q: %w", field, createErr)
		}
		if _, cpErr := io.Copy(part, f); cpErr != nil {
			_ = f.Close()
			return nil, "", fmt.Errorf("copy file %q: %w", field, cpErr)
		}
		if closeErr := f.Close(); closeErr != nil {
			return nil, "", fmt.Errorf("close file %q: %w", field, closeErr)
		}
	}

	if closeErr := w.Close(); closeErr != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", closeErr)
	}

	return buf.Bytes(), w.FormDataContentType(), nil
}

// sortedKeys returns map keys sorted alphabetically.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

// sortedFileKeys is the same as sortedKeys but kept separate for clarity
// (files vs fields are conceptually different categories).
func sortedFileKeys(m map[string]string) []string {
	return sortedKeys(m)
}

// sortStrings is an inlined insertion sort to avoid pulling in sort for
// what's typically a tiny number of form fields (<20).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j-1] > s[j] {
			s[j-1], s[j] = s[j], s[j-1]
			j--
		}
	}
}
