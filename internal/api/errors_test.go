package api

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

func newTestHandler() *Handler {
	log, err := logger.New(logger.Config{Level: "error"})
	if err != nil {
		panic(err)
	}
	return &Handler{log: log}
}

func TestInternalErr_UnknownErrorMasksDetails(t *testing.T) {
	h := newTestHandler()
	resp := h.internalErr("create user", errors.New("UNIQUE constraint failed: users.email"))
	assert.Equal(t, 500, resp.Code)
	assert.Equal(t, "服务器内部错误", resp.Message)
}

func TestInternalErr_EmailTakenMapsTo409(t *testing.T) {
	h := newTestHandler()
	resp := h.internalErr("create user", repo.ErrEmailTaken)
	assert.Equal(t, 409, resp.Code)
	assert.Equal(t, "邮箱已被使用", resp.Message)
}

func TestInternalErr_NoRowsMapsTo404(t *testing.T) {
	h := newTestHandler()
	resp := h.internalErr("get scene", sql.ErrNoRows)
	assert.Equal(t, 404, resp.Code)
}
