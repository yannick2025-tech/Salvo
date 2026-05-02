// Package protocol defines the generic interfaces for all supported
// protocols in Salvo (HTTP, DB, FTP, gRPC, etc.).
//
// The core abstraction is the Protocol interface which executes a
// Request and returns a Response. Each protocol implementation
// (e.g. internal/protocol/http) provides its own concrete logic while
// conforming to this contract.
package protocol

import (
	"context"
	"encoding/json"
	"time"
)

// Method represents the HTTP method or protocol-specific operation type.
type Method string

const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodDelete Method = "DELETE"
	MethodPatch  Method = "PATCH"
	MethodHead   Method = "HEAD"
)

// Request is a protocol-agnostic description of an operation to execute.
// Protocol-specific implementations interpret the fields as needed.
type Request struct {
	// Method is the operation type (e.g. GET, POST for HTTP).
	Method Method
	// URL is the target endpoint or resource identifier.
	URL string
	// Headers contains protocol-level metadata (e.g. HTTP headers).
	Headers map[string]string
	// Body is the request payload (may be nil).
	Body []byte
	// Timeout is the per-request timeout. Zero means no timeout.
	Timeout time.Duration
	// Metadata holds protocol-specific options that don't fit the
	// common fields.
	Metadata map[string]any
}

// Response is the result of executing a Request.
type Response struct {
	// StatusCode is the protocol-specific status code (e.g. 200 for HTTP).
	StatusCode int
	// Headers contains the response metadata.
	Headers map[string][]string
	// Body is the response payload (may be nil).
	Body []byte
	// Latency is the round-trip duration.
	Latency time.Duration
	// Error is set when the protocol execution itself fails (network
	// error, DNS failure, etc.). A non-nil Error means the request did
	// not reach the server.
	Error error
}

// IsError returns true if the status code indicates a server or client
// error (4xx or 5xx).
func (r *Response) IsError() bool {
	return r.StatusCode >= 400
}

// IsSuccess returns true if the status code is in the 2xx range.
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// BodyJSON unmarshals the response body into the provided pointer.
func (r *Response) BodyJSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Protocol is the core interface that every protocol implementation
// must satisfy. It executes a single request and returns the response.
type Protocol interface {
	// Execute sends the request and returns the response. The context
	// carries timeout and cancellation signals.
	Execute(ctx context.Context, req *Request) (*Response, error)
	// Name returns the protocol identifier (e.g. "http", "grpc").
	Name() string
}
