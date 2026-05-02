package mockserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := New()
	baseURL, err := s.StartTest()
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, baseURL
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	return resp
}

func readRespBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return b
}

func TestLogin(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp := postJSON(t, baseURL+"/api/login", map[string]string{
		"username": "admin",
		"password": "secret",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &result))
	assert.NotEmpty(t, result["token"])
	assert.Equal(t, "admin", result["username"])
}

func TestLoginMissingFields(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp := postJSON(t, baseURL+"/api/login", map[string]string{
		"username": "admin",
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUsersCRUD(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp := postJSON(t, baseURL+"/api/users", map[string]string{
		"name":  "Alice",
		"email": "alice@example.com",
	})
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created user
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &created))
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "Alice", created.Name)

	resp, err := http.Get(baseURL + "/api/users/" + created.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var fetched user
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &fetched))
	assert.Equal(t, created.ID, fetched.ID)

	resp, err = http.Get(baseURL + "/api/users")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var users []user
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &users))
	assert.GreaterOrEqual(t, len(users), 1)

	b, _ := json.Marshal(map[string]string{
		"name":  "Alice Updated",
		"email": "alice2@example.com",
	})
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/api/users/"+created.ID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	req, _ = http.NewRequest(http.MethodDelete, baseURL+"/api/users/"+created.ID, nil)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestUserNotFound(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp, err := http.Get(baseURL + "/api/users/nonexistent")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestOrders(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp := postJSON(t, baseURL+"/api/orders", map[string]any{
		"user_id": "u1",
		"amount":  99.99,
	})
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created order
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &created))
	assert.Equal(t, "created", created.Status)
	assert.NotEmpty(t, created.ID)

	resp, err := http.Get(baseURL + "/api/orders/" + created.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Get(baseURL + "/api/orders")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEcho(t *testing.T) {
	_, baseURL := setupTestServer(t)

	body := []byte(`{"echo":"hello"}`)
	resp, err := http.Post(baseURL+"/api/echo", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"echo":"hello"}`, string(readRespBody(t, resp)))
}

func TestHeaders(t *testing.T) {
	_, baseURL := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/headers", nil)
	req.Header.Set("X-Custom", "test-value")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var headers map[string][]string
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &headers))
	assert.Contains(t, headers["X-Custom"], "test-value")
}

func TestDelay(t *testing.T) {
	_, baseURL := setupTestServer(t)

	start := time.Now()
	resp, err := http.Get(baseURL + "/api/delay/100")
	require.NoError(t, err)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
}

func TestStatus(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp, err := http.Get(baseURL + "/api/status/201")
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, err = http.Get(baseURL + "/api/status/403")
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpload(t *testing.T) {
	_, baseURL := setupTestServer(t)

	body := []byte("file content data")
	resp, err := http.Post(baseURL+"/api/upload?filename=test.txt", "application/octet-stream", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &result))
	assert.Equal(t, float64(len(body)), result["size"])
	assert.NotEmpty(t, result["md5"])
}

func TestEncrypt(t *testing.T) {
	_, baseURL := setupTestServer(t)

	body := []byte("secret data")
	resp, err := http.Post(baseURL+"/api/encrypt", "text/plain", bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &result))
	assert.Equal(t, "secret data", result["input"])
	assert.NotEmpty(t, result["md5"])
}

func TestRedirect(t *testing.T) {
	_, baseURL := setupTestServer(t)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(baseURL + "/api/redirect/2")
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, resp.StatusCode)
}

func TestRedirectFinal(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp, err := http.Get(baseURL + "/api/redirect/0")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestError(t *testing.T) {
	_, baseURL := setupTestServer(t)

	resp, err := http.Get(baseURL + "/api/error")
	require.NoError(t, err)
	assert.True(t, resp.StatusCode >= 500)
}

func TestStats(t *testing.T) {
	_, baseURL := setupTestServer(t)

	_, _ = http.Get(baseURL + "/api/echo")
	_, _ = http.Get(baseURL + "/api/echo")

	resp, err := http.Get(baseURL + "/api/stats")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats map[string]any
	require.NoError(t, json.Unmarshal(readRespBody(t, resp), &stats))
	assert.GreaterOrEqual(t, stats["total_requests"], float64(2))
}

func TestRequestCount(t *testing.T) {
	s, baseURL := setupTestServer(t)

	_, _ = http.Get(baseURL + "/api/echo")
	_, _ = http.Get(baseURL + "/api/echo")
	_, _ = http.Get(baseURL + "/api/echo")

	assert.GreaterOrEqual(t, s.RequestCount(), int64(3))
}

func TestSetLatency(t *testing.T) {
	s, baseURL := setupTestServer(t)

	s.SetLatency(100 * time.Millisecond)
	start := time.Now()
	_, _ = http.Get(baseURL + "/api/echo")
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)

	s.SetLatency(0)
	_ = s
}

func TestSetErrorRate(t *testing.T) {
	s, baseURL := setupTestServer(t)

	s.SetErrorRate(1.0)
	resp, err := http.Get(baseURL + "/api/echo")
	require.NoError(t, err)
	assert.True(t, resp.StatusCode >= 500)

	s.SetErrorRate(0)
	_ = s
}
