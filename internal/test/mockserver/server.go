// Package mockserver provides a configurable HTTP server for end-to-end
// testing of Salvo. It exposes REST endpoints that mimic a typical web
// service including authentication, CRUD, file upload, and error
// simulation.
package mockserver

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	mathrand "math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type user struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type order struct {
	ID     string  `json:"id"`
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
	Status string  `json:"status"`
}

// Server is a mock HTTP server for end-to-end testing of Salvo.
type Server struct {
	mu       sync.RWMutex
	users    map[string]user
	orders   map[string]order
	tokens   map[string]string
	server   *http.Server
	port     int
	reqCount atomic.Int64
	latency  time.Duration
	errRate  float64
}

// New creates a new mock Server.
func New() *Server {
	return &Server{
		users:  make(map[string]user),
		orders: make(map[string]order),
		tokens: make(map[string]string),
	}
}

// SetLatency configures a fixed delay for all responses.
func (s *Server) SetLatency(d time.Duration) {
	s.mu.Lock()
	s.latency = d
	s.mu.Unlock()
}

// SetErrorRate configures the probability (0.0-1.0) of returning a
// random 5xx error instead of the normal response.
func (s *Server) SetErrorRate(rate float64) {
	s.mu.Lock()
	s.errRate = rate
	s.mu.Unlock()
}

// RequestCount returns the total number of requests served.
func (s *Server) RequestCount() int64 {
	return s.reqCount.Load()
}

// Start starts the server on the given port. If port is 0, a random
// available port is used.
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	addr := fmt.Sprintf(":%d", port)
	s.server = &http.Server{Addr: addr, Handler: mux}

	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// StartTest starts the server on a random port and returns the base URL.
// This is intended for use in tests.
func (s *Server) StartTest() (string, error) {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	s.port = ln.Addr().(*net.TCPAddr).Port
	s.server = &http.Server{Handler: mux}

	go func() { _ = s.server.Serve(ln) }()

	return fmt.Sprintf("http://127.0.0.1:%d", s.port), nil
}

// Close shuts down the server.
func (s *Server) Close() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/login", s.wrap(s.handleLogin))
	mux.HandleFunc("/api/users", s.wrap(s.handleUsers))
	mux.HandleFunc("/api/users/", s.wrap(s.handleUserByID))
	mux.HandleFunc("/api/orders", s.wrap(s.handleOrders))
	mux.HandleFunc("/api/orders/", s.wrap(s.handleOrderByID))
	mux.HandleFunc("/api/upload", s.wrap(s.handleUpload))
	mux.HandleFunc("/api/delay/", s.wrap(s.handleDelay))
	mux.HandleFunc("/api/status/", s.wrap(s.handleStatus))
	mux.HandleFunc("/api/echo", s.wrap(s.handleEcho))
	mux.HandleFunc("/api/headers", s.wrap(s.handleHeaders))
	mux.HandleFunc("/api/encrypt", s.wrap(s.handleEncrypt))
	mux.HandleFunc("/api/chunked", s.wrap(s.handleChunked))
	mux.HandleFunc("/api/redirect/", s.wrap(s.handleRedirect))
	mux.HandleFunc("/api/error", s.wrap(s.handleError))
	mux.HandleFunc("/api/stats", s.wrap(s.handleStats))
}

func (s *Server) maybeError() bool {
	s.mu.RLock()
	rate := s.errRate
	s.mu.RUnlock()
	if rate <= 0 {
		return false
	}
	return mathrand.Float64() < rate
}

func (s *Server) addLatency() {
	s.mu.RLock()
	d := s.latency
	s.mu.RUnlock()
	if d > 0 {
		time.Sleep(d)
	}
}

func (s *Server) wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.reqCount.Add(1)
		s.addLatency()

		if s.maybeError() {
			codes := []int{500, 502, 503}
			w.WriteHeader(codes[mathrand.Intn(len(codes))])
			_, _ = w.Write([]byte(`{"error":"random_server_error"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	_, _ = fmt.Fprintf(w, `{"error":"%s"}`, msg)
}

func readBody(r *http.Request) []byte {
	b, _ := io.ReadAll(r.Body)
	return b
}

func decodeJSON(r *http.Request, v any) {
	_ = json.Unmarshal(readBody(r), v)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decodeJSON(r, &creds)

	if creds.Username == "" || creds.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	token := generateToken(32)
	s.mu.Lock()
	s.tokens[token] = creds.Username
	s.mu.Unlock()

	writeJSON(w, map[string]string{
		"token":    token,
		"username": creds.Username,
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		users := make([]user, 0, len(s.users))
		for _, u := range s.users {
			users = append(users, u)
		}
		s.mu.RUnlock()
		writeJSON(w, users)

	case http.MethodPost:
		var u user
		decodeJSON(r, &u)
		if u.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		u.ID = generateID()
		s.mu.Lock()
		s.users[u.ID] = u
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, u)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/users/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		u, ok := s.users[id]
		s.mu.RUnlock()
		if !ok {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, u)

	case http.MethodPut:
		var u user
		decodeJSON(r, &u)
		u.ID = id
		s.mu.Lock()
		s.users[id] = u
		s.mu.Unlock()
		writeJSON(w, u)

	case http.MethodDelete:
		s.mu.Lock()
		delete(s.users, id)
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		orders := make([]order, 0, len(s.orders))
		for _, o := range s.orders {
			orders = append(orders, o)
		}
		s.mu.RUnlock()
		writeJSON(w, orders)

	case http.MethodPost:
		var o order
		decodeJSON(r, &o)
		o.ID = generateID()
		o.Status = "created"
		s.mu.Lock()
		s.orders[o.ID] = o
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, o)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOrderByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/orders/")
	s.mu.RLock()
	o, ok := s.orders[id]
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, o)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body := readBody(r)
	hash := md5.Sum(body)

	writeJSON(w, map[string]any{
		"size":     len(body),
		"md5":      hex.EncodeToString(hash[:]),
		"filename": r.URL.Query().Get("filename"),
	})
}

func (s *Server) handleDelay(w http.ResponseWriter, r *http.Request) {
	msStr := strings.TrimPrefix(r.URL.Path, "/api/delay/")
	ms, err := strconv.Atoi(msStr)
	if err != nil || ms < 0 {
		writeError(w, http.StatusBadRequest, "invalid delay")
		return
	}

	time.Sleep(time.Duration(ms) * time.Millisecond)
	writeJSON(w, map[string]any{
		"delay_ms": ms,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	codeStr := strings.TrimPrefix(r.URL.Path, "/api/status/")
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid status code")
		return
	}

	w.WriteHeader(code)
	writeJSON(w, map[string]any{
		"status_code": code,
	})
}

func (s *Server) handleEcho(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	_, _ = w.Write(body)
}

func (s *Server) handleHeaders(w http.ResponseWriter, r *http.Request) {
	headers := make(map[string][]string)
	for k, v := range r.Header {
		headers[k] = v
	}
	writeJSON(w, headers)
}

func (s *Server) handleEncrypt(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	writeJSON(w, map[string]string{
		"input":  string(body),
		"md5":    fmt.Sprintf("%x", md5.Sum(body)),
		"length": strconv.Itoa(len(body)),
	})
}

func (s *Server) handleChunked(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for i := 0; i < 3; i++ {
		_, _ = fmt.Fprintf(w, `{"chunk":%d}`, i)
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	countStr := strings.TrimPrefix(r.URL.Path, "/api/redirect/")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if count <= 0 {
		writeJSON(w, map[string]string{"status": "done"})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/api/redirect/%d", count-1), http.StatusFound)
}

func (s *Server) handleError(w http.ResponseWriter, r *http.Request) {
	codes := []int{500, 502, 503}
	code := codes[mathrand.Intn(len(codes))]
	w.WriteHeader(code)
	writeJSON(w, map[string]any{
		"error":       "random_error",
		"status_code": code,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	userCount := len(s.users)
	orderCount := len(s.orders)
	s.mu.RUnlock()

	writeJSON(w, map[string]any{
		"total_requests": s.reqCount.Load(),
		"users":          userCount,
		"orders":         orderCount,
	})
}

func generateToken(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateID() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000000000))
	return fmt.Sprintf("id_%d", n.Int64()+1000000000)
}
