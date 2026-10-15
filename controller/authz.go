package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
)

// GetPermissionCatalog returns the permission schema used by the client to
// render the permission editor: the registry of resources with their actions
// and display label keys, plus the roles with their baseline grant matrices.
// ?app= selects one application's resources: omitted means this gateway,
// "canvas" the canvas application, "*" every application.
func GetPermissionCatalog(c *gin.Context) {
	app := c.Query("app")
	resources := make([]authz.ResourceDefinition, 0)
	for _, resource := range authz.Catalog() {
		if app == "*" || resource.App == app {
			resources = append(resources, resource)
		}
	}
	roles, err := authz.Roles()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"resources": resources, "roles": roles})
}

// writeAuthzRoleError maps role management errors to web console messages.
func writeAuthzRoleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, authz.ErrRoleNotFound):
		common.ApiErrorT(c, "Role not found")
	case errors.Is(err, authz.ErrRoleExists):
		common.ApiErrorT(c, "Role already exists")
	case errors.Is(err, authz.ErrRoleBuiltIn):
		common.ApiErrorT(c, "Built-in roles cannot be changed")
	case errors.Is(err, authz.ErrRoleInvalid):
		common.ApiErrorT(c, "Invalid role key or name")
	case errors.Is(err, authz.ErrPermissionUnknown):
		common.ApiErrorT(c, "Unknown permission")
	default:
		common.ApiError(c, err)
	}
}

type authzRoleRequest struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListAuthzRoles returns built-in and custom roles with their grant matrices.
func ListAuthzRoles(c *gin.Context) {
	roles, err := authz.Roles()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, roles)
}

func CreateAuthzRole(c *gin.Context) {
	var request authzRoleRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := authz.CreateRole(strings.TrimSpace(request.Key), request.Name, request.Description); err != nil {
		writeAuthzRoleError(c, err)
		return
	}
	recordManageAuditFor(c, 0, "authz.role_create", map[string]any{"key": request.Key})
	common.ApiSuccess(c, nil)
}

func UpdateAuthzRole(c *gin.Context) {
	var request authzRoleRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	key := c.Param("key")
	if err := authz.UpdateRole(key, request.Name, request.Description); err != nil {
		writeAuthzRoleError(c, err)
		return
	}
	recordManageAuditFor(c, 0, "authz.role_update", map[string]any{"key": key})
	common.ApiSuccess(c, nil)
}

func DeleteAuthzRole(c *gin.Context) {
	key := c.Param("key")
	if err := authz.DeleteRole(key); err != nil {
		writeAuthzRoleError(c, err)
		return
	}
	recordManageAuditFor(c, 0, "authz.role_delete", map[string]any{"key": key})
	common.ApiSuccess(c, nil)
}

// SetAuthzRolePermissions replaces a custom role's grants with the given
// "resource.action" codes.
func SetAuthzRolePermissions(c *gin.Context) {
	var request struct {
		Permissions []string `json:"permissions"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	key := c.Param("key")
	if err := authz.SetRolePermissions(key, request.Permissions); err != nil {
		writeAuthzRoleError(c, err)
		return
	}
	recordManageAuditFor(c, 0, "authz.role_permissions", map[string]any{"key": key, "permissions": strings.Join(request.Permissions, ",")})
	common.ApiSuccess(c, nil)
}

func GetUserAuthzRoles(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if _, err := model.GetUserById(id, false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"roles": authz.UserRoles(id)})
}

// SetUserAuthzRoles replaces the custom roles of a user. Changing what an
// account may do needs the same step-up proof as editing admin permissions.
func SetUserAuthzRoles(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	var request struct {
		Roles []string `json:"roles"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	authorization := requireAdminUserProof(c, service.VerificationScopeAdminUserUpdate, service.AdminUserContext{UserID: user.Id})
	if authorization == nil {
		return
	}
	if err := authz.SetUserRoles(user.Id, request.Roles); err != nil {
		writeAuthzRoleError(c, err)
		return
	}
	recordManageAuditFor(c, user.Id, "user.roles_update", map[string]any{
		"username":            user.Username,
		"id":                  user.Id,
		"roles":               strings.Join(authz.UserRoles(user.Id), ","),
		"verification_method": authorization.Method,
	})
	common.ApiSuccess(c, gin.H{"roles": authz.UserRoles(user.Id)})
}
