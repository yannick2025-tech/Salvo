package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/logger"
)

var appLog logger.Logger

type multiWriter struct {
	writers []io.Writer
}

func (mw *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

var (
	reqLogOnce sync.Once
	reqLog     *log.Logger
	errorCodes = []int{500, 401, 400, 502, 504, 503}
)

func initRequestLog() {
	reqLogOnce.Do(func() {
		logDir := getLogDir()
		logPath := filepath.Join(logDir, "mock-requests.log")

		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			mw := &multiWriter{writers: []io.Writer{f, os.Stdout}}
			reqLog = log.New(mw, "[MOCK-REQ] ", log.LstdFlags)
		} else {
			reqLog = log.New(os.Stdout, "[MOCK-REQ] ", log.LstdFlags)
		}
	})
}

func getLogDir() string {
	if projectRoot := os.Getenv("SALVO_ROOT"); projectRoot != "" {
		logDir := filepath.Join(projectRoot, "logs")
		if err := os.MkdirAll(logDir, 0755); err == nil {
			return logDir
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		logDir := filepath.Join(cwd, "logs")
		if err := os.MkdirAll(logDir, 0755); err == nil {
			return logDir
		}
	}

	return "./logs"
}

func init() {
	var err error
	appLog, err = logger.New(logger.Config{Level: logger.InfoLevel, Format: logger.FormatText})
	if err != nil {
		panic(err)
	}
}

type MockServer struct {
	server *http.Server
	port   int
}

func NewMockServer(port int) *MockServer {
	return &MockServer{port: port}
}

func (m *MockServer) Start() error {
	initRequestLog()

	mux := http.NewServeMux()

	logHandler := func(pattern string, handler http.HandlerFunc) {
		mux.HandleFunc(pattern, m.logRequest(handler))
	}

	logHandler("/mock/api/users", m.handleUsers)
	logHandler("/mock/api/users/", m.handleUserDetail)
	logHandler("/mock/api/products", m.handleProducts)
	logHandler("/mock/api/products/", m.handleProductDetail)
	logHandler("/mock/api/orders", m.handleOrders)
	logHandler("/mock/api/orders/", m.handleOrderDetail)
	logHandler("/mock/api/auth/login", m.handleAuthLogin)
	logHandler("/mock/api/payment", m.handlePayment)
	logHandler("/mock/api/notify", m.handleNotify)
	logHandler("/mock/health", m.handleHealth)

	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.port),
		Handler: mux,
	}

	appLog.Info("mock HTTP server starting", logger.F("port", m.port))
	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLog.Error("mock server error", logger.F("error", err))
		}
	}()

	return nil
}

func (m *MockServer) Stop() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

func (m *MockServer) Port() int {
	return m.port
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (m *MockServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users := []map[string]any{
			{"id": 1, "name": "Alice", "email": "alice@example.com", "role": "admin", "created_at": time.Now().Format(time.RFC3339)},
			{"id": 2, "name": "Bob", "email": "bob@example.com", "role": "user", "created_at": time.Now().Add(-24 * time.Hour).Format(time.RFC3339)},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com", "role": "user", "created_at": time.Now().Add(-48 * time.Hour).Format(time.RFC3339)},
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": users, "total": len(users)})
	case http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		writeJSON(w, 201, map[string]any{"code": 0, "data": map[string]any{"id": 4, "name": req["name"], "email": req["email"], "role": "user", "created_at": time.Now().Format(time.RFC3339)}})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (m *MockServer) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/mock/api/users/")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": id, "name": "User" + id, "email": "user" + id + "@example.com", "role": "user"}})
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": id, "name": req["name"], "email": req["email"]}})
	case http.MethodDelete:
		writeJSON(w, 200, map[string]any{"code": 0, "message": "deleted"})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (m *MockServer) handleProducts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page <= 0 {
			page = 1
		}
		products := []map[string]any{
			{"id": 1, "name": "Widget A", "price": 29.99, "stock": 100},
			{"id": 2, "name": "Widget B", "price": 49.99, "stock": 50},
			{"id": 3, "name": "Gadget C", "price": 99.99, "stock": 25},
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": products, "page": page, "total": len(products)})
	case http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		writeJSON(w, 201, map[string]any{"code": 0, "data": map[string]any{"id": 4, "name": req["name"], "price": req["price"], "stock": req["stock"]}})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (m *MockServer) handleProductDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/mock/api/products/")
	writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": id, "name": "Product" + id, "price": 29.99, "stock": 100}})
}

func (m *MockServer) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		orders := []map[string]any{
			{"id": "ORD-001", "user_id": 1, "total": 79.98, "status": "completed", "items": 2},
			{"id": "ORD-002", "user_id": 2, "total": 149.99, "status": "pending", "items": 1},
		}
		writeJSON(w, 200, map[string]any{"code": 0, "data": orders, "total": len(orders)})
	case http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		writeJSON(w, 201, map[string]any{"code": 0, "data": map[string]any{"id": "ORD-003", "user_id": req["user_id"], "total": req["total"], "status": "created"}})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (m *MockServer) handleOrderDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/mock/api/orders/")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": id, "user_id": 1, "total": 79.98, "status": "completed"}})
	case http.MethodPut:
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": id, "status": "updated"}})
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
	}
}

func (m *MockServer) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req map[string]any
	json.Unmarshal(body, &req)
	email, _ := req["email"].(string)
	password, _ := req["password"].(string)
	if email == "admin@example.com" && password == "admin123" {
		writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"token": "mock-jwt-token-xyz", "user": map[string]any{"id": 1, "email": email, "role": "admin"}}})
	} else {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "invalid credentials"})
	}
}

func (m *MockServer) handlePayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req map[string]any
	json.Unmarshal(body, &req)
	writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"payment_id": "PAY-" + strconv.FormatInt(time.Now().Unix(), 10), "status": "success", "amount": req["amount"]}})
}

func (m *MockServer) handleNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"notified": true, "timestamp": time.Now().Format(time.RFC3339)}})
}

func (m *MockServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "service": "salvo-mock", "timestamp": time.Now().Format(time.RFC3339)})
}

func (m *MockServer) logRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: 200}

		if r.URL.Path != "/mock/health" {
			delayMs := rand.Intn(1971) + 30
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}

		if r.URL.Path != "/mock/health" && rand.Float64() < 0.30 {
			code := errorCodes[rand.Intn(len(errorCodes))]
			writeJSON(rw, code, map[string]any{"code": code, "message": http.StatusText(code), "_injected": true})
			reqLog.Printf("%s %s | %d | %.2fms [INJECTED]", r.Method, r.URL.Path, code, float64(time.Since(start).Microseconds())/1000)
			return
		}

		next(rw, r)
		elapsed := time.Since(start)
		reqLog.Printf("%s %s | %d | %.2fms", r.Method, r.URL.Path, rw.statusCode, float64(elapsed.Microseconds())/1000)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
