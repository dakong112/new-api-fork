package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// registerAuthzRoutes mounts the authorization API under its own /authz
// namespace. GET /authz/catalog returns the permission schema (resources,
// actions, and role baselines) used by the client permission editor; the
// /roles routes manage custom roles. Only root manages authorization.
func registerAuthzRoutes(apiRouter *gin.RouterGroup) {
	authzRoute := apiRouter.Group("/authz")
	authzRoute.Use(middleware.RootAuth(), middleware.DisableCache())
	{
		authzRoute.GET("/catalog", controller.GetPermissionCatalog)
		authzRoute.GET("/roles", controller.ListAuthzRoles)
		authzRoute.POST("/roles", controller.CreateAuthzRole)
		authzRoute.PUT("/roles/:key", controller.UpdateAuthzRole)
		authzRoute.DELETE("/roles/:key", controller.DeleteAuthzRole)
		authzRoute.PUT("/roles/:key/permissions", controller.SetAuthzRolePermissions)
	}
}
