package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yannick2025-tech/Salvo/internal/api/dto"
	"github.com/yannick2025-tech/Salvo/internal/auth"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
	"github.com/yannick2025-tech/Salvo/internal/store/repo"
)

func (h *Handler) Login(r *http.Request) dto.Response {
	req, err := decode[dto.LoginRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.Email == "" || req.Password == "" {
		return dto.ErrorResp(400, "邮箱和密码不能为空")
	}
	if !isValidEmail(req.Email) {
		return dto.ErrorResp(400, "邮箱格式不正确")
	}

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(401, "邮箱或密码错误")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get user: %v", err))
	}

	if user.Status != model.UserStatusActive {
		return dto.ErrorResp(403, "account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return dto.ErrorResp(401, "invalid email or password")
	}

	role, _ := h.roles.GetByID(r.Context(), user.RoleID)
	roleName := ""
	if role != nil {
		roleName = role.Name
	}

	h.log.Debug("login: generating token",
		logger.F("user_id", user.ID),
		logger.F("user_role_id", user.RoleID),
		logger.F("role_name", roleName),
	)

	token, err := h.jwt.Generate(user.ID, roleName)
	if err != nil {
		return dto.ErrorResp(500, "failed to generate token")
	}

	now := time.Now().UTC()
	user.LastLoginAt = &now
	_ = h.users.Update(r.Context(), user)

	h.log.Debug("login: token generated successfully",
		logger.F("user_id", user.ID),
		logger.F("role_name", roleName),
	)

	return dto.OK(dto.LoginResponse{
		Token: token,
		User:  toUserDTO(user, roleName),
	})
}

func (h *Handler) Me(r *http.Request) dto.Response {
	userID := auth.UserIDFromCtx(r.Context())
	if userID == 0 {
		return dto.ErrorResp(401, "not authenticated")
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		// User not found: token references a user that no longer exists
		// (e.g., after DB rebuild). Return 401 to trigger client-side
		// token invalidation rather than 404 which is confusing.
		return dto.ErrorResp(401, "invalid or expired token")
	}

	role, _ := h.roles.GetByID(r.Context(), user.RoleID)
	roleName := ""
	if role != nil {
		roleName = role.Name
	}

	userDTO := toUserDTO(user, roleName)

	perms, _ := h.rp.ListPermissions(r.Context(), user.RoleID)
	permStrs := make([]string, 0, len(perms))
	for _, p := range perms {
		permStrs = append(permStrs, p.Resource+":"+p.Action)
	}

	return dto.OK(map[string]any{
		"user":        userDTO,
		"permissions": permStrs,
	})
}

func (h *Handler) Logout(r *http.Request) dto.Response {
	return dto.OK(nil)
}

func (h *Handler) ChangePassword(r *http.Request) dto.Response {
	req, err := decode[dto.ChangePasswordRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		return dto.ErrorResp(400, "当前密码和新密码不能为空")
	}

	userID := auth.UserIDFromCtx(r.Context())
	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		return dto.ErrorResp(404, "用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return dto.ErrorResp(401, "当前密码不正确")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return dto.ErrorResp(500, "密码加密失败")
	}

	user.PasswordHash = string(hash)
	if err := h.users.Update(r.Context(), user); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("更新用户失败: %v", err))
	}

	return dto.OK(nil)
}

func (h *Handler) ResetPassword(r *http.Request) dto.Response {
	req, err := decode[dto.ResetPasswordRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.UserID == 0 || req.NewPassword == "" {
		return dto.ErrorResp(400, "用户ID和新密码不能为空")
	}

	user, err := h.users.GetByID(r.Context(), req.UserID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "用户不存在")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("获取用户失败: %v", err))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return dto.ErrorResp(500, "failed to hash password")
	}

	user.PasswordHash = string(hash)
	if err := h.users.Update(r.Context(), user); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update user: %v", err))
	}

	return dto.OK(nil)
}

func (h *Handler) ListUsers(r *http.Request) dto.Response {
	req, err := decode[dto.ListUsersRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	users, err := h.users.List(r.Context(), repo.Filter{
		Status: req.Status,
		Offset: req.Offset,
		Limit:  limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list users: %v", err))
	}

	items := make([]dto.UserDTO, 0, len(users))
	for _, u := range users {
		role, _ := h.roles.GetByID(r.Context(), u.RoleID)
		roleName := ""
		if role != nil {
			roleName = role.Name
		}
		items = append(items, toUserDTO(u, roleName))
	}

	return dto.OK(dto.ListResponse[[]dto.UserDTO]{
		Items:      items,
		Pagination: dto.Pagination{Total: len(items), Offset: req.Offset, Limit: limit},
	})
}

func (h *Handler) CreateUser(r *http.Request) dto.Response {
	req, err := decode[dto.CreateUserRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.Email == "" || req.Password == "" {
		return dto.ErrorResp(400, "邮箱和密码不能为空")
	}
	if !isValidEmail(req.Email) {
		return dto.ErrorResp(400, "邮箱格式不正确")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return dto.ErrorResp(500, "密码加密失败")
	}

	user := &model.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Nickname:     req.Nickname,
		RoleID:       req.RoleID,
		Status:       model.UserStatusActive,
	}
	if user.Nickname == "" {
		user.Nickname = strings.Split(req.Email, "@")[0]
	}

	if err := h.users.Create(r.Context(), user); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("create user: %v", err))
	}

	role, _ := h.roles.GetByID(r.Context(), user.RoleID)
	roleName := ""
	if role != nil {
		roleName = role.Name
	}

	return dto.OK(toUserDTO(user, roleName))
}

func (h *Handler) UpdateUser(r *http.Request) dto.Response {
	req, err := decode[dto.UpdateUserRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	user, err := h.users.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "user not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get user: %v", err))
	}

	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.RoleID != 0 {
		user.RoleID = req.RoleID
	}
	if req.Status != "" {
		user.Status = req.Status
	}

	if err := h.users.Update(r.Context(), user); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update user: %v", err))
	}

	role, _ := h.roles.GetByID(r.Context(), user.RoleID)
	roleName := ""
	if role != nil {
		roleName = role.Name
	}

	return dto.OK(toUserDTO(user, roleName))
}

func (h *Handler) DeleteUser(r *http.Request) dto.Response {
	req, err := decode[dto.IDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	if err := h.users.Delete(r.Context(), req.ID); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("delete user: %v", err))
	}

	return dto.OK(nil)
}

func (h *Handler) ListRoles(r *http.Request) dto.Response {
	req, err := decode[dto.ListRolesRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	roles, err := h.roles.List(r.Context(), repo.Filter{
		Offset: req.Offset,
		Limit:  limit,
	})
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("list roles: %v", err))
	}

	items := make([]dto.RoleDTO, 0, len(roles))
	for _, rl := range roles {
		perms, _ := h.rp.ListPermissions(r.Context(), rl.ID)
		items = append(items, toRoleDTO(rl, perms))
	}

	return dto.OK(dto.ListResponse[[]dto.RoleDTO]{
		Items:      items,
		Pagination: dto.Pagination{Total: len(items), Offset: req.Offset, Limit: limit},
	})
}

func (h *Handler) CreateRole(r *http.Request) dto.Response {
	req, err := decode[dto.CreateRoleRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.Name == "" {
		return dto.ErrorResp(400, "name is required")
	}

	role := &model.Role{
		Name:        req.Name,
		Description: req.Description,
		IsBuiltin:   false,
	}

	if err := h.roles.Create(r.Context(), role); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("create role: %v", err))
	}

	return dto.OK(toRoleDTO(role, nil))
}

func (h *Handler) UpdateRole(r *http.Request) dto.Response {
	req, err := decode[dto.UpdateRoleRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	role, err := h.roles.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "role not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get role: %v", err))
	}

	if role.IsBuiltin {
		return dto.ErrorResp(403, "cannot modify built-in role")
	}

	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	if err := h.roles.Update(r.Context(), role); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("update role: %v", err))
	}

	if req.Permissions != nil {
		_ = h.rp.RevokeAll(r.Context(), role.ID)
		for _, permKey := range req.Permissions {
			parts := strings.SplitN(permKey, ":", 2)
			if len(parts) != 2 {
				continue
			}
			p, err := h.perms.GetByResourceAction(r.Context(), parts[0], parts[1])
			if err != nil {
				continue
			}
			_ = h.rp.Assign(r.Context(), role.ID, p.ID)
		}
	}

	perms, _ := h.rp.ListPermissions(r.Context(), role.ID)
	return dto.OK(toRoleDTO(role, perms))
}

func (h *Handler) DeleteRole(r *http.Request) dto.Response {
	req, err := decode[dto.IDRequest](r)
	if err != nil {
		return dto.ErrorResp(400, err.Error())
	}
	if req.ID == 0 {
		return dto.ErrorResp(400, "id is required")
	}

	role, err := h.roles.GetByID(r.Context(), req.ID)
	if err == sql.ErrNoRows {
		return dto.ErrorResp(404, "role not found")
	}
	if err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("get role: %v", err))
	}

	if role.IsBuiltin {
		return dto.ErrorResp(403, "cannot delete built-in role")
	}

	if err := h.roles.Delete(r.Context(), req.ID); err != nil {
		return dto.ErrorResp(500, fmt.Sprintf("delete role: %v", err))
	}

	return dto.OK(nil)
}

func toUserDTO(u *model.User, roleName string) dto.UserDTO {
	return dto.UserDTO{
		ID:          u.ID,
		Email:       u.Email,
		Nickname:    u.Nickname,
		RoleID:      u.RoleID,
		RoleName:    roleName,
		Status:      u.Status,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func toRoleDTO(r *model.Role, perms []*model.Permission) dto.RoleDTO {
	permDTOs := make([]dto.PermissionDTO, 0, len(perms))
	for _, p := range perms {
		permDTOs = append(permDTOs, dto.PermissionDTO{
			ID:          p.ID,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
		})
	}
	return dto.RoleDTO{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		IsBuiltin:   r.IsBuiltin,
		Permissions: permDTOs,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toPermissionDTO(p *model.Permission) dto.PermissionDTO {
	return dto.PermissionDTO{
		ID:          p.ID,
		Resource:    p.Resource,
		Action:      p.Action,
		Description: p.Description,
	}
}

func toPermissionDTOs(perms []*model.Permission) []dto.PermissionDTO {
	items := make([]dto.PermissionDTO, 0, len(perms))
	for _, p := range perms {
		items = append(items, toPermissionDTO(p))
	}
	return items
}

var _ = snowflake.ID(0)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
