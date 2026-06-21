package runner

import (
	"context"
	"fmt"
	"sync"

	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
)

type Manager struct {
	mu      sync.Mutex
	runners map[snowflake.ID]*Runner

	scenes      repo.SceneRepo
	nodes       repo.NodeRepo
	edges       repo.EdgeRepo
	runs        repo.RunRecordRepo
	reports     repo.ReportRepo
	dataSources repo.DataSourceRepo
	tracer      *tracelib.Tracer
	tsStore     TimeSeriesStore
	log         logger.Logger
}

func NewManager(scenes repo.SceneRepo, nodes repo.NodeRepo, edges repo.EdgeRepo, runs repo.RunRecordRepo, reports repo.ReportRepo, dataSources repo.DataSourceRepo, tracer *tracelib.Tracer, tsStore TimeSeriesStore, log logger.Logger) *Manager {
	return &Manager{
		runners:     make(map[snowflake.ID]*Runner),
		scenes:      scenes,
		nodes:       nodes,
		edges:       edges,
		runs:        runs,
		reports:     reports,
		dataSources: dataSources,
		tracer:      tracer,
		tsStore:     tsStore,
		log:         log,
	}
}

func (m *Manager) Start(ctx context.Context, cfg Config) (*Runner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.runners[cfg.SceneID]; exists {
		return nil, fmt.Errorf("scene %d is already running", cfg.SceneID)
	}

	r, err := New(cfg, m.scenes, m.nodes, m.edges, m.runs, m.reports, m.dataSources, m.tracer, m.tsStore, m.log)
	if err != nil {
		return nil, err
	}

	m.runners[cfg.SceneID] = r

	safeGo(context.Background(), m.log, "manager-start", func() {
		err := r.Run(context.Background())
		if err != nil {
			r.setError(err)
			m.log.Error("runner run failed",
				logger.F("scene_id", cfg.SceneID.String()),
				logger.F("run_id", r.runID.String()),
				logger.F("error", err),
			)
		}
		m.mu.Lock()
		delete(m.runners, cfg.SceneID)
		m.mu.Unlock()
	})

	return r, nil
}

func (m *Manager) Stop(sceneID snowflake.ID) error {
	m.mu.Lock()
	r, exists := m.runners[sceneID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("scene %d is not running", sceneID)
	}

	r.Stop()
	return nil
}

func (m *Manager) Get(sceneID snowflake.ID) (*Runner, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runners[sceneID]
	return r, ok
}

func (m *Manager) List() map[snowflake.ID]*Runner {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make(map[snowflake.ID]*Runner, len(m.runners))
	for k, v := range m.runners {
		cp[k] = v
	}
	return cp
}
