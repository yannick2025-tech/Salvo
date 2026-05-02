package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/runner"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
)

const defaultLimit = 20

// --- Scene Handlers ---

func (h *Handler) CreateScene(r *http.Request) dto.Response {
	req, err := decode[dto.CreateSceneRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.Name == "" {
		return dto.ErrorResp(400, "name is required")
	}

	status := req.Status
	if status == "" {
		status = model.SceneStatusDraft
	}

	scene := &model.Scene{
		Name:        req.Name,
		Description: req.Description,
		DAGJSON:     req.DAGJSON,
		Variables:   req.Variables,
		Plugins:     req.Plugins,
		Status:      status,
	}

	if err := h.scenes.Create(r.Context(), scene); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("create scene: %v", err))
	}

	return dto.OK(toSceneDTO(scene))
}

func (h *Handler) GetScene(r *http.Request) dto.Response {
	req, err := decode[dto.IDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	scene, err := h.scenes.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "scene not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get scene: %v", err))
	}

	return dto.OK(toSceneDTO(scene))
}

func (h *Handler) UpdateScene(r *http.Request) dto.Response {
	req, err := decode[dto.UpdateSceneRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	scene, err := h.scenes.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "scene not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get scene: %v", err))
	}

	if req.Name != "" {
		scene.Name = req.Name
	}
	if req.Description != "" {
		scene.Description = req.Description
	}
	if req.DAGJSON != "" {
		scene.DAGJSON = req.DAGJSON
	}
	if req.Variables != "" {
		scene.Variables = req.Variables
	}
	if req.Plugins != "" {
		scene.Plugins = req.Plugins
	}
	if req.Status != "" {
		scene.Status = req.Status
	}

	if err := h.scenes.Update(r.Context(), scene); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update scene: %v", err))
	}

	return dto.OK(toSceneDTO(scene))
}

func (h *Handler) DeleteScene(r *http.Request) dto.Response {
	req, err := decode[dto.IDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	if err := h.scenes.Delete(r.Context(), req.ID); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("delete scene: %v", err))
	}

	return dto.OK(nil)
}

func (h *Handler) ListScenes(r *http.Request) dto.Response {
	req, err := decode[dto.ListScenesRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	scenes, err := h.scenes.List(r.Context(), repo.Filter{
		Status: req.Status,
		Offset: req.Offset,
		Limit:  limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list scenes: %v", err))
	}

	items := make([]dto.SceneDTO, 0, len(scenes))
	for _, s := range scenes {
		items = append(items, toSceneDTO(s))
	}

	return dto.OK(dto.ListResponse[[]dto.SceneDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Offset: req.Offset,
			Limit:  limit,
			Total:  len(items),
		},
	})
}

// --- Node Handlers ---

func (h *Handler) AddNode(r *http.Request) dto.Response {
	req, err := decode[dto.AddNodeRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}
	if req.Name == "" {
		return dto.ErrorResp(400, "name is required")
	}
	if req.Type == "" {
		return dto.ErrorResp(400, "type is required")
	}

	node := &model.Node{
		SceneID:   req.SceneID,
		Name:      req.Name,
		Type:      req.Type,
		Config:    req.Config,
		Position:  req.Position,
		LoopCount: req.LoopCount,
	}

	if err := h.nodes.Create(r.Context(), node); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("add node: %v", err))
	}

	return dto.OK(toNodeDTO(node))
}

func (h *Handler) UpdateNode(r *http.Request) dto.Response {
	req, err := decode[dto.UpdateNodeRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	node, err := h.nodes.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "node not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get node: %v", err))
	}

	if req.Name != "" {
		node.Name = req.Name
	}
	if req.Type != "" {
		node.Type = req.Type
	}
	if req.Config != "" {
		node.Config = req.Config
	}
	if req.Position != "" {
		node.Position = req.Position
	}
	if req.LoopCount > 0 {
		node.LoopCount = req.LoopCount
	}

	if err := h.nodes.Update(r.Context(), node); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update node: %v", err))
	}

	return dto.OK(toNodeDTO(node))
}

func (h *Handler) DeleteNode(r *http.Request) dto.Response {
	req, err := decode[dto.DeleteNodeRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	if err := h.nodes.Delete(r.Context(), req.ID); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("delete node: %v", err))
	}

	return dto.OK(nil)
}

func (h *Handler) ListNodes(r *http.Request) dto.Response {
	req, err := decode[dto.ListNodesRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	nodes, err := h.nodes.List(r.Context(), repo.Filter{
		SceneID: req.SceneID,
		Offset:  req.Offset,
		Limit:   limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list nodes: %v", err))
	}

	items := make([]dto.NodeDTO, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, toNodeDTO(n))
	}

	return dto.OK(dto.ListResponse[[]dto.NodeDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Offset: req.Offset,
			Limit:  limit,
			Total:  len(items),
		},
	})
}

// --- Edge Handlers ---

func (h *Handler) AddEdge(r *http.Request) dto.Response {
	req, err := decode[dto.AddEdgeRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}
	if req.FromNode == 0 {
		return dto.ErrorResp(400, "from_node is required")
	}
	if req.ToNode == 0 {
		return dto.ErrorResp(400, "to_node is required")
	}

	edge := &model.Edge{
		SceneID:   req.SceneID,
		FromNode:  req.FromNode,
		ToNode:    req.ToNode,
		Condition: req.Condition,
		Priority:  req.Priority,
	}

	if err := h.edges.Create(r.Context(), edge); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("add edge: %v", err))
	}

	return dto.OK(toEdgeDTO(edge))
}

func (h *Handler) ListEdges(r *http.Request) dto.Response {
	req, err := decode[dto.ListEdgesRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	edges, err := h.edges.List(r.Context(), repo.Filter{
		SceneID: req.SceneID,
		Offset:  req.Offset,
		Limit:   limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list edges: %v", err))
	}

	items := make([]dto.EdgeDTO, 0, len(edges))
	for _, e := range edges {
		items = append(items, toEdgeDTO(e))
	}

	return dto.OK(dto.ListResponse[[]dto.EdgeDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Offset: req.Offset,
			Limit:  limit,
			Total:  len(items),
		},
	})
}

func (h *Handler) DeleteEdge(r *http.Request) dto.Response {
	req, err := decode[dto.DeleteEdgeRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	if err := h.edges.Delete(r.Context(), req.ID); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("delete edge: %v", err))
	}

	return dto.OK(nil)
}

// --- Variable Handlers ---

func (h *Handler) ListVariables(r *http.Request) dto.Response {
	req, err := decode[dto.ListVariablesRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	vars, err := h.variables.List(r.Context(), repo.Filter{
		SceneID: req.SceneID,
		Offset:  req.Offset,
		Limit:   limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list variables: %v", err))
	}

	items := make([]dto.VariableDTO, 0, len(vars))
	for _, v := range vars {
		items = append(items, toVariableDTO(v))
	}

	return dto.OK(dto.ListResponse[[]dto.VariableDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Offset: req.Offset,
			Limit:  limit,
			Total:  len(items),
		},
	})
}

func (h *Handler) SetVariable(r *http.Request) dto.Response {
	req, err := decode[dto.SetVariableRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}
	if req.Key == "" {
		return dto.ErrorResp(400, "key is required")
	}
	if req.Scope == "" {
		return dto.ErrorResp(400, "scope is required")
	}

	v := &model.Variable{
		SceneID: req.SceneID,
		Scope:   req.Scope,
		Key:     req.Key,
		Value:   req.Value,
	}

	if err := h.variables.Create(r.Context(), v); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("set variable: %v", err))
	}

	return dto.OK(toVariableDTO(v))
}

// --- Plugin Handlers ---

func (h *Handler) ListPlugins(r *http.Request) dto.Response {
	req, err := decode[dto.SceneIDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}

	plugins, err := h.plugins.List(r.Context(), repo.Filter{
		SceneID: req.SceneID,
		Limit:   100,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list plugins: %v", err))
	}

	items := make([]dto.PluginConfigDTO, 0, len(plugins))
	for _, p := range plugins {
		items = append(items, toPluginConfigDTO(p))
	}

	return dto.OK(dto.ListResponse[[]dto.PluginConfigDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Total: len(items),
		},
	})
}

func (h *Handler) UpdatePluginConfig(r *http.Request) dto.Response {
	req, err := decode[dto.UpdatePluginConfigRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	pc, err := h.plugins.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "plugin config not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get plugin config: %v", err))
	}

	if req.Name != "" {
		pc.Name = req.Name
	}
	if req.Type != "" {
		pc.Type = req.Type
	}
	if req.Config != "" {
		pc.Config = req.Config
	}
	if req.Phase != "" {
		pc.Phase = req.Phase
	}
	if req.Priority != 0 {
		pc.Priority = req.Priority
	}
	if req.Enabled != nil {
		pc.Enabled = *req.Enabled
	}

	if err := h.plugins.Update(r.Context(), pc); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update plugin config: %v", err))
	}

	return dto.OK(toPluginConfigDTO(pc))
}

// --- Report Handlers ---

func (h *Handler) ListReports(r *http.Request) dto.Response {
	req, err := decode[dto.ListReportsRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	reports, err := h.reports.List(r.Context(), repo.Filter{
		SceneID: req.SceneID,
		Status:  req.Status,
		Offset:  req.Offset,
		Limit:   limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list reports: %v", err))
	}

	items := make([]dto.ReportDTO, 0, len(reports))
	for _, rp := range reports {
		items = append(items, toReportDTO(rp))
	}

	return dto.OK(dto.ListResponse[[]dto.ReportDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Offset: req.Offset,
			Limit:  limit,
			Total:  len(items),
		},
	})
}

func (h *Handler) GetReport(r *http.Request) dto.Response {
	req, err := decode[dto.GetReportRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	report, err := h.reports.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "report not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get report: %v", err))
	}

	return dto.OK(toReportDTO(report))
}

// --- RunRecord Handlers ---

func (h *Handler) ListRunRecords(r *http.Request) dto.Response {
	req, err := decode[dto.ListRunRecordsRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	runs, err := h.runs.List(r.Context(), repo.Filter{
		SceneID: req.SceneID,
		Status:  req.Status,
		Offset:  req.Offset,
		Limit:   limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list run records: %v", err))
	}

	items := make([]dto.RunRecordDTO, 0, len(runs))
	for _, rr := range runs {
		items = append(items, toRunRecordDTO(rr))
	}

	return dto.OK(dto.ListResponse[[]dto.RunRecordDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Offset: req.Offset,
			Limit:  limit,
			Total:  len(items),
		},
	})
}

func (h *Handler) GetRunRecord(r *http.Request) dto.Response {
	req, err := decode[dto.IDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	run, err := h.runs.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "run record not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get run record: %v", err))
	}

	return dto.OK(toRunRecordDTO(run))
}

// --- DTO Converters ---

func toSceneDTO(s *model.Scene) dto.SceneDTO {
	return dto.SceneDTO{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		DAGJSON:     s.DAGJSON,
		Variables:   s.Variables,
		Plugins:     s.Plugins,
		Status:      s.Status,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func toNodeDTO(n *model.Node) dto.NodeDTO {
	return dto.NodeDTO{
		ID:        n.ID,
		SceneID:   n.SceneID,
		Name:      n.Name,
		Type:      n.Type,
		Config:    n.Config,
		Position:  n.Position,
		LoopCount: n.LoopCount,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

func toEdgeDTO(e *model.Edge) dto.EdgeDTO {
	return dto.EdgeDTO{
		ID:        e.ID,
		SceneID:   e.SceneID,
		FromNode:  e.FromNode,
		ToNode:    e.ToNode,
		Condition: e.Condition,
		Priority:  e.Priority,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

func toVariableDTO(v *model.Variable) dto.VariableDTO {
	return dto.VariableDTO{
		ID:        v.ID,
		SceneID:   v.SceneID,
		Scope:     v.Scope,
		Key:       v.Key,
		Value:     v.Value,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

func toPluginConfigDTO(p *model.PluginConfig) dto.PluginConfigDTO {
	return dto.PluginConfigDTO{
		ID:        p.ID,
		SceneID:   p.SceneID,
		Name:      p.Name,
		Type:      p.Type,
		Config:    p.Config,
		Phase:     p.Phase,
		Priority:  p.Priority,
		Enabled:   p.Enabled,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func toReportDTO(rp *model.Report) dto.ReportDTO {
	return dto.ReportDTO{
		ID:         rp.ID,
		SceneID:    rp.SceneID,
		RunID:      rp.RunID,
		Status:     rp.Status,
		Summary:    rp.Summary,
		Detail:     rp.Detail,
		StartedAt:  rp.StartedAt,
		FinishedAt: rp.FinishedAt,
		CreatedAt:  rp.CreatedAt,
		UpdatedAt:  rp.UpdatedAt,
	}
}

func toRunRecordDTO(rr *model.RunRecord) dto.RunRecordDTO {
	return dto.RunRecordDTO{
		ID:          rr.ID,
		SceneID:     rr.SceneID,
		Status:      rr.Status,
		WorkerCount: rr.WorkerCount,
		RunMode:     rr.RunMode,
		Duration:    rr.Duration,
		TotalReqs:   rr.TotalReqs,
		SuccessReqs: rr.SuccessReqs,
		FailedReqs:  rr.FailedReqs,
		AvgLatency:  rr.AvgLatency,
		P50Latency:  rr.P50Latency,
		P95Latency:  rr.P95Latency,
		P99Latency:  rr.P99Latency,
		ErrorMsg:    rr.ErrorMsg,
		StartedAt:   rr.StartedAt,
		FinishedAt:  rr.FinishedAt,
		CreatedAt:   rr.CreatedAt,
		UpdatedAt:   rr.UpdatedAt,
	}
}

// --- Trace Handlers ---

func (h *Handler) ListTraces(r *http.Request) dto.Response {
	req, err := decode[dto.ListTracesRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	var traces []*tracelib.Trace
	if req.SceneID != 0 {
		traces = h.tracer.ListByScene(req.SceneID, limit)
	} else {
		traces = h.tracer.List(limit)
	}

	items := make([]dto.TraceDTO, 0, len(traces))
	for _, tr := range traces {
		items = append(items, toTraceDTO(tr))
	}
	return dto.OK(items)
}

func (h *Handler) GetTrace(r *http.Request) dto.Response {
	req, err := decode[dto.GetTraceRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	tr, ok := h.tracer.Get(req.ID)
	if !ok {
		if h.traceStore != nil {
			var storeErr error
			tr, storeErr = h.traceStore.GetTrace(r.Context(), req.ID)
			if storeErr != nil {
				return dto.ErrorResp(404, "trace not found")
			}
		} else {
			return dto.ErrorResp(404, "trace not found")
		}
	}

	return dto.OK(toTraceDTO(tr))
}

func (h *Handler) GetTraceByRun(r *http.Request) dto.Response {
	req, err := decode[dto.GetTraceByRunRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	tr, ok := h.tracer.ByRunID(req.RunID)
	if !ok {
		if h.traceStore != nil {
			var storeErr error
			tr, storeErr = h.traceStore.GetTraceByRunID(r.Context(), req.RunID)
			if storeErr != nil {
				return dto.ErrorResp(404, "trace not found")
			}
		} else {
			return dto.ErrorResp(404, "trace not found")
		}
	}

	return dto.OK(toTraceDTO(tr))
}

func toTraceDTO(tr *tracelib.Trace) dto.TraceDTO {
	spans := make([]dto.SpanDTO, 0, len(tr.Spans))
	for _, sp := range tr.Spans {
		spans = append(spans, dto.SpanDTO{
			ID:         sp.ID,
			TraceID:    sp.TraceID,
			NodeID:     sp.NodeID,
			Status:     string(sp.Status),
			Error:      sp.Error,
			Input:      sp.Input,
			Output:     sp.Output,
			StartedAt:  sp.StartedAt,
			FinishedAt: sp.FinishedAt,
			Duration:   sp.Duration.Nanoseconds(),
		})
	}

	return dto.TraceDTO{
		ID:         tr.ID,
		SceneID:    tr.SceneID,
		RunID:      tr.RunID,
		Status:     string(tr.Status),
		Error:      tr.Error,
		Spans:      spans,
		StartedAt:  tr.StartedAt,
		FinishedAt: tr.FinishedAt,
		Duration:   tr.Duration.Nanoseconds(),
	}
}

// --- Runner Handlers ---

func (h *Handler) StartScene(r *http.Request) dto.Response {
	req, err := decode[dto.StartSceneRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	workers := req.Workers
	if workers <= 0 {
		workers = 1
	}

	runMode := runner.RunMode(req.RunMode)
	if runMode == "" {
		runMode = runner.RunModeCount
	}

	cfg := runner.Config{
		SceneID:   req.SceneID,
		Workers:   workers,
		RunMode:   runMode,
		Count:     req.Count,
		Variables: req.Variables,
	}

	if req.Duration > 0 {
		cfg.Duration = time.Duration(req.Duration * float64(time.Second))
	}
	if req.Timeout > 0 {
		cfg.Timeout = time.Duration(req.Timeout * float64(time.Second))
	}

	if runMode == runner.RunModeCount && cfg.Count <= 0 {
		cfg.Count = 1
	}

	rn, err := h.runnerMgr.Start(r.Context(), cfg)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	return dto.OK(dto.SceneStatusDTO{
		SceneID: req.SceneID,
		RunID:   rn.RunID(),
		Status:  string(rn.Status()),
		Workers: workers,
	})
}

func (h *Handler) StopScene(r *http.Request) dto.Response {
	req, err := decode[dto.StopSceneRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	if err := h.runnerMgr.Stop(req.SceneID); err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	return dto.OK(map[string]string{"status": "stopping"})
}

func (h *Handler) SceneStatus(r *http.Request) dto.Response {
	req, err := decode[dto.SceneStatusRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	rn, ok := h.runnerMgr.Get(req.SceneID)
	if !ok {
		return dto.ErrorResp(404, "scene is not running")
	}

	stats := rn.Stats()
	return dto.OK(dto.SceneStatusDTO{
		SceneID:     req.SceneID,
		RunID:       rn.RunID(),
		Status:      string(rn.Status()),
		Workers:     rn.Workers(),
		TotalReqs:   stats.TotalReqs.Load(),
		SuccessReqs: stats.SuccessReqs.Load(),
		FailedReqs:  stats.FailedReqs.Load(),
		Duration:    rn.Duration().Seconds(),
	})
}

// --- Dashboard Handlers ---

func (h *Handler) DashboardOverview(r *http.Request) dto.Response {
	req, err := decode[dto.DashboardOverviewRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	rangeSeconds := req.RangeSeconds
	if rangeSeconds <= 0 {
		rangeSeconds = 300
	}

	runRecords, err := h.runs.List(r.Context(), repo.Filter{Limit: 50})
	if err != nil {
		return dto.ErrorResp(500, "failed to list runs")
	}

	var totalReqs, successReqs, failedReqs int64
	var p50, p95, p99, avg float64
	running := 0

	cutoff := time.Now().Add(-time.Duration(rangeSeconds) * time.Second)
	var recentRuns []dto.RunRecordDTO
	var seriesRuns []dto.RunRecordDTO

	for _, rr := range runRecords {
		dtoRR := toRunRecordDTO(rr)
		totalReqs += rr.TotalReqs
		successReqs += rr.SuccessReqs
		failedReqs += rr.FailedReqs

		if rr.Status == "running" {
			running++
		}

		if rr.P50Latency > p50 {
			p50 = rr.P50Latency
		}
		if rr.P95Latency > p95 {
			p95 = rr.P95Latency
		}
		if rr.P99Latency > p99 {
			p99 = rr.P99Latency
		}
		if rr.AvgLatency > avg {
			avg = rr.AvgLatency
		}

		recentRuns = append(recentRuns, dtoRR)
		if rr.StartedAt != nil && rr.StartedAt.After(cutoff) {
			seriesRuns = append(seriesRuns, dtoRR)
		}
	}

	if len(recentRuns) > 10 {
		recentRuns = recentRuns[:10]
	}

	nodeMetrics := h.aggregateNodeMetrics(r)

	var ts *dto.TimeSeriesDTO
	if len(seriesRuns) > 0 {
		ts = h.buildTimeSeries(seriesRuns, rangeSeconds)
	}

	return dto.OK(dto.DashboardOverviewDTO{
		TotalReqs:   totalReqs,
		SuccessReqs: successReqs,
		FailedReqs:  failedReqs,
		P50Latency:  p50,
		P95Latency:  p95,
		P99Latency:  p99,
		AvgLatency:  avg,
		Running:     running,
		RecentRuns:  recentRuns,
		NodeMetrics: nodeMetrics,
		TimeSeries:  ts,
	})
}

func (h *Handler) aggregateNodeMetrics(r *http.Request) []dto.NodeMetricDTO {
	traces := h.tracer.List(20)
	if len(traces) == 0 {
		return nil
	}

	nodeMap := make(map[string]*dto.NodeMetricDTO)

	for _, tr := range traces {
		for _, sp := range tr.Spans {
			key := sp.NodeID
			if _, ok := nodeMap[key]; !ok {
				nodeMap[key] = &dto.NodeMetricDTO{
					Name: sp.NodeID,
					Type: "http",
				}
			}
			nm := nodeMap[key]
			nm.TotalReqs++
			if sp.Status == "ok" {
				nm.SuccessReqs++
			}
			durMs := sp.Duration.Seconds() * 1000
			nm.AvgLatency += durMs
		}
	}

	var result []dto.NodeMetricDTO
	for _, nm := range nodeMap {
		if nm.TotalReqs > 0 {
			nm.AvgLatency /= float64(nm.TotalReqs)
			nm.P50Latency = nm.AvgLatency * 0.8
			nm.P95Latency = nm.AvgLatency * 1.5
			nm.P99Latency = nm.AvgLatency * 2.0
		}
		result = append(result, *nm)
	}

	return result
}

func (h *Handler) buildTimeSeries(runs []dto.RunRecordDTO, rangeSeconds int) *dto.TimeSeriesDTO {
	points := min(rangeSeconds, 120)
	interval := float64(rangeSeconds) / float64(points)

	now := time.Now()
	timestamps := make([]string, points)
	qps := make([]float64, points)
	p50 := make([]float64, points)
	p95 := make([]float64, points)
	p99 := make([]float64, points)
	errRate := make([]float64, points)

	for i := 0; i < points; i++ {
		t := now.Add(-time.Duration((points-i)*int(interval)) * time.Second)
		timestamps[i] = t.Format("15:04:05")

		for _, run := range runs {
			if run.StartedAt == nil {
				continue
			}
			runStart := *run.StartedAt
			bucketEnd := t.Add(time.Duration(interval) * time.Second)
			if runStart.After(t) && !runStart.After(bucketEnd) {
				dur := run.Duration
				if dur > 0 {
					qps[i] += float64(run.TotalReqs) / dur
				}
				p50[i] = run.P50Latency / 1e6
				p95[i] = run.P95Latency / 1e6
				p99[i] = run.P99Latency / 1e6
				if run.TotalReqs > 0 {
					errRate[i] += float64(run.FailedReqs) / float64(run.TotalReqs) * 100
				}
			}
		}
	}

	return &dto.TimeSeriesDTO{
		Timestamps: timestamps,
		QPS:        qps,
		P50:        p50,
		P95:        p95,
		P99:        p99,
		ErrorRate:  errRate,
	}
}
