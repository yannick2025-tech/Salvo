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

	scenes  repo.SceneRepo
	nodes   repo.NodeRepo
	edges   repo.EdgeRepo
	runs    repo.RunRecordRepo
	reports repo.ReportRepo
	tracer  *tracelib.Tracer
	log     logger.Logger
}

func NewManager(scenes repo.SceneRepo, nodes repo.NodeRepo, edges repo.EdgeRepo, runs repo.RunRecordRepo, reports repo.ReportRepo, tracer *tracelib.Tracer, log logger.Logger) *Manager {
	return &Manager{
		runners: make(map[snowflake.ID]*Runner),
		scenes:  scenes,
		nodes:   nodes,
		edges:   edges,
		runs:    runs,
		reports: reports,
		tracer:  tracer,
		log:     log,
	}
}

func (m *Manager) Start(ctx context.Context, cfg Config) (*Runner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.runners[cfg.SceneID]; exists {
		return nil, fmt.Errorf("scene %d is already running", cfg.SceneID)
	}

	r, err := New(cfg, m.scenes, m.nodes, m.edges, m.runs, m.reports, m.tracer, m.log)
	if err != nil {
		return nil, err
	}

	m.runners[cfg.SceneID] = r

	go func() {
		_ = r.Run(context.Background())
		m.mu.Lock()
		delete(m.runners, cfg.SceneID)
		m.mu.Unlock()
	}()

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
