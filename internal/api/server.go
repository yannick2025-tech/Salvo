// Package api provides the REST API server for Salvo. All endpoints use
// POST method; query parameters are passed via the JSON request body.
// The server uses Go 1.22+ enhanced ServeMux for route matching.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
	"github.com/yannick2025-tech/Salvo/internal/store/sqlite"
)

// Server is the REST API server for Salvo.
type Server struct {
	httpServer *http.Server
	db         *sqlite.DB
	logger     logger.Logger
	handler    *Handler
}

// Config holds the API server configuration.
type Config struct {
	// Addr is the listen address (e.g. ":8080").
	Addr string
	// DB is the SQLite database wrapper.
	DB *sqlite.DB
	// Logger is the structured logger.
	Logger logger.Logger
}

// New creates a new API server with the given configuration.
func New(cfg Config) *Server {
	h := &Handler{
		scenes:   sqlite.NewSceneRepo(cfg.DB),
		nodes:    sqlite.NewNodeRepo(cfg.DB),
		edges:    sqlite.NewEdgeRepo(cfg.DB),
		variables: sqlite.NewVariableRepo(cfg.DB),
		plugins:  sqlite.NewPluginConfigRepo(cfg.DB),
		reports:  sqlite.NewReportRepo(cfg.DB),
		runs:     sqlite.NewRunRecordRepo(cfg.DB),
	}

	s := &Server{
		db:      cfg.DB,
		logger:  cfg.Logger,
		handler: h,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	var handler http.Handler = mux
	handler = s.recoveryMiddleware(handler)
	handler = s.loggingMiddleware(handler)
	handler = s.corsMiddleware(handler)

	s.httpServer = &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start starts the API server in a blocking manner.
func (s *Server) Start() error {
	s.logger.Info("api server starting", logger.F("addr", s.httpServer.Addr))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the API server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("api server shutting down")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/scenes/list", s.handle(s.handler.ListScenes))
	mux.HandleFunc("POST /api/v1/scenes/create", s.handle(s.handler.CreateScene))
	mux.HandleFunc("POST /api/v1/scenes/get", s.handle(s.handler.GetScene))
	mux.HandleFunc("POST /api/v1/scenes/update", s.handle(s.handler.UpdateScene))
	mux.HandleFunc("POST /api/v1/scenes/delete", s.handle(s.handler.DeleteScene))

	mux.HandleFunc("POST /api/v1/scenes/nodes/list", s.handle(s.handler.ListNodes))
	mux.HandleFunc("POST /api/v1/scenes/nodes/add", s.handle(s.handler.AddNode))
	mux.HandleFunc("POST /api/v1/scenes/nodes/update", s.handle(s.handler.UpdateNode))
	mux.HandleFunc("POST /api/v1/scenes/nodes/delete", s.handle(s.handler.DeleteNode))

	mux.HandleFunc("POST /api/v1/scenes/edges/add", s.handle(s.handler.AddEdge))
	mux.HandleFunc("POST /api/v1/scenes/edges/delete", s.handle(s.handler.DeleteEdge))

	mux.HandleFunc("POST /api/v1/scenes/variables/list", s.handle(s.handler.ListVariables))
	mux.HandleFunc("POST /api/v1/scenes/variables/set", s.handle(s.handler.SetVariable))

	mux.HandleFunc("POST /api/v1/plugins/list", s.handle(s.handler.ListPlugins))
	mux.HandleFunc("POST /api/v1/plugins/config", s.handle(s.handler.UpdatePluginConfig))

	mux.HandleFunc("POST /api/v1/reports/list", s.handle(s.handler.ListReports))
	mux.HandleFunc("POST /api/v1/reports/get", s.handle(s.handler.GetReport))

	mux.HandleFunc("POST /api/v1/runs/list", s.handle(s.handler.ListRunRecords))
	mux.HandleFunc("POST /api/v1/runs/get", s.handle(s.handler.GetRunRecord))
}

// handlerFunc is an adapter that returns a standard dto.Response.
type handlerFunc func(r *http.Request) dto.Response

func (s *Server) handle(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := h(r)
		writeJSON(w, resp)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s, _ := json.Marshal(dto.ErrorResp(500, "internal encoding error"))
		_, _ = w.Write(s)
	}
}

func decode[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("decode request body: %w", err)
	}
	return v, nil
}

// --- Middleware ---

func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Error("panic recovered",
					logger.F("path", r.URL.Path),
					logger.F("panic", rec),
				)
				writeJSON(w, dto.ErrorResp(500, "internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http request",
			logger.F("method", r.Method),
			logger.F("path", r.URL.Path),
			logger.F("duration", time.Since(start).String()),
		)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Handler ---

// Handler holds all repository references and implements the API handlers.
type Handler struct {
	scenes    repo.SceneRepo
	nodes     repo.NodeRepo
	edges     repo.EdgeRepo
	variables repo.VariableRepo
	plugins   repo.PluginConfigRepo
	reports   repo.ReportRepo
	runs      repo.RunRecordRepo
}
