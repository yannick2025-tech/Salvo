package api

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

// --- SO Plugin Handlers ---

func (h *Handler) UploadSOPlugin(r *http.Request) dto.Response {
	req, err := decode[dto.UploadSOPluginRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	if req.Name == "" || req.Version == "" || req.FilePath == "" {
		return dto.ErrorResp(400, "name, version, and file_path are required")
	}

	status := req.Status
	if status == "" {
		status = model.SOPluginStatusDisabled
	}

	p := &model.SOPlugin{
		Name:     req.Name,
		Version:  req.Version,
		FilePath: req.FilePath,
		Status:   status,
		Config:   req.Config,
	}

	if err := h.soPlugins.Create(r.Context(), p); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("create so plugin: %v", err))
	}

	return dto.OK(toSOPluginDTO(p))
}

func (h *Handler) ListSOPlugins(r *http.Request) dto.Response {
	req, err := decode[dto.ListSOPluginsRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	plugins, err := h.soPlugins.List(r.Context(), repo.Filter{
		Status: req.Status,
		Offset: req.Offset,
		Limit:  limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list so plugins: %v", err))
	}

	items := make([]dto.SOPluginDTO, 0, len(plugins))
	for _, p := range plugins {
		items = append(items, toSOPluginDTO(p))
	}

	return dto.OK(dto.ListResponse[[]dto.SOPluginDTO]{
		Items:      items,
		Pagination: dto.Pagination{Total: len(items), Offset: req.Offset, Limit: limit},
	})
}

func (h *Handler) GetSOPlugin(r *http.Request) dto.Response {
	req, err := decode[dto.GetSOPluginRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	p, err := h.soPlugins.GetByID(r.Context(), req.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			return dto.ErrorResp(404, "so plugin not found")
		}
		return dto.ErrorResp(500, fmt.Sprintf("get so plugin: %v", err))
	}

	return dto.OK(toSOPluginDTO(p))
}

func (h *Handler) UpdateSOPluginStatus(r *http.Request) dto.Response {
	req, err := decode[dto.UpdateSOPluginStatusRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	if req.Status != model.SOPluginStatusEnabled && req.Status != model.SOPluginStatusDisabled {
		return dto.ErrorResp(400, "status must be 'enabled' or 'disabled'")
	}

	if err := h.soPlugins.UpdateStatus(r.Context(), req.ID, req.Status); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update so plugin status: %v", err))
	}

	return dto.OK(nil)
}

func (h *Handler) UpdateSOPluginConfig(r *http.Request) dto.Response {
	req, err := decode[dto.UpdateSOPluginConfigRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	if err := h.soPlugins.UpdateConfig(r.Context(), req.ID, req.Config); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update so plugin config: %v", err))
	}

	return dto.OK(nil)
}

func (h *Handler) DeleteSOPlugin(r *http.Request) dto.Response {
	req, err := decode[dto.DeleteSOPluginRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	if err := h.soPlugins.Delete(r.Context(), req.ID); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("delete so plugin: %v", err))
	}

	return dto.OK(nil)
}

// --- Helpers ---

func toSOPluginDTO(p *model.SOPlugin) dto.SOPluginDTO {
	return dto.SOPluginDTO{
		ID:        p.ID,
		Name:      p.Name,
		Version:   p.Version,
		FilePath:  p.FilePath,
		Status:    p.Status,
		Config:    p.Config,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}