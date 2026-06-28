// Package api implements the HTTP API layer for Salvo, handling scene
// management, user authentication, DAG node/edge operations, and test
// execution control.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/auth"
	"github.com/yannick2025-tech/Salvo/internal/core/expr"
	"github.com/yannick2025-tech/Salvo/internal/generator/builtin"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/plugin/so"
	"github.com/yannick2025-tech/Salvo/internal/runner"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
	"github.com/yannick2025-tech/Salvo/internal/store/sqlite"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
	tracestore "github.com/yannick2025-tech/Salvo/internal/trace/store"
)

type Server struct {
	httpServer *http.Server
	soLoader   *so.Loader
	soReg      *expr.FunctionRegistry
	db         *sqlite.DB
	logger     logger.Logger
	handler    *Handler
	jwt        *auth.JWTManager
	rbac       *auth.RBACChecker
	webDir     string
}

type Config struct {
	Addr      string
	DB        *sqlite.DB
	Logger    logger.Logger
	JWT       *auth.JWTManager
	RBAC      *auth.RBACChecker
	WebDir    string
	Variables map[string]string
}

func New(cfg Config) *Server {
	h := &Handler{
		log:        cfg.Logger,
		scenes:     sqlite.NewSceneRepo(cfg.DB),
		nodes:      sqlite.NewNodeRepo(cfg.DB),
		edges:      sqlite.NewEdgeRepo(cfg.DB),
		variables:  sqlite.NewVariableRepo(cfg.DB),
		plugins:    sqlite.NewPluginConfigRepo(cfg.DB),
		reports:    sqlite.NewReportRepo(cfg.DB),
		runs:       sqlite.NewRunRecordRepo(cfg.DB),
		users:      sqlite.NewUserRepo(cfg.DB),
		roles:      sqlite.NewRoleRepo(cfg.DB),
		perms:      sqlite.NewPermissionRepo(cfg.DB),
		rp:         sqlite.NewRolePermissionRepo(cfg.DB),
		dataSources: sqlite.NewDataSourceRepo(cfg.DB),
		soPlugins:   sqlite.NewSOPluginRepo(cfg.DB),
		jwt:         cfg.JWT,
		soLoader:    nil, // will be set after bootstrap
		rbac:       cfg.RBAC,
		globalVars: cfg.Variables,
	}

	h.traceStore = tracestore.New(cfg.DB.DB)

	tracer, err := tracelib.NewTracer(tracelib.Config{
		BufferSize: 1000,
		Persister:  h.traceStore,
	})
	if err != nil {
		cfg.Logger.Error("failed to create tracer", logger.F("error", err))
	}
	h.tracer = tracer

	if err := h.tracer.LoadFromDB(context.Background()); err != nil {
		cfg.Logger.Warn("failed to load traces from db on startup", logger.F("error", err))
	} else {
		cfg.Logger.Info("loaded traces from database on startup", logger.F("count", h.tracer.Total()))
	}

	h.tsStore = runner.NewSQLiteTimeSeriesStore(cfg.DB.DB)
	h.runnerMgr = runner.NewManager(h.scenes, h.nodes, h.edges, h.runs, h.reports, h.dataSources, h.tracer, h.tsStore, cfg.Logger, nil)

	// Bootstrap SO plugin system: create a registry and load enabled plugins.
	soReg := expr.NewFunctionRegistry()
	builtin.RegisterAll(soReg)
	soLoader, err := so.InitFromDB(context.Background(), h.soPlugins, soReg)
	if err != nil {
		cfg.Logger.Warn("so plugin bootstrap completed with errors", logger.F("error", err))
	} else {
		cfg.Logger.Info("so plugin bootstrap completed")
	}
	// Pass the SO expression registry to the runner manager so that generator
	// nodes can resolve ${__so(...)} expressions.
	h.runnerMgr.SetExprRegistry(soReg)
	// Store soLoader in handler for hot-load after upload.
	h.soLoader = soLoader

	s := &Server{
		soLoader: soLoader,
		soReg:    soReg,
		db:       cfg.DB,
		logger:  cfg.Logger,
		handler: h,
		jwt:     cfg.JWT,
		rbac:    cfg.RBAC,
		webDir:  cfg.WebDir,
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

func (s *Server) Start() error {
	s.logger.Info("api server starting", logger.F("addr", s.httpServer.Addr))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("api server shutting down")
	return s.httpServer.Shutdown(ctx)
}

var publicRoutes = map[string]bool{
	"/api/v1/auth/login":          true,
	"/api/v1/auth/change-password": true,
}

var routePermissions = map[string]string{
	"/api/v1/dashboard/overview":   "dashboard:read",
	"/api/v1/scenes/list":          "scene:read",
	"/api/v1/scenes/create":      "scene:write",
	"/api/v1/scenes/import":      "scene:write",
	"/api/v1/scenes/get":           "scene:read",
	"/api/v1/scenes/update":        "scene:write",
	"/api/v1/scenes/delete":        "scene:write",
	"/api/v1/scenes/nodes/list":    "scene:read",
	"/api/v1/scenes/nodes/add":     "scene:write",
	"/api/v1/scenes/nodes/update":  "scene:write",
	"/api/v1/scenes/nodes/delete":  "scene:write",
	"/api/v1/scenes/edges/list":    "scene:read",
	"/api/v1/scenes/edges/add":     "scene:write",
	"/api/v1/scenes/edges/delete":  "scene:write",
	"/api/v1/scenes/variables/list": "scene:read",
	"/api/v1/scenes/variables/set":  "scene:write",
	"/api/v1/scenes/start":         "scene:run",
	"/api/v1/scenes/stop":          "scene:run",
	"/api/v1/scenes/status":        "runner:read",
	"/api/v1/plugins/list":         "settings:read",
	"/api/v1/plugins/config":       "settings:write",
	"/api/v1/so-plugins/upload-file": "admin:write",
	"/api/v1/so-plugins/create":    "admin:write",
	"/api/v1/so-plugins/list":      "admin:read",
	"/api/v1/so-plugins/get":       "admin:read",
	"/api/v1/so-plugins/status":    "admin:write",
	"/api/v1/so-plugins/config":    "admin:write",
	"/api/v1/so-plugins/delete":    "admin:write",
	"/api/v1/reports/list":         "report:read",
	"/api/v1/reports/get":          "report:read",
	"/api/v1/runs/list":            "runner:read",
	"/api/v1/runs/get":             "runner:read",
	"/api/v1/traces/list":          "trace:read",
	"/api/v1/traces/get":           "trace:read",
	"/api/v1/traces/get-by-run":    "trace:read",
	"/api/v1/auth/me":              "",
	"/api/v1/auth/logout":          "",
	"/api/v1/auth/reset-password":  "user:write",
	"/api/v1/users/list":           "user:read",
	"/api/v1/users/create":         "user:write",
	"/api/v1/users/update":         "user:write",
	"/api/v1/users/delete":         "user:write",
	"/api/v1/roles/list":           "role:read",
	"/api/v1/roles/create":         "role:write",
	"/api/v1/roles/update":         "role:write",
	"/api/v1/roles/delete":         "role:write",
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", s.handlePublic(s.handler.Login))
	mux.HandleFunc("POST /api/v1/auth/me", s.handleAuth(s.handler.Me))
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleAuth(s.handler.Logout))
	mux.HandleFunc("POST /api/v1/auth/change-password", s.handleAuth(s.handler.ChangePassword))
	mux.HandleFunc("POST /api/v1/auth/reset-password", s.handleAuth(s.handler.ResetPassword))

	mux.HandleFunc("POST /api/v1/dashboard/overview", s.handleAuth(s.handler.DashboardOverview))
	mux.HandleFunc("POST /api/v1/dashboard/history", s.handleAuth(s.handler.DashboardHistory))

	mux.HandleFunc("POST /api/v1/scenes/list", s.handleAuth(s.handler.ListScenes))
	mux.HandleFunc("POST /api/v1/scenes/create", s.handleAuth(s.handler.CreateScene))
	mux.HandleFunc("POST /api/v1/scenes/import", s.handleAuth(s.handler.ImportYAML))
	mux.HandleFunc("POST /api/v1/scenes/export", s.handleAuth(s.handler.ExportYAML))
	mux.HandleFunc("POST /api/v1/scenes/get", s.handleAuth(s.handler.GetScene))
	mux.HandleFunc("POST /api/v1/scenes/update", s.handleAuth(s.handler.UpdateScene))
	mux.HandleFunc("POST /api/v1/scenes/delete", s.handleAuth(s.handler.DeleteScene))

	mux.HandleFunc("POST /api/v1/scenes/nodes/list", s.handleAuth(s.handler.ListNodes))
	mux.HandleFunc("POST /api/v1/scenes/nodes/add", s.handleAuth(s.handler.AddNode))
	mux.HandleFunc("POST /api/v1/scenes/nodes/update", s.handleAuth(s.handler.UpdateNode))
	mux.HandleFunc("POST /api/v1/scenes/nodes/delete", s.handleAuth(s.handler.DeleteNode))

	mux.HandleFunc("POST /api/v1/scenes/edges/add", s.handleAuth(s.handler.AddEdge))
	mux.HandleFunc("POST /api/v1/scenes/edges/list", s.handleAuth(s.handler.ListEdges))
	mux.HandleFunc("POST /api/v1/scenes/edges/delete", s.handleAuth(s.handler.DeleteEdge))

	mux.HandleFunc("POST /api/v1/scenes/variables/list", s.handleAuth(s.handler.ListVariables))
	mux.HandleFunc("POST /api/v1/scenes/variables/set", s.handleAuth(s.handler.SetVariable))
	mux.HandleFunc("POST /api/v1/scenes/variables/batch-set", s.handleAuth(s.handler.BatchSetVariables))

	mux.HandleFunc("POST /api/v1/scenes/datasources/upload", s.handleAuth(s.handler.UploadDataSource))
	mux.HandleFunc("POST /api/v1/scenes/datasources/list", s.handleAuth(s.handler.ListDataSources))
	mux.HandleFunc("POST /api/v1/scenes/datasources/preview", s.handleAuth(s.handler.PreviewDataSource))
	mux.HandleFunc("POST /api/v1/scenes/datasources/delete", s.handleAuth(s.handler.DeleteDataSource))

	mux.HandleFunc("POST /api/v1/plugins/list", s.handleAuth(s.handler.ListPlugins))
	mux.HandleFunc("POST /api/v1/plugins/config", s.handleAuth(s.handler.UpdatePluginConfig))

	mux.HandleFunc("POST /api/v1/so-plugins/upload-file", s.handleAuth(s.handler.UploadSOPluginFile))
	mux.HandleFunc("POST /api/v1/so-plugins/create", s.handleAuth(s.handler.UploadSOPlugin))
	mux.HandleFunc("POST /api/v1/so-plugins/list", s.handleAuth(s.handler.ListSOPlugins))
	mux.HandleFunc("POST /api/v1/so-plugins/get", s.handleAuth(s.handler.GetSOPlugin))
	mux.HandleFunc("POST /api/v1/so-plugins/status", s.handleAuth(s.handler.UpdateSOPluginStatus))
	mux.HandleFunc("POST /api/v1/so-plugins/config", s.handleAuth(s.handler.UpdateSOPluginConfig))
	mux.HandleFunc("POST /api/v1/so-plugins/delete", s.handleAuth(s.handler.DeleteSOPlugin))

	mux.HandleFunc("POST /api/v1/generators/list", s.handleAuth(s.handler.ListGenerators))

	mux.HandleFunc("POST /api/v1/reports/list", s.handleAuth(s.handler.ListReports))
	mux.HandleFunc("POST /api/v1/reports/get", s.handleAuth(s.handler.GetReport))
	mux.HandleFunc("GET /api/v1/reports/{id}/export", s.authMiddleware(s.handler.ExportReport))
	mux.HandleFunc("POST /api/v1/reports/batch-export", s.authMiddleware(s.handler.BatchExportReports))

	mux.HandleFunc("POST /api/v1/runs/list", s.handleAuth(s.handler.ListRunRecords))
	mux.HandleFunc("POST /api/v1/runs/get", s.handleAuth(s.handler.GetRunRecord))

	mux.HandleFunc("POST /api/v1/traces/list", s.handleAuth(s.handler.ListTraces))
	mux.HandleFunc("POST /api/v1/traces/get", s.handleAuth(s.handler.GetTrace))
	mux.HandleFunc("POST /api/v1/traces/get-by-run", s.handleAuth(s.handler.GetTraceByRun))

	mux.HandleFunc("POST /api/v1/scenes/start", s.handleAuth(s.handler.StartScene))
	mux.HandleFunc("POST /api/v1/scenes/stop", s.handleAuth(s.handler.StopScene))
	mux.HandleFunc("POST /api/v1/scenes/status", s.handleAuth(s.handler.SceneStatus))

	mux.HandleFunc("POST /api/v1/users/list", s.handleAuth(s.handler.ListUsers))
	mux.HandleFunc("POST /api/v1/users/create", s.handleAuth(s.handler.CreateUser))
	mux.HandleFunc("POST /api/v1/users/update", s.handleAuth(s.handler.UpdateUser))
	mux.HandleFunc("POST /api/v1/users/delete", s.handleAuth(s.handler.DeleteUser))

	mux.HandleFunc("POST /api/v1/roles/list", s.handleAuth(s.handler.ListRoles))
	mux.HandleFunc("POST /api/v1/roles/create", s.handleAuth(s.handler.CreateRole))
	mux.HandleFunc("POST /api/v1/roles/update", s.handleAuth(s.handler.UpdateRole))
	mux.HandleFunc("POST /api/v1/roles/delete", s.handleAuth(s.handler.DeleteRole))

	if s.webDir != "" {
		s.registerSPA(mux)
	}
}

func (s *Server) registerSPA(mux *http.ServeMux) {
	fileServer := http.FileServer(http.Dir(s.webDir))

	mux.HandleFunc("GET /assets/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fileServer.ServeHTTP(w, r)
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(s.webDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(s.webDir, "index.html"))
	})
}

type handlerFunc func(r *http.Request) dto.Response

func (s *Server) handle(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := h(r)
		writeJSON(w, resp)
	}
}

func (s *Server) handlePublic(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := h(r)
		writeJSON(w, resp)
	}
}

func (s *Server) handleAuth(h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, dto.ErrorResp(401, "missing authorization header"))
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			writeJSON(w, dto.ErrorResp(401, "invalid authorization format"))
			return
		}

		claims, err := s.jwt.Parse(tokenStr)
		if err != nil {
			writeJSON(w, dto.ErrorResp(401, "invalid or expired token"))
			return
		}

		ctx := auth.WithUserID(r.Context(), claims.UserID)
		ctx = auth.WithRoleID(ctx, claims.RoleID)

		perm, exists := routePermissions[r.URL.Path]
		if exists && perm != "" {
			parts := strings.SplitN(perm, ":", 2)
			if len(parts) == 2 {
				ok, err := s.rbac.HasPermission(ctx, claims.RoleID, parts[0], parts[1])
				if err != nil {
					s.logger.Error("rbac check failed", logger.F("error", err), logger.F("role_id", claims.RoleID), logger.F("path", r.URL.Path))
					writeJSON(w, dto.ErrorResp(500, "permission check failed"))
					return
				}
				if !ok {
					s.logger.Warn("permission denied",
						logger.F("role_id", claims.RoleID),
						logger.F("user_id", claims.UserID),
						logger.F("required", perm),
						logger.F("path", r.URL.Path),
					)
					writeJSON(w, dto.ErrorResp(403, "insufficient permissions"))
					return
				}
			}
		}

		h2 := h
		resp := h2(r.WithContext(ctx))
		writeJSON(w, resp)
	}
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, dto.ErrorResp(401, "missing authorization header"))
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			writeJSON(w, dto.ErrorResp(401, "invalid authorization format"))
			return
		}

		claims, err := s.jwt.Parse(tokenStr)
		if err != nil {
			writeJSON(w, dto.ErrorResp(401, "invalid or expired token"))
			return
		}

		ctx := auth.WithUserID(r.Context(), claims.UserID)
		ctx = auth.WithRoleID(ctx, claims.RoleID)

		next(w, r.WithContext(ctx))
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

type Handler struct {
	log         logger.Logger
	scenes      repo.SceneRepo
	nodes       repo.NodeRepo
	edges       repo.EdgeRepo
	variables   repo.VariableRepo
	plugins     repo.PluginConfigRepo
	reports     repo.ReportRepo
	runs        repo.RunRecordRepo
	users       repo.UserRepo
	roles       repo.RoleRepo
	perms       repo.PermissionRepo
	rp          repo.RolePermissionRepo
	dataSources repo.DataSourceRepo
	soPlugins   repo.SOPluginRepo
	soLoader    *so.Loader
	tracer      *tracelib.Tracer
	traceStore  *tracestore.Store
	tsStore     runner.TimeSeriesStore
	runnerMgr   *runner.Manager
	jwt         *auth.JWTManager
	rbac        *auth.RBACChecker
	globalVars  map[string]string
}
