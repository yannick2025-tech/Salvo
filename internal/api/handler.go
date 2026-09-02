package api

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/generator/builtin"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/runner"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
	tracelib "github.com/yannick2025-tech/Salvo/internal/trace"
	"gopkg.in/yaml.v3"
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
		Name:           req.Name,
		Description:    req.Description,
		DAGJSON:        req.DAGJSON,
		Variables:      req.Variables,
		Plugins:        req.Plugins,
		Status:         status,
		DefaultTimeout: req.DefaultTimeout,
	}

	if err := h.scenes.Create(r.Context(), scene); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("create scene: %v", err))
	}

	return dto.OK(toSceneDTO(scene))
}

type yamlScene struct {
	Name           string            `yaml:"name"`
	DefaultTimeout int               `yaml:"default_timeout,omitempty"`
	Description    string            `yaml:"description"`
	Variables      []yamlVarItem     `yaml:"variables,omitempty"`
	ConfigParams   map[string]string `yaml:"config_params,omitempty"`
	DerivedParams  map[string]string `yaml:"derived_params,omitempty"`
	DataSources    []yamlDataSource  `yaml:"data_sources,omitempty"`
	Setup          []yamlNode        `yaml:"setup,omitempty"`
	Nodes          []yamlNode        `yaml:"nodes"`
	Teardown       []yamlNode        `yaml:"teardown,omitempty"`
	Edges          []yamlEdge        `yaml:"edges,omitempty"`
}

type yamlDataSource struct {
	Name    string              `yaml:"name"`
	Columns []string            `yaml:"columns"`
	Rows    []map[string]string `yaml:"rows"`
}

type yamlRetry struct {
	Count    int    `yaml:"count" json:"count"`
	Interval string `yaml:"interval" json:"interval"`
}

type yamlNode struct {
	Name         string         `yaml:"name"`
	Type         string         `yaml:"type"`
	Config       map[string]any `yaml:"config,omitempty"`
	ThinkTime    string         `yaml:"think_time,omitempty"`
	Retry        *yamlRetry     `yaml:"retry,omitempty"`
	Condition    string         `yaml:"condition,omitempty"`
	TimedTrigger string         `yaml:"timed_trigger,omitempty"`
	BlockOnError bool           `yaml:"block_on_error,omitempty"`
}

type yamlEdge struct {
	From      string `yaml:"from"`
	To        string `yaml:"to"`
	Condition string `yaml:"condition,omitempty"`
}

type yamlVarItem struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

func (h *Handler) ImportYAML(r *http.Request) dto.Response {
	req, err := decode[dto.ImportYAMLRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.YAML == "" {
		return dto.ErrorResp(400, "yaml content is required")
	}

	var ys yamlScene
	if err := yaml.Unmarshal([]byte(req.YAML), &ys); err != nil {
		return dto.ErrorResp(400, fmt.Sprintf("invalid yaml: %v", err))
	}

	name := req.Name
	if name == "" {
		name = ys.Name
	}
	if name == "" {
		return dto.ErrorResp(400, "scene name is required")
	}

	// Store variables, config_params, derived_params as separate JSON fields
	// so export can reconstruct the original YAML sections without heuristics.
	var varsJSON string
	vm := make(map[string]string)
	for _, v := range ys.Variables {
		vm[v.Key] = v.Value
	}
	if len(vm) > 0 {
		vb, _ := json.Marshal(vm)
		varsJSON = string(vb)
	}
	var configParamsJSON string
	if len(ys.ConfigParams) > 0 {
		vb, _ := json.Marshal(ys.ConfigParams)
		configParamsJSON = string(vb)
	}
	var derivedParamsJSON string
	if len(ys.DerivedParams) > 0 {
		vb, _ := json.Marshal(ys.DerivedParams)
		derivedParamsJSON = string(vb)
	}

	scene := &model.Scene{
		Name:          name,
		Description:   req.Description,
		Variables:     varsJSON,
		ConfigParams:  configParamsJSON,
		DerivedParams: derivedParamsJSON,
		Status:        model.SceneStatusDraft,
	}
	if scene.Description == "" {
		scene.Description = ys.Description
	}
	if ys.DefaultTimeout > 0 {
		scene.DefaultTimeout = ys.DefaultTimeout
	}

	if err := h.scenes.Create(r.Context(), scene); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("create scene: %v", err))
	}

	// Import data sources: create DataSource records and build name→ID map.
	dsNameToID := make(map[string]snowflake.ID)
	for _, yds := range ys.DataSources {
		if yds.Name == "" {
			return dto.ErrorResp(400, "data source name is required")
		}
		// Skip if a CSV-uploaded data source with same name already exists
		existingCSV, _ := h.dataSources.GetBySceneIDAndNameAndSource(r.Context(), scene.ID, yds.Name, "csv")
		if existingCSV != nil {
			dsNameToID[yds.Name] = existingCSV.ID
			continue
		}
		// Delete existing yaml data source with same name before creating
		existingYAML, _ := h.dataSources.GetBySceneIDAndNameAndSource(r.Context(), scene.ID, yds.Name, "yaml")
		if existingYAML != nil {
			_ = h.dataSources.Delete(r.Context(), existingYAML.ID)
		}
		columnsJSON, _ := json.Marshal(yds.Columns)
		rowsJSON, _ := json.Marshal(yds.Rows)
		ds := &model.DataSource{
			SceneID:  scene.ID,
			Name:     yds.Name,
			FileName: yds.Name + ".csv",
			Columns:  string(columnsJSON),
			Rows:     string(rowsJSON),
			RowCount: len(yds.Rows),
			Source:   "yaml",
		}
		if err := h.dataSources.Create(r.Context(), ds); err != nil {
			return dto.ErrorResp(500, fmt.Sprintf("create data source %q: %v", yds.Name, err))
		}
		dsNameToID[yds.Name] = ds.ID
	}

	nodeNameToID := make(map[string]snowflake.ID)

	// Build a tagged list so each node carries the YAML section (setup/nodes/teardown)
	// it came from. Lifecycle is orthogonal to Type and stored on the Node for
	// lossless reconstruction of setup/teardown sections on export.
	type taggedNode struct {
		yn        yamlNode
		lifecycle string
	}
	allNodes := make([]taggedNode, 0, len(ys.Setup)+len(ys.Nodes)+len(ys.Teardown))
	for _, yn := range ys.Setup {
		allNodes = append(allNodes, taggedNode{yn: yn, lifecycle: model.NodeLifecycleSetup})
	}
	for _, yn := range ys.Nodes {
		allNodes = append(allNodes, taggedNode{yn: yn, lifecycle: model.NodeLifecycleMain})
	}
	for _, yn := range ys.Teardown {
		allNodes = append(allNodes, taggedNode{yn: yn, lifecycle: model.NodeLifecycleTeardown})
	}

	for _, item := range allNodes {
		yn := item.yn
		if err := validateNodeName(yn.Name); err != nil {
			return dto.ErrorResp(400, fmt.Sprintf("node %q: %v", yn.Name, err))
		}

		// Merge think_time, retry, condition, timed_trigger into Config.
		if yn.Config == nil {
			yn.Config = make(map[string]any)
		}
		if yn.ThinkTime != "" {
			yn.Config["think_time"] = yn.ThinkTime
		}
		if yn.Retry != nil {
			yn.Config["retry"] = yn.Retry
		}
		if yn.Condition != "" {
			yn.Config["condition"] = yn.Condition
		}
		if yn.TimedTrigger != "" {
			yn.Config["timed_trigger"] = yn.TimedTrigger
		}

		configBytes, _ := json.Marshal(yn.Config)
		node := &model.Node{
			SceneID:      scene.ID,
			Name:         yn.Name,
			Type:         yn.Type,
			Config:       string(configBytes),
			BlockOnError: yn.BlockOnError,
			Lifecycle:    item.lifecycle,
		}
		if err := h.nodes.Create(r.Context(), node); err != nil {
			return dto.ErrorResp(500, fmt.Sprintf("create node %q: %v", yn.Name, err))
		}
		nodeNameToID[yn.Name] = node.ID
	}

	// Resolve group node_ids from names to IDs.
	// Group config stores node_ids as string IDs; in YAML they are node names.
	for _, item := range allNodes {
		yn := item.yn
		if yn.Type != model.NodeTypeGroup {
			continue
		}
		nodeID, ok := nodeNameToID[yn.Name]
		if !ok {
			continue
		}
		nodeNames, _ := yn.Config["node_ids"].([]any)
		resolvedIDs := make([]string, 0, len(nodeNames))
		for _, nameVal := range nodeNames {
			name, ok := nameVal.(string)
			if !ok {
				return dto.ErrorResp(400, fmt.Sprintf("group node %q: node_ids must be strings", yn.Name))
			}
			childID, ok := nodeNameToID[name]
			if !ok {
				return dto.ErrorResp(400, fmt.Sprintf("group node %q: child node %q not found", yn.Name, name))
			}
			resolvedIDs = append(resolvedIDs, childID.String())
		}
		yn.Config["node_ids"] = resolvedIDs
		configBytes, _ := json.Marshal(yn.Config)
		node, err := h.nodes.GetByID(r.Context(), nodeID)
		if err != nil {
			return dto.ErrorResp(500, fmt.Sprintf("get group node %q: %v", yn.Name, err))
		}
		node.Config = string(configBytes)
		if err := h.nodes.Update(r.Context(), node); err != nil {
			return dto.ErrorResp(500, fmt.Sprintf("update group node %q: %v", yn.Name, err))
		}
	}

	if len(ys.Edges) > 0 {
		for _, ye := range ys.Edges {
			fromID, ok := nodeNameToID[ye.From]
			if !ok {
				return dto.ErrorResp(400, fmt.Sprintf("edge from node %q not found", ye.From))
			}
			toID, ok := nodeNameToID[ye.To]
			if !ok {
				return dto.ErrorResp(400, fmt.Sprintf("edge to node %q not found", ye.To))
			}
			edge := &model.Edge{
				SceneID:   scene.ID,
				FromNode:  fromID,
				ToNode:    toID,
				Condition: ye.Condition,
			}
			if err := h.edges.Create(r.Context(), edge); err != nil {
				return dto.ErrorResp(500, fmt.Sprintf("create edge: %v", err))
			}
		}
	} else {
		setupIDs := make([]snowflake.ID, 0, len(ys.Setup))
		for _, n := range ys.Setup {
			if id, ok := nodeNameToID[n.Name]; ok {
				setupIDs = append(setupIDs, id)
			}
		}
		mainIDs := make([]snowflake.ID, 0, len(ys.Nodes))
		for _, n := range ys.Nodes {
			if id, ok := nodeNameToID[n.Name]; ok {
				mainIDs = append(mainIDs, id)
			}
		}
		teardownIDs := make([]snowflake.ID, 0, len(ys.Teardown))
		for _, n := range ys.Teardown {
			if id, ok := nodeNameToID[n.Name]; ok {
				teardownIDs = append(teardownIDs, id)
			}
		}

		var lastSetupID snowflake.ID
		for i, id := range setupIDs {
			if i > 0 {
				h.edges.Create(r.Context(), &model.Edge{SceneID: scene.ID, FromNode: lastSetupID, ToNode: id})
			}
			lastSetupID = id
		}

		var lastMainID snowflake.ID
		for i, id := range mainIDs {
			if i > 0 {
				h.edges.Create(r.Context(), &model.Edge{SceneID: scene.ID, FromNode: lastMainID, ToNode: id})
			}
			lastMainID = id
		}

		if lastSetupID != 0 && len(mainIDs) > 0 {
			h.edges.Create(r.Context(), &model.Edge{SceneID: scene.ID, FromNode: lastSetupID, ToNode: mainIDs[0]})
		}

		var lastTeardownID snowflake.ID
		for i, id := range teardownIDs {
			if i > 0 {
				h.edges.Create(r.Context(), &model.Edge{SceneID: scene.ID, FromNode: lastTeardownID, ToNode: id})
			}
			lastTeardownID = id
		}

		if lastMainID != 0 && len(teardownIDs) > 0 {
			h.edges.Create(r.Context(), &model.Edge{SceneID: scene.ID, FromNode: lastMainID, ToNode: teardownIDs[0]})
		}
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
	if req.DefaultTimeout != nil {
		scene.DefaultTimeout = *req.DefaultTimeout
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

// ExportYAML serializes a scene and all its nodes, edges, variables,
// and data sources into a YAML document for backup or migration.
func (h *Handler) ExportYAML(r *http.Request) dto.Response {
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

	// Fetch all nodes.
	nodes, err := h.nodes.List(r.Context(), repo.Filter{SceneID: req.ID, Limit: 1000})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list nodes: %v", err))
	}

	// Fetch all edges.
	edges, err := h.edges.List(r.Context(), repo.Filter{SceneID: req.ID, Limit: 1000})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list edges: %v", err))
	}

	// Fetch data sources with source=yaml for YAML export.
	var yamlDataSources []yamlDataSource
	allDataSources, _ := h.dataSources.ListBySceneID(r.Context(), req.ID)
	for _, ds := range allDataSources {
		if ds.Source == "yaml" {
			var columns []string
			if ds.Columns != "" {
				json.Unmarshal([]byte(ds.Columns), &columns)
			}
			var rows []map[string]string
			if ds.Rows != "" {
				json.Unmarshal([]byte(ds.Rows), &rows)
			}
			yamlDataSources = append(yamlDataSources, yamlDataSource{
				Name:    ds.Name,
				Columns: columns,
				Rows:    rows,
			})
		}
	}

	// Read variables, config_params, derived_params from their dedicated
	// DB fields (stored separately at import time for YAML section fidelity).
	var varsList []yamlVarItem
	if scene.Variables != "" {
		var asMap map[string]string
		if err := json.Unmarshal([]byte(scene.Variables), &asMap); err == nil {
			for k, v := range asMap {
				varsList = append(varsList, yamlVarItem{Key: k, Value: v})
			}
		} else {
			var asSlice []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal([]byte(scene.Variables), &asSlice); err == nil {
				for _, v := range asSlice {
					varsList = append(varsList, yamlVarItem{Key: v.Key, Value: v.Value})
				}
			}
		}
	}
	var configParams map[string]string
	if scene.ConfigParams != "" {
		_ = json.Unmarshal([]byte(scene.ConfigParams), &configParams)
	}
	var derivedParams map[string]string
	if scene.DerivedParams != "" {
		_ = json.Unmarshal([]byte(scene.DerivedParams), &derivedParams)
	}
	// Sort variables by key for deterministic output.
	sort.Slice(varsList, func(i, j int) bool {
		return varsList[i].Key < varsList[j].Key
	})

	// Build YAML nodes.
	var yamlNodes []yamlNode
	var yamlSetup []yamlNode
	var yamlTeardown []yamlNode
	nodeNameMap := make(map[string]*model.Node)

	for _, n := range nodes {
		nodeNameMap[n.Name] = n

		parsedConfig := make(map[string]any)
		if n.Config != "" {
			json.Unmarshal([]byte(n.Config), &parsedConfig)
		}

		yn := yamlNode{
			Name:         n.Name,
			Type:         n.Type,
			Config:       nil,
			BlockOnError: n.BlockOnError,
		}

		// Extract known top-level fields back out of config.
		var extraConfig map[string]any
		if parsedConfig != nil {
			extraConfig = make(map[string]any)
			for k, v := range parsedConfig {
				switch k {
				case "think_time":
					if s, ok := v.(string); ok {
						yn.ThinkTime = s
					} else {
						extraConfig[k] = v
					}
				case "retry":
					if m, ok := v.(map[string]any); ok {
						yn.Retry = &yamlRetry{}
						if c, ok2 := m["count"].(float64); ok2 {
							yn.Retry.Count = int(c)
						}
						if iv, ok2 := m["interval"].(string); ok2 {
							yn.Retry.Interval = iv
						}
					} else {
						extraConfig[k] = v
					}
				case "condition":
					if s, ok := v.(string); ok {
						yn.Condition = s
					} else {
						extraConfig[k] = v
					}
				case "timed_trigger":
					if s, ok := v.(string); ok {
						yn.TimedTrigger = s
					} else {
						extraConfig[k] = v
					}
				default:
					extraConfig[k] = v
				}
			}
		}
		// Only set Config if there are remaining fields.
		if len(extraConfig) > 0 {
			yn.Config = extraConfig
		}

		switch n.Lifecycle {
		case model.NodeLifecycleSetup:
			yamlSetup = append(yamlSetup, yn)
		case model.NodeLifecycleTeardown:
			yamlTeardown = append(yamlTeardown, yn)
		default:
			yamlNodes = append(yamlNodes, yn)
		}
	}

	// Build YAML edges.
	var yamlEdges []yamlEdge
	for _, e := range edges {
		fromName := ""
		toName := ""
		for name, n := range nodeNameMap {
			if n.ID == e.FromNode {
				fromName = name
			}
			if n.ID == e.ToNode {
				toName = name
			}
		}
		if fromName != "" && toName != "" {
			yamlEdges = append(yamlEdges, yamlEdge{
				From:      fromName,
				To:        toName,
				Condition: e.Condition,
			})
		}
	}

	// Build YAML scene.
	ys := yamlScene{
		Name:           scene.Name,
		Description:    scene.Description,
		DefaultTimeout: scene.DefaultTimeout,
		DataSources:    yamlDataSources,
		Variables:      varsList,
		ConfigParams:   configParams,
		DerivedParams:  derivedParams,
		Setup:          yamlSetup,
		Nodes:          yamlNodes,
		Teardown:       yamlTeardown,
		Edges:          yamlEdges,
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(ys); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("marshal yaml: %v", err))
	}
	enc.Close()
	yamlBytes := buf.Bytes()

	return dto.OK(dto.ExportYAMLResponse{
		YAML: string(yamlBytes),
	})
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

var sqlInjectionPattern = regexp.MustCompile(`['";]|--|/\*|\*/`)

func validateNodeName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("node name is required")
	}
	if len(name) > 50 {
		return fmt.Errorf("node name must be at most 50 characters")
	}
	if sqlInjectionPattern.MatchString(name) {
		return fmt.Errorf("node name contains invalid characters")
	}
	return nil
}

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
	if err := validateNodeName(req.Name); err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.Type == "" {
		return dto.ErrorResp(400, "type is required")
	}

	node := &model.Node{
		SceneID:      req.SceneID,
		Name:         req.Name,
		Type:         req.Type,
		Config:       req.Config,
		Position:     req.Position,
		LoopCount:    req.LoopCount,
		BlockOnError: req.BlockOnError,
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
		if err := validateNodeName(req.Name); err != nil {
			return dto.ErrorResp(400, err.Error())
		}
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
	if req.BlockOnError != nil {
		node.BlockOnError = *req.BlockOnError
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

// BatchSetVariables replaces all scene-level variables with the provided map.
// It serializes the variables map to JSON and updates the scene's Variables field.
func (h *Handler) BatchSetVariables(r *http.Request) dto.Response {
	req, err := decode[dto.BatchSetVariablesRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}

	scene, err := h.scenes.GetByID(r.Context(), req.SceneID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "scene not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get scene: %v", err))
	}

	varsJSON, err := json.Marshal(req.Variables)
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("marshal variables: %v", err))
	}
	scene.Variables = string(varsJSON)

	if err := h.scenes.Update(r.Context(), scene); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update scene variables: %v", err))
	}

	return dto.OK(dto.BatchSetVariablesResponse{Variables: req.Variables})
}

// --- DataSource Handlers ---

func (h *Handler) UploadDataSource(r *http.Request) dto.Response {
	req, err := decode[dto.UploadDataSourceRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}
	if req.FileName == "" {
		return dto.ErrorResp(400, "file_name is required")
	}
	if req.Content == "" {
		return dto.ErrorResp(400, "content is required")
	}

	// Verify scene exists
	if _, err := h.scenes.GetByID(r.Context(), req.SceneID); err == sql.ErrNoRows {
		return dto.ErrorResp(404, "scene not found")
	} else if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get scene: %v", err))
	}

	// Parse CSV (empty rows are automatically removed)
	columns, rows, removedEmptyRows, err := runner.ParseCSV(req.FileName, strings.NewReader(req.Content))
	if err != nil {
		return dto.ErrorResp(400, fmt.Sprintf("parse csv: %v", err))
	}

	// Check for existing CSV data source with same name (upsert, YAML data source is preserved)
	dsModel := runner.ToDataSourceModel(req.SceneID, req.FileName, columns, rows)

	existingCSV, _ := h.dataSources.GetBySceneIDAndNameAndSource(r.Context(), req.SceneID, dsModel.Name, "csv")
	if existingCSV != nil {
		_ = h.dataSources.Delete(r.Context(), existingCSV.ID)
	}

	if err := h.dataSources.Create(r.Context(), dsModel); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("create data source: %v", err))
	}

	return dto.OK(dto.DataSourceDTO{
		ID:               dsModel.ID,
		SceneID:          dsModel.SceneID,
		Name:             dsModel.Name,
		FileName:         dsModel.FileName,
		Columns:          columns,
		RowCount:         dsModel.RowCount,
		Source:           dsModel.Source,
		RemovedEmptyRows: removedEmptyRows,
		CreatedAt:        dsModel.CreatedAt,
		UpdatedAt:        dsModel.UpdatedAt,
	})
}

func (h *Handler) ListDataSources(r *http.Request) dto.Response {
	req, err := decode[dto.SceneIDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.SceneID == 0 {
		return dto.ErrorResp(400, "scene_id is required")
	}

	sources, err := h.dataSources.ListBySceneID(r.Context(), req.SceneID)
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list data sources: %v", err))
	}

	var items []dto.DataSourceDTO
	for _, ds := range sources {
		var columns []string
		if err := json.Unmarshal([]byte(ds.Columns), &columns); err != nil {
			// Legacy format: comma-separated string (from older YAML imports)
			if ds.Columns != "" {
				columns = strings.Split(ds.Columns, ",")
			}
		}
		items = append(items, dto.DataSourceDTO{
			ID:        ds.ID,
			SceneID:   ds.SceneID,
			Name:      ds.Name,
			FileName:  ds.FileName,
			Columns:   columns,
			RowCount:  ds.RowCount,
			Source:    ds.Source,
			CreatedAt: ds.CreatedAt,
			UpdatedAt: ds.UpdatedAt,
		})
	}
	return dto.OK(items)
}

func (h *Handler) PreviewDataSource(r *http.Request) dto.Response {
	req, err := decode[dto.IDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	ds, err := h.dataSources.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "data source not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get data source: %v", err))
	}

	var columns []string
	if err := json.Unmarshal([]byte(ds.Columns), &columns); err != nil {
		// Legacy format: comma-separated string (from older YAML imports)
		if ds.Columns != "" {
			columns = strings.Split(ds.Columns, ",")
		}
	}
	var rows []map[string]string
	_ = json.Unmarshal([]byte(ds.Rows), &rows)

	// Limit preview to first 10 rows
	previewRows := rows
	if len(rows) > 10 {
		previewRows = rows[:10]
	}

	return dto.OK(dto.DataSourcePreviewDTO{
		ID:        ds.ID,
		SceneID:   ds.SceneID,
		Name:      ds.Name,
		FileName:  ds.FileName,
		Columns:   columns,
		RowCount:  ds.RowCount,
		Rows:      previewRows,
		CreatedAt: ds.CreatedAt,
		UpdatedAt: ds.UpdatedAt,
	})
}

func (h *Handler) UpdateDataSource(r *http.Request) dto.Response {
	req, err := decode[dto.UpdateDataSourceRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}
	if req.Content == "" {
		return dto.ErrorResp(400, "content is required")
	}

	existing, err := h.dataSources.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "data source not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get data source: %v", err))
	}

	// Parse the updated CSV content
	columns, rows, removedEmptyRows, err := runner.ParseCSV(existing.FileName, strings.NewReader(req.Content))
	if err != nil {
		return dto.ErrorResp(400, fmt.Sprintf("parse csv: %v", err))
	}

	// Update the existing record in-place, preserving its source (yaml/csv)
	columnsJSON, _ := json.Marshal(columns)
	rowsJSON, _ := json.Marshal(rows)
	existing.Columns = string(columnsJSON)
	existing.Rows = string(rowsJSON)
	existing.RowCount = len(rows)

	if err := h.dataSources.Update(r.Context(), existing); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update data source: %v", err))
	}

	return dto.OK(dto.DataSourceDTO{
		ID:               existing.ID,
		SceneID:          existing.SceneID,
		Name:             existing.Name,
		FileName:         existing.FileName,
		Columns:          columns,
		RowCount:         existing.RowCount,
		Source:           existing.Source,
		RemovedEmptyRows: removedEmptyRows,
		CreatedAt:        existing.CreatedAt,
		UpdatedAt:        existing.UpdatedAt,
	})
}

func (h *Handler) DeleteDataSource(r *http.Request) dto.Response {
	req, err := decode[dto.IDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	if err := h.dataSources.Delete(r.Context(), req.ID); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("delete data source: %v", err))
	}
	return dto.OK(nil)
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

func (h *Handler) ListGenerators(r *http.Request) dto.Response {
	catalog := builtin.Catalog()
	return dto.OK(map[string]any{
		"categories": catalog,
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

	items := make([]dto.ReportListItemDTO, 0, len(reports))
	for _, rp := range reports {
		items = append(items, toReportListItemDTO(rp))
	}

	return dto.OK(dto.ListResponse[[]dto.ReportListItemDTO]{
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

func (h *Handler) ExportReport(w http.ResponseWriter, r *http.Request) {
	reportIDStr := r.PathValue("id")
	if reportIDStr == "" {
		http.Error(w, "report id is required", http.StatusBadRequest)
		return
	}

	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid report id", http.StatusBadRequest)
		return
	}

	report, err := h.reports.GetByID(r.Context(), snowflake.ID(reportID))
	if err == sql.ErrNoRows {
		http.Error(w, "report not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.log.Error("failed to get report", logger.F("report_id", reportID), logger.F("error", err))
		http.Error(w, fmt.Sprintf("get report: %v", err), http.StatusInternalServerError)
		return
	}

	if report.Detail == "" || report.Detail == "{}" {
		http.Error(w, "report detail not available", http.StatusNotFound)
		return
	}

	htmlContent, err := GenerateEnhancedHTML(report.Detail)
	if err != nil {
		h.log.Error("failed to generate enhanced HTML", logger.F("report_id", reportID), logger.F("error", err))
		http.Error(w, fmt.Sprintf("generate report: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.html", generateReportFilename(report.SceneID)))
	w.Write([]byte(htmlContent))
}

func (h *Handler) BatchExportReports(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReportIDs []int64 `json:"report_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.ReportIDs) == 0 {
		http.Error(w, "report_ids is required", http.StatusBadRequest)
		return
	}

	if len(req.ReportIDs) > 50 {
		http.Error(w, "too many reports (max 50)", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=reports.zip")

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, reportID := range req.ReportIDs {
		report, err := h.reports.GetByID(r.Context(), snowflake.ID(reportID))
		if err != nil {
			continue
		}

		if report.Detail == "" || report.Detail == "{}" {
			continue
		}

		htmlContent, err := GenerateEnhancedHTML(report.Detail)
		if err != nil {
			continue
		}

		filename := fmt.Sprintf("%s.html", generateReportFilename(report.SceneID))
		f, err := zw.Create(filename)
		if err != nil {
			continue
		}

		f.Write([]byte(htmlContent))
	}
}

func generateReportFilename(sceneID snowflake.ID) string {
	sceneStr := sceneID.String()
	last8 := ""
	if len(sceneStr) > 8 {
		last8 = sceneStr[len(sceneStr)-8:]
	} else {
		last8 = sceneStr
	}
	now := time.Now()
	return fmt.Sprintf("salvo-report-%s-%s-%s", last8, now.Format("20060102"), now.Format("150405"))
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
	runningMap := make(map[string]*runner.Runner)
	if rm := h.runnerMgr.List(); len(rm) > 0 {
		for sceneID, rn := range rm {
			runningMap[sceneID.String()] = rn
		}
	}
	for _, rr := range runs {
		dtoRR := toRunRecordDTO(rr)
		if rr.Status == "running" {
			if rn, ok := runningMap[rr.SceneID.String()]; ok {
				st := rn.Stats()
				dtoRR.TotalReqs = st.TotalReqs.Load()
				dtoRR.SuccessReqs = st.SuccessReqs.Load()
				dtoRR.FailedReqs = st.FailedReqs.Load()
				if dtoRR.TotalReqs > 0 {
					avg, p50, p90, p95, p99 := st.LatencyPercentiles()
					dtoRR.AvgLatency = avg.Seconds()
					dtoRR.P50Latency = p50.Seconds()
					dtoRR.P90Latency = p90.Seconds()
					dtoRR.P95Latency = p95.Seconds()
					dtoRR.P99Latency = p99.Seconds()
				}
			}
		}
		items = append(items, dtoRR)
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
		ID:             s.ID,
		Name:           s.Name,
		Description:    s.Description,
		DAGJSON:        s.DAGJSON,
		Variables:      s.Variables,
		ConfigParams:   s.ConfigParams,
		DerivedParams:  s.DerivedParams,
		Plugins:        s.Plugins,
		Status:         s.Status,
		DefaultTimeout: s.DefaultTimeout,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

func toNodeDTO(n *model.Node) dto.NodeDTO {
	return dto.NodeDTO{
		ID:           n.ID,
		SceneID:      n.SceneID,
		Name:         n.Name,
		Type:         n.Type,
		Config:       n.Config,
		Position:     n.Position,
		LoopCount:    n.LoopCount,
		BlockOnError: n.BlockOnError,
		Lifecycle:    n.Lifecycle,
		CreatedAt:    n.CreatedAt,
		UpdatedAt:    n.UpdatedAt,
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

func toReportListItemDTO(rp *model.Report) dto.ReportListItemDTO {
	return dto.ReportListItemDTO{
		ID:         rp.ID,
		SceneID:    rp.SceneID,
		RunID:      rp.RunID,
		Status:     rp.Status,
		Summary:    rp.Summary,
		StartedAt:  rp.StartedAt,
		FinishedAt: rp.FinishedAt,
		CreatedAt:  rp.CreatedAt,
		UpdatedAt:  rp.UpdatedAt,
	}
}

func toRunRecordDTO(rr *model.RunRecord) dto.RunRecordDTO {
	return dto.RunRecordDTO{
		ID:          rr.ID,
		RunID:       rr.RunID,
		SceneID:     rr.SceneID,
		Status:      rr.Status,
		WorkerCount: rr.WorkerCount,
		RunMode:     rr.RunMode,
		Duration:    rr.Duration,
		Count:       rr.Count,
		TotalReqs:   rr.TotalReqs,
		SuccessReqs: rr.SuccessReqs,
		FailedReqs:  rr.FailedReqs,
		AvgLatency:  rr.AvgLatency,
		P50Latency:  rr.P50Latency,
		P90Latency:  rr.P90Latency,
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
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	var traces []*tracelib.Trace
	if req.SceneID != 0 {
		traces = h.tracer.ListByScene(req.SceneID, limit, offset)
	} else {
		traces = h.tracer.List(limit, offset)
	}

	items := make([]dto.TraceDTO, 0, len(traces))
	for _, tr := range traces {
		sName, nMap := h.resolveTraceNames(r.Context(), tr.SceneID, tr)
		items = append(items, toTraceDTO(tr, sName, nMap))
	}
	return dto.OK(dto.ListResponse[[]dto.TraceDTO]{
		Items: items,
		Pagination: dto.Pagination{
			Total:  h.tracer.Total(),
			Limit:  limit,
			Offset: offset,
		},
	})
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

	sName, nMap := h.resolveTraceNames(r.Context(), tr.SceneID, tr)
	return dto.OK(toTraceDTO(tr, sName, nMap))
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

	sName, nMap := h.resolveTraceNames(r.Context(), tr.SceneID, tr)
	return dto.OK(toTraceDTO(tr, sName, nMap))
}

func (h *Handler) resolveTraceNames(ctx context.Context, sceneID snowflake.ID, tr *tracelib.Trace) (string, map[string]string) {
	sceneName := ""
	if sceneID != 0 {
		if sc, err := h.scenes.GetByID(ctx, sceneID); err == nil && sc != nil {
			sceneName = sc.Name
		}
	}

	nodeNameMap := make(map[string]string)
	for _, sp := range tr.Spans {
		if _, exists := nodeNameMap[sp.NodeID]; exists {
			continue
		}
		var nodeID snowflake.ID
		if parseErr := nodeID.Parse(sp.NodeID); parseErr != nil {
			continue
		}
		if n, err := h.nodes.GetByID(ctx, nodeID); err == nil && n != nil {
			nodeNameMap[sp.NodeID] = n.Name
		}
	}
	return sceneName, nodeNameMap
}

func toTraceDTO(tr *tracelib.Trace, sceneName string, nodeNameMap map[string]string) dto.TraceDTO {
	spans := make([]dto.SpanDTO, 0, len(tr.Spans))
	for _, sp := range tr.Spans {
		spans = append(spans, dto.SpanDTO{
			ID:           sp.ID,
			TraceID:      sp.TraceID,
			ChainID:      sp.ChainID,
			NodeID:       sp.NodeID,
			NodeName:     nodeNameMap[sp.NodeID],
			ParentNodeID: sp.ParentNodeID,
			Status:       string(sp.Status),
			Error:        sp.Error,
			Input:        sp.Input,
			Output:       sp.Output,
			StartedAt:    sp.StartedAt,
			FinishedAt:   sp.FinishedAt,
			Duration:     sp.Duration.Nanoseconds(),
		})
	}

	return dto.TraceDTO{
		ID:         tr.ID,
		SceneID:    tr.SceneID,
		SceneName:  sceneName,
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

	// Merge global variables with request variables
	// Request variables override global variables
	mergedVars := make(map[string]string)
	if h.globalVars != nil {
		for k, v := range h.globalVars {
			mergedVars[k] = v
		}
	}
	if req.Variables != nil {
		for k, v := range req.Variables {
			mergedVars[k] = v
		}
	}

	cfg := runner.Config{
		SceneID:             req.SceneID,
		Workers:             workers,
		RunMode:             runMode,
		Count:               req.Count,
		Variables:           mergedVars,
		EnableSystemMetrics: true,
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
		h.log.Error("failed to start scene",
			logger.F("scene_id", req.SceneID.String()),
			logger.F("workers", workers),
			logger.F("run_mode", runMode),
			logger.F("error", err),
		)
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
		rangeSeconds = 3600
	}

	var sceneID int64
	if req.SceneID != "" {
		sceneID, err = strconv.ParseInt(req.SceneID, 10, 64)
		if err != nil {
			h.log.Error("Failed to parse SceneID",
				logger.F("scene_id", req.SceneID),
				logger.F("error", err))
			return dto.ErrorResp(400, "invalid scene_id")
		}
		h.log.Debug("DashboardOverview with scene",
			logger.F("scene_id", sceneID))
	}

	filter := repo.Filter{Limit: 50}
	if sceneID > 0 {
		filter.SceneID = snowflake.ID(sceneID)
	}

	runRecords, err := h.runs.List(r.Context(), filter)
	if err != nil {
		return dto.ErrorResp(500, "failed to list runs")
	}

	if len(runRecords) == 0 && sceneID > 0 {
		h.log.Warn("no run records found for scene in DashboardOverview",
			logger.F("scene_id", sceneID))
	}

	if sceneID > 0 && len(runRecords) > 0 {
		var earliest, latest time.Time
		for _, rr := range runRecords {
			if rr.StartedAt != nil && (earliest.IsZero() || rr.StartedAt.Before(earliest)) {
				earliest = *rr.StartedAt
			}
			if rr.FinishedAt != nil && (latest.IsZero() || rr.FinishedAt.After(latest)) {
				latest = *rr.FinishedAt
			}
		}

		if !earliest.IsZero() {
			now := time.Now()
			var endTime time.Time
			if !latest.IsZero() && latest.After(now) {
				endTime = latest
			} else {
				endTime = now
			}
			timeSinceEarliest := int(endTime.Sub(earliest).Seconds())
			sceneRange := timeSinceEarliest + 300
			if sceneRange > rangeSeconds {
				rangeSeconds = sceneRange
			}
		}
	}

	runningMap := make(map[string]*runner.Runner)
	if rm := h.runnerMgr.List(); len(rm) > 0 {
		for sceneID, rn := range rm {
			runningMap[sceneID.String()] = rn
		}
	}

	h.log.Debug("DashboardOverview debug",
		logger.F("scene_id", sceneID),
		logger.F("run_records_count", len(runRecords)),
		logger.F("running_map_count", len(runningMap)))

	var totalReqs, successReqs, failedReqs int64
	var p50, p95, p99, avg float64
	var allLatencies []float64
	running := 0

	var runningDuration float64
	var startedAt, finishedAt *time.Time

	var recentRuns []dto.RunRecordDTO
	var seriesRuns []dto.RunRecordDTO

	cutoff := time.Now().Add(-time.Duration(rangeSeconds) * time.Second)

	for _, rr := range runRecords {
		dtoRR := toRunRecordDTO(rr)

		if rr.Status == "running" {
			if rn, ok := runningMap[rr.SceneID.String()]; ok {
				rnStatus := rn.Status()
				st := rn.Stats()
				liveTotal := st.TotalReqs.Load()
				liveSuccess := st.SuccessReqs.Load()
				liveFailed := st.FailedReqs.Load()

				// Only use live stats when the runner is still actively running.
				// When the runner has stopped (done/canceled/failed), fall through
				// to use the stored run_record values to avoid latency fluctuations
				// from in-flight requests completing after stop.
				if rnStatus == "running" {
					running++

					totalReqs += liveTotal
					successReqs += liveSuccess
					failedReqs += liveFailed

					dtoRR.TotalReqs = liveTotal
					dtoRR.SuccessReqs = liveSuccess
					dtoRR.FailedReqs = liveFailed
					currentDuration := rn.Duration().Seconds()
					dtoRR.Duration = currentDuration

					if currentDuration > runningDuration {
						runningDuration = currentDuration
					}

					if rr.StartedAt != nil {
						t := rr.StartedAt
						if startedAt == nil || t.Before(*startedAt) {
							startedAt = t
						}
					}

					if liveTotal > 0 {
						lavg, lp50, lp90, lp95, lp99 := st.LatencyPercentiles()
						dtoRR.AvgLatency = lavg.Seconds()
						dtoRR.P50Latency = lp50.Seconds()
						dtoRR.P90Latency = lp90.Seconds()
						dtoRR.P95Latency = lp95.Seconds()
						dtoRR.P99Latency = lp99.Seconds()

						rawLatencies := st.GetAllLatencies()
						for _, lat := range rawLatencies {
							allLatencies = append(allLatencies, float64(lat.Microseconds()))
						}
					}
				} else {
					totalReqs += rr.TotalReqs
					successReqs += rr.SuccessReqs
					failedReqs += rr.FailedReqs

					dtoRR.TotalReqs = rr.TotalReqs
					dtoRR.SuccessReqs = rr.SuccessReqs
					dtoRR.FailedReqs = rr.FailedReqs

					dtoRR.AvgLatency = rr.AvgLatency
					dtoRR.P50Latency = rr.P50Latency
					dtoRR.P90Latency = rr.P90Latency
					dtoRR.P95Latency = rr.P95Latency
					dtoRR.P99Latency = rr.P99Latency

					allLatencies = append(allLatencies, rr.P50Latency*1e6)
					allLatencies = append(allLatencies, rr.P90Latency*1e6)
					allLatencies = append(allLatencies, rr.P99Latency*1e6)

					if rr.StartedAt != nil {
						t := rr.StartedAt
						if startedAt == nil || t.Before(*startedAt) {
							startedAt = t
						}
					}
					if rr.FinishedAt != nil {
						t := rr.FinishedAt
						if finishedAt == nil || t.After(*finishedAt) {
							finishedAt = t
						}
					}
				}
			} else {
				totalReqs += rr.TotalReqs
				successReqs += rr.SuccessReqs
				failedReqs += rr.FailedReqs

				dtoRR.AvgLatency = rr.AvgLatency
				dtoRR.P50Latency = rr.P50Latency
				dtoRR.P90Latency = rr.P90Latency
				dtoRR.P95Latency = rr.P95Latency
				dtoRR.P99Latency = rr.P99Latency

				allLatencies = append(allLatencies, rr.P50Latency*1e6)
				allLatencies = append(allLatencies, rr.P90Latency*1e6)
				allLatencies = append(allLatencies, rr.P99Latency*1e6)

				if rr.StartedAt != nil {
					t := rr.StartedAt
					if startedAt == nil || t.Before(*startedAt) {
						startedAt = t
					}
				}
				if rr.FinishedAt != nil {
					t := rr.FinishedAt
					if finishedAt == nil || t.After(*finishedAt) {
						finishedAt = t
					}
				}
			}
		} else {
			totalReqs += rr.TotalReqs
			successReqs += rr.SuccessReqs
			failedReqs += rr.FailedReqs

			dtoRR.AvgLatency = rr.AvgLatency
			dtoRR.P50Latency = rr.P50Latency
			dtoRR.P90Latency = rr.P90Latency
			dtoRR.P95Latency = rr.P95Latency
			dtoRR.P99Latency = rr.P99Latency

			allLatencies = append(allLatencies, rr.P50Latency*1e6)
			allLatencies = append(allLatencies, rr.P90Latency*1e6)
			allLatencies = append(allLatencies, rr.P95Latency*1e6)
			allLatencies = append(allLatencies, rr.P99Latency*1e6)

			if rr.StartedAt != nil {
				t := rr.StartedAt
				if startedAt == nil || t.Before(*startedAt) {
					startedAt = t
				}
			}
			if rr.FinishedAt != nil {
				t := rr.FinishedAt
				if finishedAt == nil || t.After(*finishedAt) {
					finishedAt = t
				}
			}
		}

		recentRuns = append(recentRuns, dtoRR)
		if rr.StartedAt != nil && rr.StartedAt.After(cutoff) {
			seriesRuns = append(seriesRuns, dtoRR)
		} else if rr.FinishedAt != nil && rr.FinishedAt.After(cutoff) {
			seriesRuns = append(seriesRuns, dtoRR)
		}
	}

	if len(allLatencies) > 0 {
		sort.Float64s(allLatencies)
		n := len(allLatencies)

		var totalMicros float64
		for _, lat := range allLatencies {
			totalMicros += lat
		}
		avg = totalMicros / float64(n) / 1e6

		p50 = allLatencies[n*50/100] / 1e6
		p95 = allLatencies[n*95/100] / 1e6
		p99 = allLatencies[n*99/100] / 1e6
	}

	if len(recentRuns) > 10 {
		recentRuns = recentRuns[:10]
	}

	nodeMetrics := h.aggregateNodeMetricsWithSceneID(r, sceneID)

	var ts *dto.TimeSeriesDTO
	if len(seriesRuns) > 0 {
		ts = h.buildTimeSeriesWithDB(r.Context(), seriesRuns, rangeSeconds)
	} else if len(recentRuns) > 0 && running == 0 {
		ts = h.buildTimeSeriesWithDB(r.Context(), recentRuns, rangeSeconds)
	}

	response := dto.DashboardOverviewDTO{
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
	}

	if sceneID > 0 {
		response.SceneID = sceneID
	}

	if sceneID > 0 {
		if rn, ok := runningMap[strconv.FormatInt(sceneID, 10)]; ok {
			if httpOnlySamples := rn.HttpOnlyGlobalTimeSeries(); len(httpOnlySamples) > 0 {
				response.HttpOnlyTimeSeries = h.samplesToTimeSeriesDTO(httpOnlySamples)
			}
		}
	}
	// If the scene is not running, fall back to the latest completed report.
	if sceneID > 0 {
		if rn, ok := runningMap[strconv.FormatInt(sceneID, 10)]; ok {
			if snapshots := rn.RuntimeMetricsSnapshots(); len(snapshots) > 0 {
				last := snapshots[len(snapshots)-1]
				response.SystemMetrics = &dto.RuntimeMetricsDTO{
					GoroutineCount:  last.GoroutineCount,
					HeapAllocMB:     last.HeapAllocMB,
					HeapSysMB:       last.HeapSysMB,
					CPUUsagePercent: last.CPUUsagePercent,
					RSSMemoryMB:     last.RSSMemoryMB,
					ActiveWorkers:   last.ActiveWorkers,
					PendingQueueLen: last.PendingQueueLen,
					TaskWaitP50Ms:   last.TaskWaitP50Ms,
					TaskWaitP95Ms:   last.TaskWaitP95Ms,
					TaskWaitP99Ms:   last.TaskWaitP99Ms,
					GCPauseLastMs:   float64(last.GCPauseLastNs) / 1e6,
				}
			}
		} else {
			response.SystemMetrics, response.SystemMetricsTimeSeries = h.loadSystemMetricsFromDB(r.Context(), snowflake.ID(sceneID))
		}
	} else if len(runningMap) > 0 {
		for _, rn := range runningMap {
			if snapshots := rn.RuntimeMetricsSnapshots(); len(snapshots) > 0 {
				last := snapshots[len(snapshots)-1]
				response.SystemMetrics = &dto.RuntimeMetricsDTO{
					GoroutineCount:  last.GoroutineCount,
					HeapAllocMB:     last.HeapAllocMB,
					HeapSysMB:       last.HeapSysMB,
					CPUUsagePercent: last.CPUUsagePercent,
					RSSMemoryMB:     last.RSSMemoryMB,
					ActiveWorkers:   last.ActiveWorkers,
					PendingQueueLen: last.PendingQueueLen,
					TaskWaitP50Ms:   last.TaskWaitP50Ms,
					TaskWaitP95Ms:   last.TaskWaitP95Ms,
					TaskWaitP99Ms:   last.TaskWaitP99Ms,
					GCPauseLastMs:   float64(last.GCPauseLastNs) / 1e6,
				}
			}
			break
		}
	}

	return dto.OK(response)
}

func (h *Handler) loadSystemMetricsFromDB(ctx context.Context, sceneID snowflake.ID) (*dto.RuntimeMetricsDTO, []dto.RuntimeMetricsDTO) {
	filter := repo.Filter{SceneID: sceneID, Limit: 1}
	runRecords, err := h.runs.List(ctx, filter)
	if err != nil || len(runRecords) == 0 {
		return nil, nil
	}
	latestRR := runRecords[0]
	if latestRR.ID == 0 {
		return nil, nil
	}
	report, err := h.reports.GetByRunID(ctx, latestRR.ID)
	if err != nil || report == nil || report.Detail == "" || report.Detail == "{}" {
		return nil, nil
	}
	var detail struct {
		SystemMetrics *struct {
			TimeSeries []struct {
				Timestamp       string  `json:"timestamp"`
				GoroutineCount  int64   `json:"goroutine_count"`
				HeapAllocMB     float64 `json:"heap_alloc_mb"`
				HeapSysMB       float64 `json:"heap_sys_mb"`
				CPUUsagePercent float64 `json:"cpu_percent"`
				RSSMemoryMB     float64 `json:"rss_mb"`
				ActiveWorkers   int     `json:"active_workers"`
				PendingQueueLen int     `json:"pending_queue_len"`
				TaskWaitP50Ms   float64 `json:"task_wait_p50_ms"`
				TaskWaitP95Ms   float64 `json:"task_wait_p95_ms"`
				TaskWaitP99Ms   float64 `json:"task_wait_p99_ms"`
				GCPauseLastNs   uint64  `json:"gc_pause_last_ns"`
			} `json:"time_series,omitempty"`
			Summary struct {
				GoroutineMax     float64 `json:"goroutine_max"`
				GoroutineAvg     float64 `json:"goroutine_avg"`
				HeapAllocMaxMB   float64 `json:"heap_alloc_max_mb"`
				HeapAllocAvgMB   float64 `json:"heap_alloc_avg_mb"`
				CPUMax           float64 `json:"cpu_max"`
				CPUAvg           float64 `json:"cpu_avg"`
				TaskWaitAvgMs    float64 `json:"task_wait_avg_ms"`
				TaskWaitP50MaxMs float64 `json:"task_wait_p50_max_ms"`
				TaskWaitP95MaxMs float64 `json:"task_wait_p95_max_ms"`
				TaskWaitP99MaxMs float64 `json:"task_wait_p99_max_ms"`
				GCPauseTotalMs   float64 `json:"gc_pause_total_ms"`
				PendingQueueMax  float64 `json:"pending_queue_max"`
				PendingQueueAvg  float64 `json:"pending_queue_avg"`
				ActiveWorkersMax float64 `json:"active_workers_max"`
				ActiveWorkersAvg float64 `json:"active_workers_avg"`
			} `json:"summary"`
		} `json:"system_metrics,omitempty"`
	}
	if err := json.Unmarshal([]byte(report.Detail), &detail); err != nil {
		return nil, nil
	}
	sm := detail.SystemMetrics
	if sm == nil {
		return nil, nil
	}

	snapshot := &dto.RuntimeMetricsDTO{
		GoroutineCount:  int64(sm.Summary.GoroutineMax),
		HeapAllocMB:     sm.Summary.HeapAllocMaxMB,
		HeapSysMB:       0,
		CPUUsagePercent: sm.Summary.CPUMax,
		RSSMemoryMB:     0,
		ActiveWorkers:   int(sm.Summary.ActiveWorkersMax),
		PendingQueueLen: int(sm.Summary.PendingQueueMax),
		TaskWaitP50Ms:   sm.Summary.TaskWaitP50MaxMs,
		TaskWaitP95Ms:   sm.Summary.TaskWaitP95MaxMs,
		TaskWaitP99Ms:   sm.Summary.TaskWaitP99MaxMs,
		GCPauseLastMs:   sm.Summary.GCPauseTotalMs,
	}

	var timeSeries []dto.RuntimeMetricsDTO
	for _, ts := range sm.TimeSeries {
		timeSeries = append(timeSeries, dto.RuntimeMetricsDTO{
			Timestamp:       ts.Timestamp,
			GoroutineCount:  ts.GoroutineCount,
			HeapAllocMB:     ts.HeapAllocMB,
			HeapSysMB:       ts.HeapSysMB,
			CPUUsagePercent: ts.CPUUsagePercent,
			RSSMemoryMB:     ts.RSSMemoryMB,
			ActiveWorkers:   ts.ActiveWorkers,
			PendingQueueLen: ts.PendingQueueLen,
			TaskWaitP50Ms:   ts.TaskWaitP50Ms,
			TaskWaitP95Ms:   ts.TaskWaitP95Ms,
			TaskWaitP99Ms:   ts.TaskWaitP99Ms,
			GCPauseLastMs:   float64(ts.GCPauseLastNs) / 1e6,
		})
	}

	return snapshot, timeSeries
}

func (h *Handler) DashboardHistory(r *http.Request) dto.Response {
	req, err := decode[dto.DashboardHistoryRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	filter := repo.Filter{Limit: limit}
	if req.SceneID > 0 {
		filter.SceneID = snowflake.ID(req.SceneID)
	}

	runRecords, err := h.runs.List(r.Context(), filter)
	if err != nil {
		return dto.ErrorResp(500, "failed to list runs")
	}

	var history []dto.RunHistoryDTO
	for _, rr := range runRecords {
		timeSeriesData, tsErr := h.tsStore.QueryByRunID(r.Context(), rr.ID)

		globalSamples := make([]dto.TimeSeriesSampleDTO, 0)
		nodeSamplesMap := make(map[string][]dto.TimeSeriesSampleDTO)

		if tsErr == nil && len(timeSeriesData) > 0 {
			for _, record := range timeSeriesData {
				sample := dto.TimeSeriesSampleDTO{
					Timestamp:     record.SampleTime.Unix(),
					QPS:           record.QPS,
					TotalRequests: record.TotalRequests,
					SuccessCount:  record.SuccessCount,
					FailCount:     record.FailCount,
					AvgLatencyMs:  record.AvgLatencyMs,
					P50LatencyMs:  record.P50LatencyMs,
					P95LatencyMs:  record.P95LatencyMs,
					P99LatencyMs:  record.P99LatencyMs,
				}
				if record.NodeID == "" {
					globalSamples = append(globalSamples, sample)
				} else {
					nodeSamplesMap[record.NodeID] = append(nodeSamplesMap[record.NodeID], sample)
				}
			}
		} else {
			report, rptErr := h.reports.GetByRunID(r.Context(), rr.ID)
			if rptErr == nil && report != nil && len(report.Detail) > 0 {
				globalSamples, nodeSamplesMap = h.extractTimeSeriesFromReportDetail(report.Detail)
			}
		}

		history = append(history, dto.RunHistoryDTO{
			RunID:         rr.ID,
			SceneID:       rr.SceneID,
			Status:        rr.Status,
			StartedAt:     rr.StartedAt,
			FinishedAt:    rr.FinishedAt,
			TotalReqs:     rr.TotalReqs,
			SuccessReqs:   rr.SuccessReqs,
			FailedReqs:    rr.FailedReqs,
			AvgLatency:    rr.AvgLatency,
			P50Latency:    rr.P50Latency,
			P95Latency:    rr.P95Latency,
			P99Latency:    rr.P99Latency,
			GlobalSamples: globalSamples,
			NodeSamples:   nodeSamplesMap,
		})
	}

	return dto.OK(dto.DashboardHistoryDTO{
		History: history,
		Total:   len(history),
	})
}

func (h *Handler) extractTimeSeriesFromReportDetail(detailJSON string) ([]dto.TimeSeriesSampleDTO, map[string][]dto.TimeSeriesSampleDTO) {
	globalSamples := make([]dto.TimeSeriesSampleDTO, 0)
	nodeSamplesMap := make(map[string][]dto.TimeSeriesSampleDTO)

	var detail struct {
		GlobalTimeSeries []struct {
			Timestamp     time.Time `json:"t"`
			QPS           float64   `json:"qps"`
			TotalRequests int64     `json:"total"`
			SuccessCount  int64     `json:"success"`
			FailCount     int64     `json:"fail"`
			AvgLatencyMs  float64   `json:"avg_ms"`
			P50LatencyMs  float64   `json:"p50_ms"`
			P90LatencyMs  float64   `json:"p90_ms"`
			P95LatencyMs  float64   `json:"p95_ms"`
			P99LatencyMs  float64   `json:"p99_ms"`
		} `json:"global_time_series"`
		NodeMetrics []struct {
			NodeID     string `json:"node_id"`
			TimeSeries []struct {
				Timestamp     time.Time `json:"t"`
				QPS           float64   `json:"qps"`
				TotalRequests int64     `json:"total"`
				SuccessCount  int64     `json:"success"`
				FailCount     int64     `json:"fail"`
				AvgLatencyMs  float64   `json:"avg_ms"`
				P50LatencyMs  float64   `json:"p50_ms"`
				P90LatencyMs  float64   `json:"p90_ms"`
				P95LatencyMs  float64   `json:"p95_ms"`
				P99LatencyMs  float64   `json:"p99_ms"`
			} `json:"time_series"`
		} `json:"node_metrics"`
	}

	if err := json.Unmarshal([]byte(detailJSON), &detail); err != nil {
		return globalSamples, nodeSamplesMap
	}

	for _, ts := range detail.GlobalTimeSeries {
		globalSamples = append(globalSamples, dto.TimeSeriesSampleDTO{
			Timestamp:     ts.Timestamp.Unix(),
			QPS:           ts.QPS,
			TotalRequests: ts.TotalRequests,
			SuccessCount:  ts.SuccessCount,
			FailCount:     ts.FailCount,
			AvgLatencyMs:  ts.AvgLatencyMs,
			P50LatencyMs:  ts.P50LatencyMs,
			P95LatencyMs:  ts.P95LatencyMs,
			P99LatencyMs:  ts.P99LatencyMs,
		})
	}

	for _, node := range detail.NodeMetrics {
		for _, ts := range node.TimeSeries {
			nodeSamplesMap[node.NodeID] = append(nodeSamplesMap[node.NodeID], dto.TimeSeriesSampleDTO{
				Timestamp:     ts.Timestamp.Unix(),
				QPS:           ts.QPS,
				TotalRequests: ts.TotalRequests,
				SuccessCount:  ts.SuccessCount,
				FailCount:     ts.FailCount,
				AvgLatencyMs:  ts.AvgLatencyMs,
				P50LatencyMs:  ts.P50LatencyMs,
				P95LatencyMs:  ts.P95LatencyMs,
				P99LatencyMs:  ts.P99LatencyMs,
			})
		}
	}

	return globalSamples, nodeSamplesMap
}

func (h *Handler) aggregateNodeMetrics(r *http.Request) []dto.NodeMetricDTO {
	traces := h.tracer.List(10000, 0)
	if len(traces) == 0 {
		return nil
	}

	type spanEntry struct {
		startedAt time.Time
		duration  float64
		status    string
	}

	nodeSpans := make(map[string][]spanEntry)
	var nodeIDs []string
	var latestSceneID string
	var globalStart, globalEnd time.Time

	for _, tr := range traces {
		if latestSceneID == "" {
			latestSceneID = tr.SceneID.String()
		}
		for _, sp := range tr.Spans {
			key := sp.NodeID
			if _, ok := nodeSpans[key]; !ok {
				nodeIDs = append(nodeIDs, key)
				nodeSpans[key] = make([]spanEntry, 0)
			}
			durSec := sp.Duration.Seconds()
			nodeSpans[key] = append(nodeSpans[key], spanEntry{
				startedAt: sp.StartedAt,
				duration:  durSec,
				status:    string(sp.Status),
			})
			if globalStart.IsZero() || sp.StartedAt.Before(globalStart) {
				globalStart = sp.StartedAt
			}
			finished := sp.StartedAt.Add(sp.Duration)
			if globalEnd.IsZero() || finished.After(globalEnd) {
				globalEnd = finished
			}
		}
	}

	orderMap := h.computeNodeOrder(r, latestSceneID, nodeIDs)

	nodesByName := make(map[string]string)
	nodesByType := make(map[string]string)
	if nodeList, err := h.nodes.List(r.Context(), repo.Filter{Limit: 200}); err == nil {
		for _, n := range nodeList {
			nodesByName[n.ID.String()] = n.Name
			nodesByType[n.ID.String()] = n.Type
		}
	}

	numBuckets := 30
	totalDur := globalEnd.Sub(globalStart)
	if totalDur <= 0 {
		totalDur = time.Minute
		globalStart = time.Now().Add(-time.Minute)
		globalEnd = time.Now()
	}
	bucketSize := totalDur / time.Duration(numBuckets)

	result := make([]dto.NodeMetricDTO, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		spans := nodeSpans[nodeID]
		nm := dto.NodeMetricDTO{
			NodeID:    nodeID,
			Name:      nodesByName[nodeID],
			Type:      nodesByType[nodeID],
			SortOrder: orderMap[nodeID],
		}
		if nm.Name == "" {
			nm.Name = nodeID
		}
		if nm.Type == "" {
			nm.Type = "http"
		}

		var allDurs []float64
		for _, s := range spans {
			// Skip spans are not real requests — they represent nodes that
			// were never executed (e.g. downstream of a failed parent).
			// Exclude them from totals, latency, and QPS calculations.
			if s.status == "skip" {
				continue
			}
			allDurs = append(allDurs, s.duration)
			nm.TotalReqs++
			if s.status == "ok" {
				nm.SuccessReqs++
			}
		}

		if len(allDurs) > 0 {
			sort.Float64s(allDurs)
			nm.AvgLatency = avgFloat64(allDurs)
			nm.P50Latency = percentileFloat64(allDurs, 50)
			nm.P95Latency = percentileFloat64(allDurs, 95)
			nm.P99Latency = percentileFloat64(allDurs, 99)

			timestamps := make([]string, numBuckets)
			tsP50 := make([]float64, numBuckets)
			tsP95 := make([]float64, numBuckets)
			tsP99 := make([]float64, numBuckets)
			tsAvg := make([]float64, numBuckets)
			tsQPS := make([]float64, numBuckets)

			buckets := make([][]float64, numBuckets)
			bucketCounts := make([]int, numBuckets)
			for i := range buckets {
				buckets[i] = make([]float64, 0)
			}

			for _, s := range spans {
				if s.status == "skip" {
					continue
				}
				idx := int(s.startedAt.Sub(globalStart) / bucketSize)
				if idx < 0 {
					idx = 0
				}
				if idx >= numBuckets {
					idx = numBuckets - 1
				}
				buckets[idx] = append(buckets[idx], s.duration)
				bucketCounts[idx]++
			}

			bucketDurSec := totalDur.Seconds() / float64(numBuckets)
			for i := range buckets {
				bucketTime := globalStart.Add(time.Duration(i)*bucketSize + bucketSize/2)
				timestamps[i] = bucketTime.Local().Format("15:04:05")
				if len(buckets[i]) > 0 {
					sort.Float64s(buckets[i])
					tsAvg[i] = avgFloat64(buckets[i])
					tsP50[i] = percentileFloat64(buckets[i], 50)
					tsP95[i] = percentileFloat64(buckets[i], 95)
					tsP99[i] = percentileFloat64(buckets[i], 99)
				}
				if bucketDurSec > 0 {
					tsQPS[i] = float64(bucketCounts[i]) / bucketDurSec
				}
			}

			nm.Timestamps = timestamps
			nm.TSP50 = tsP50
			nm.TSP95 = tsP95
			nm.TSP99 = tsP99
			nm.TSAvg = tsAvg
			nm.TSQPS = tsQPS
		} else {
			timestamps := make([]string, numBuckets)
			for i := 0; i < numBuckets; i++ {
				bucketTime := globalStart.Add(time.Duration(i)*bucketSize + bucketSize/2)
				timestamps[i] = bucketTime.Local().Format("15:04:05")
			}
			nm.Timestamps = timestamps
			nm.TSP50 = make([]float64, numBuckets)
			nm.TSP95 = make([]float64, numBuckets)
			nm.TSP99 = make([]float64, numBuckets)
			nm.TSAvg = make([]float64, numBuckets)
			nm.TSQPS = make([]float64, numBuckets)
		}

		result = append(result, nm)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].SortOrder < result[j].SortOrder })
	return result
}

func (h *Handler) aggregateNodeMetricsWithSceneID(r *http.Request, sceneID int64) []dto.NodeMetricDTO {
	traces := h.tracer.List(10000, 0)
	if len(traces) == 0 {
		return nil
	}

	type spanEntry struct {
		startedAt time.Time
		duration  float64
		status    string
	}

	nodeSpans := make(map[string][]spanEntry)
	var nodeIDs []string
	var targetSceneID string
	var globalStart, globalEnd time.Time

	for _, tr := range traces {
		if sceneID > 0 && tr.SceneID.Int64() != sceneID {
			continue
		}

		if targetSceneID == "" {
			targetSceneID = tr.SceneID.String()
		}
		for _, sp := range tr.Spans {
			key := sp.NodeID
			if _, ok := nodeSpans[key]; !ok {
				nodeIDs = append(nodeIDs, key)
				nodeSpans[key] = make([]spanEntry, 0)
			}
			durSec := sp.Duration.Seconds()
			nodeSpans[key] = append(nodeSpans[key], spanEntry{
				startedAt: sp.StartedAt,
				duration:  durSec,
				status:    string(sp.Status),
			})
			if globalStart.IsZero() || sp.StartedAt.Before(globalStart) {
				globalStart = sp.StartedAt
			}
			finished := sp.StartedAt.Add(sp.Duration)
			if globalEnd.IsZero() || finished.After(globalEnd) {
				globalEnd = finished
			}
		}
	}

	if len(nodeIDs) == 0 {
		return nil
	}

	orderMap := h.computeNodeOrder(r, targetSceneID, nodeIDs)

	nodesByName := make(map[string]string)
	nodesByType := make(map[string]string)
	if nodeList, err := h.nodes.List(r.Context(), repo.Filter{Limit: 200}); err == nil {
		for _, n := range nodeList {
			nodesByName[n.ID.String()] = n.Name
			nodesByType[n.ID.String()] = n.Type
		}
	}

	numBuckets := 30
	totalDur := globalEnd.Sub(globalStart)
	if totalDur <= 0 {
		totalDur = time.Minute
		globalStart = time.Now().Add(-time.Minute)
		globalEnd = time.Now()
	}
	bucketSize := totalDur / time.Duration(numBuckets)

	result := make([]dto.NodeMetricDTO, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		spans := nodeSpans[nodeID]
		nm := dto.NodeMetricDTO{
			NodeID:    nodeID,
			Name:      nodesByName[nodeID],
			Type:      nodesByType[nodeID],
			SortOrder: orderMap[nodeID],
		}
		if nm.Name == "" {
			nm.Name = nodeID
		}
		if nm.Type == "" {
			nm.Type = "http"
		}

		var allDurs []float64
		for _, s := range spans {
			if s.status == "skip" {
				continue
			}
			allDurs = append(allDurs, s.duration)
			nm.TotalReqs++
			if s.status == "ok" {
				nm.SuccessReqs++
			}
		}

		if len(allDurs) > 0 {
			sort.Float64s(allDurs)
			nm.AvgLatency = avgFloat64(allDurs)
			nm.P50Latency = percentileFloat64(allDurs, 50)
			nm.P95Latency = percentileFloat64(allDurs, 95)
			nm.P99Latency = percentileFloat64(allDurs, 99)

			timestamps := make([]string, numBuckets)
			tsP50 := make([]float64, numBuckets)
			tsP95 := make([]float64, numBuckets)
			tsP99 := make([]float64, numBuckets)
			tsAvg := make([]float64, numBuckets)
			tsQPS := make([]float64, numBuckets)

			buckets := make([][]float64, numBuckets)
			bucketCounts := make([]int, numBuckets)
			for i := range buckets {
				buckets[i] = make([]float64, 0)
			}

			for _, s := range spans {
				if s.status == "skip" {
					continue
				}
				idx := int(s.startedAt.Sub(globalStart) / bucketSize)
				if idx < 0 {
					idx = 0
				}
				if idx >= numBuckets {
					idx = numBuckets - 1
				}
				buckets[idx] = append(buckets[idx], s.duration)
				bucketCounts[idx]++
			}

			bucketDurSec := totalDur.Seconds() / float64(numBuckets)
			for i := range buckets {
				bucketTime := globalStart.Add(time.Duration(i)*bucketSize + bucketSize/2)
				timestamps[i] = bucketTime.Local().Format("15:04:05")
				if len(buckets[i]) > 0 {
					sort.Float64s(buckets[i])
					tsAvg[i] = avgFloat64(buckets[i])
					tsP50[i] = percentileFloat64(buckets[i], 50)
					tsP95[i] = percentileFloat64(buckets[i], 95)
					tsP99[i] = percentileFloat64(buckets[i], 99)
				}
				if bucketDurSec > 0 {
					tsQPS[i] = float64(bucketCounts[i]) / bucketDurSec
				}
			}

			nm.Timestamps = timestamps
			nm.TSP50 = tsP50
			nm.TSP95 = tsP95
			nm.TSP99 = tsP99
			nm.TSAvg = tsAvg
			nm.TSQPS = tsQPS
		} else {
			timestamps := make([]string, numBuckets)
			for i := 0; i < numBuckets; i++ {
				bucketTime := globalStart.Add(time.Duration(i)*bucketSize + bucketSize/2)
				timestamps[i] = bucketTime.Local().Format("15:04:05")
			}
			nm.Timestamps = timestamps
			nm.TSP50 = make([]float64, numBuckets)
			nm.TSP95 = make([]float64, numBuckets)
			nm.TSP99 = make([]float64, numBuckets)
			nm.TSAvg = make([]float64, numBuckets)
			nm.TSQPS = make([]float64, numBuckets)
		}

		result = append(result, nm)
	}

	// --- Report Fallback: supplement Group child nodes missing from tracer spans ---
	// Group child nodes execute via executeGroup() directly, bypassing DAG executor's span recording.
	// Their metrics exist in r.nodeStats (HTML report) but not in tracer spans (Dashboard).
	// Fallback: query latest completed run's report detail for missing nodes.
	tracerNodeSet := make(map[string]bool, len(nodeIDs))
	for _, id := range nodeIDs {
		tracerNodeSet[id] = true
	}

	if sceneID > 0 {
		if reports, rptErr := h.reports.List(r.Context(), repo.Filter{SceneID: snowflake.ID(sceneID), Limit: 1}); rptErr == nil && len(reports) > 0 {
			latestReport := reports[0]
			if latestReport.Detail != "" && latestReport.Detail != "{}" {
				var rptDetail struct {
					Metadata struct {
						StartedAt  string  `json:"started_at"`
						FinishedAt string  `json:"finished_at"`
						Duration   float64 `json:"duration_sec"`
					} `json:"metadata"`
					NodeMetrics []struct {
						NodeID   string `json:"node_id"`
						NodeName string `json:"node_name"`
						NodeType string `json:"node_type,omitempty"`
						Summary  struct {
							TotalRequests int64   `json:"total_requests"`
							SuccessCount  int64   `json:"success_count"`
							FailCount     int64   `json:"fail_count"`
							SuccessRate   float64 `json:"success_rate"`
							AvgLatencyMs  float64 `json:"avg_latency_ms"`
							P50LatencyMs  float64 `json:"p50_latency_ms"`
							P95LatencyMs  float64 `json:"p95_latency_ms"`
							P99LatencyMs  float64 `json:"p99_latency_ms"`
							MaxLatencyMs  float64 `json:"max_latency_ms"`
							MinLatencyMs  float64 `json:"min_latency_ms"`
						} `json:"summary"`
					} `json:"node_metrics"`
				}
				if json.Unmarshal([]byte(latestReport.Detail), &rptDetail) == nil {
					runDurSec := rptDetail.Metadata.Duration
					if runDurSec <= 0 {
						runDurSec = totalDur.Seconds()
					}
					for _, nm := range rptDetail.NodeMetrics {
						if tracerNodeSet[nm.NodeID] {
							continue // already in tracer data
						}
						avgQPS := float64(0)
						if runDurSec > 0 && nm.Summary.TotalRequests > 0 {
							avgQPS = float64(nm.Summary.TotalRequests) / runDurSec
						}
						supplement := dto.NodeMetricDTO{
							NodeID:      nm.NodeID,
							Name:        nm.NodeName,
							Type:        nm.NodeType,
							SortOrder:   orderMap[nm.NodeID],
							TotalReqs:   nm.Summary.TotalRequests,
							SuccessReqs: nm.Summary.SuccessCount,
							AvgLatency:  nm.Summary.AvgLatencyMs,
							P50Latency:  nm.Summary.P50LatencyMs,
							P95Latency:  nm.Summary.P95LatencyMs,
							P99Latency:  nm.Summary.P99LatencyMs,
							Timestamps:  []string{},
							TSP50:       []float64{},
							TSP95:       []float64{},
							TSP99:       []float64{},
							TSAvg:       []float64{},
							TSQPS:       []float64{avgQPS},
						}
						if supplement.Name == "" {
							supplement.Name = nm.NodeID
						}
						if supplement.Type == "" {
							supplement.Type = "http"
						}
						result = append(result, supplement)
					}
				}
			}
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].SortOrder < result[j].SortOrder })
	return result
}

func avgFloat64(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func percentileFloat64(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (h *Handler) computeNodeOrder(r *http.Request, sceneIDStr string, nodeIDs []string) map[string]int {
	if sceneIDStr == "" || len(nodeIDs) == 0 {
		result := make(map[string]int)
		for i, id := range nodeIDs {
			result[id] = i
		}
		return result
	}
	var sceneID snowflake.ID
	if err := sceneID.Parse(sceneIDStr); err != nil {
		result := make(map[string]int)
		for i, id := range nodeIDs {
			result[id] = i
		}
		return result
	}
	edges, err := h.edges.List(r.Context(), repo.Filter{SceneID: sceneID})
	if err != nil || len(edges) == 0 {
		result := make(map[string]int)
		for i, id := range nodeIDs {
			result[id] = i
		}
		return result
	}
	nodeSet := make(map[string]bool)
	for _, id := range nodeIDs {
		nodeSet[id] = true
	}
	adj := make(map[string][]string)
	inDeg := make(map[string]int)
	for _, n := range nodeIDs {
		adj[n] = nil
		inDeg[n] = 0
	}
	for _, e := range edges {
		from := e.FromNode.String()
		to := e.ToNode.String()
		if !nodeSet[from] || !nodeSet[to] {
			continue
		}
		adj[from] = append(adj[from], to)
		inDeg[to]++
	}
	var queue []string
	for n, d := range inDeg {
		if d == 0 {
			queue = append(queue, n)
		}
	}
	order := make(map[string]int)
	idx := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order[cur] = idx
		idx++
		for _, next := range adj[cur] {
			inDeg[next]--
			if inDeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	for _, n := range nodeIDs {
		if _, ok := order[n]; !ok {
			order[n] = idx
			idx++
		}
	}
	return order
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
		timestamps[i] = t.Local().Format("15:04:05")
		bucketEnd := t.Add(time.Duration(interval) * time.Second)

		for _, run := range runs {
			if run.StartedAt == nil {
				continue
			}
			runStart := *run.StartedAt
			isRunning := run.Status == "running"

			var inBucket bool
			if isRunning {
				inBucket = !runStart.After(bucketEnd)
			} else {
				inBucket = runStart.After(t) && !runStart.After(bucketEnd)
			}

			if inBucket {
				dur := run.Duration
				if dur > 0 {
					qps[i] += float64(run.TotalReqs) / dur
				}
				p50[i] = run.P50Latency * 1e3
				p95[i] = run.P95Latency * 1e3
				p99[i] = run.P99Latency * 1e3
				if run.TotalReqs > 0 {
					errRate[i] += float64(run.FailedReqs) / float64(run.TotalReqs) * 100
				}
			}
		}
	}

	// Trim trailing zero-value buckets (running tests: windowEnd=now
	// but actual data may lag behind by a few seconds).
	n := trimTrailingZeros(qps, p50, p95, p99, errRate)
	timestamps = timestamps[:n]
	qps = qps[:n]
	p50 = p50[:n]
	p95 = p95[:n]
	p99 = p99[:n]
	errRate = errRate[:n]

	return &dto.TimeSeriesDTO{
		Timestamps: timestamps,
		QPS:        qps,
		P50:        p50,
		P95:        p95,
		P99:        p99,
		ErrorRate:  errRate,
	}
}

// trimTrailingZeros returns the length after removing trailing entries
// where ALL metric arrays are zero. Keeps at least 1 entry.
func trimTrailingZeros(arrays ...[]float64) int {
	if len(arrays) == 0 || len(arrays[0]) == 0 {
		return 0
	}
	n := len(arrays[0])
	for n > 1 {
		allZero := true
		for _, a := range arrays {
			if a[n-1] != 0 {
				allZero = false
				break
			}
		}
		if !allZero {
			break
		}
		n--
	}
	return n
}

func (h *Handler) buildTimeSeriesWithDB(ctx context.Context, runs []dto.RunRecordDTO, rangeSeconds int) *dto.TimeSeriesDTO {
	var windowStart, windowEnd time.Time
	hasRunning := false

	for _, run := range runs {
		if run.StartedAt != nil {
			runStart := *run.StartedAt
			if windowStart.IsZero() || runStart.Before(windowStart) {
				windowStart = runStart
			}
		}
		if run.Status == "running" {
			hasRunning = true
			windowEnd = time.Now()
		} else if run.FinishedAt != nil {
			if windowEnd.IsZero() || run.FinishedAt.After(windowEnd) {
				windowEnd = *run.FinishedAt
			}
		}
	}

	if !hasRunning && !windowStart.IsZero() && !windowEnd.IsZero() {
		// Dynamic padding: 10% of test duration, min 10s, max 60s
		padding := windowEnd.Sub(windowStart) / 10
		if padding < 10*time.Second {
			padding = 10 * time.Second
		}
		if padding > 60*time.Second {
			padding = 60 * time.Second
		}
		windowStart = windowStart.Add(-padding)
		windowEnd = windowEnd.Add(padding)
	} else if hasRunning && !windowStart.IsZero() {
		// For running tests, only pad the start; end is "now"
		padding := time.Since(windowStart) / 10
		if padding < 10*time.Second {
			padding = 10 * time.Second
		}
		if padding > 60*time.Second {
			padding = 60 * time.Second
		}
		windowStart = windowStart.Add(-padding)
		windowEnd = time.Now()
	}

	actualRangeSec := int(windowEnd.Sub(windowStart).Seconds())
	if actualRangeSec < 60 {
		actualRangeSec = 60
	}

	points := min(actualRangeSec, 120)
	interval := float64(actualRangeSec) / float64(points)

	timestamps := make([]string, points)
	qps := make([]float64, points)
	p50 := make([]float64, points)
	p95 := make([]float64, points)
	p99 := make([]float64, points)
	errRate := make([]float64, points)

	for i := 0; i < points; i++ {
		t := windowStart.Add(time.Duration(float64(i)*interval) * time.Second)
		timestamps[i] = t.Local().Format("15:04:05")
		bucketEnd := t.Add(time.Duration(interval) * time.Second)

		for _, run := range runs {
			if run.StartedAt == nil {
				continue
			}

			tsData, tsErr := h.tsStore.QueryByRunID(ctx, run.ID)
			if tsErr == nil && len(tsData) > 0 {
				for _, record := range tsData {
					if record.NodeID != "" {
						continue
					}
					if record.SampleTime.After(t) && !record.SampleTime.After(bucketEnd) {
						qps[i] += record.QPS
						p50[i] = record.P50LatencyMs
						p95[i] = record.P95LatencyMs
						p99[i] = record.P99LatencyMs
						if record.TotalRequests > 0 {
							errRate[i] += float64(record.FailCount) / float64(record.TotalRequests) * 100
						}
					}
				}
			} else {
				runStart := *run.StartedAt
				isRunning := run.Status == "running"

				var runEnd time.Time
				if isRunning {
					runEnd = time.Now()
				} else if run.FinishedAt != nil {
					runEnd = *run.FinishedAt
				} else {
					runEnd = runStart.Add(time.Duration(run.Duration) * time.Second)
				}

				bucketStart := t
				bucketEndTime := bucketEnd

				overlaps := !(runEnd.Before(bucketStart) || runStart.After(bucketEndTime))

				if overlaps {
					dur := run.Duration
					if dur > 0 {
						qps[i] += float64(run.TotalReqs) / dur
					}
					p50[i] = run.P50Latency * 1e3
					p95[i] = run.P95Latency * 1e3
					p99[i] = run.P99Latency * 1e3
					if run.TotalReqs > 0 {
						errRate[i] += float64(run.FailedReqs) / float64(run.TotalReqs) * 100
					}
				}
			}
		}
	}

	// Trim trailing zero-value buckets (running tests: windowEnd=now
	// but actual data may lag behind by a few seconds).
	n := trimTrailingZeros(qps, p50, p95, p99, errRate)
	timestamps = timestamps[:n]
	qps = qps[:n]
	p50 = p50[:n]
	p95 = p95[:n]
	p99 = p99[:n]
	errRate = errRate[:n]

	return &dto.TimeSeriesDTO{
		Timestamps:  timestamps,
		QPS:         qps,
		P50:         p50,
		P95:         p95,
		P99:         p99,
		ErrorRate:   errRate,
		WindowStart: windowStart.Local().Format("2006-01-02T15:04:05"),
		WindowEnd:   windowEnd.Local().Format("2006-01-02T15:04:05"),
		HasRunning:  hasRunning,
	}
}

func (h *Handler) samplesToTimeSeriesDTO(samples []runner.Sample) *dto.TimeSeriesDTO {
	if len(samples) == 0 {
		return nil
	}

	timestamps := make([]string, len(samples))
	qps := make([]float64, len(samples))
	p50 := make([]float64, len(samples))
	p95 := make([]float64, len(samples))
	p99 := make([]float64, len(samples))
	errRate := make([]float64, len(samples))

	for i, s := range samples {
		timestamps[i] = s.Timestamp.Local().Format("15:04:05")
		qps[i] = s.QPS
		p50[i] = s.P50LatencyMs
		p95[i] = s.P95LatencyMs
		p99[i] = s.P99LatencyMs
		if s.TotalRequests > 0 {
			errRate[i] = float64(s.FailCount) / float64(s.TotalRequests) * 100
		}
	}

	return &dto.TimeSeriesDTO{
		Timestamps: timestamps,
		QPS:        qps,
		P50:        p50,
		P95:        p95,
		P99:        p99,
		ErrorRate:  errRate,
		HasRunning: true,
	}
}
