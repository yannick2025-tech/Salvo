package api

import (
	"database/sql"
	"errors"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

// internalErr maps an execution error to a unified API response so that raw
// backend errors are never leaked to the frontend:
//
//   - known business sentinel errors map to their 4xx semantic responses
//     with a user-friendly Chinese message;
//   - everything else is treated as an internal error: the response is the
//     generic 500 message and the full original error is only logged
//     (with the op prefix) for troubleshooting.
func (h *Handler) internalErr(op string, err error) dto.Response {
	switch {
	case errors.Is(err, repo.ErrEmailTaken):
		return dto.ErrorResp(409, "邮箱已被使用")
	case errors.Is(err, sql.ErrNoRows):
		return dto.ErrorResp(404, "资源不存在")
	default:
		h.log.Error("internal error", logger.F("op", op), logger.F("error", err))
		return dto.ErrorResp(500, "服务器内部错误")
	}
}
