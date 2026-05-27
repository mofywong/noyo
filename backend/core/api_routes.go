package core

import "github.com/gogf/gf/v2/net/ghttp"

func permissionGroup(group *ghttp.RouterGroup, permission string) *ghttp.RouterGroup {
	protected := group.Group("/")
	protected.Middleware(PermissionMiddleware(permission))
	return protected
}

func permissionGET(group *ghttp.RouterGroup, pattern, permission string, handler ghttp.HandlerFunc) {
	permissionGroup(group, permission).GET(pattern, handler)
}

func permissionPOST(group *ghttp.RouterGroup, pattern, permission string, handler ghttp.HandlerFunc) {
	permissionGroup(group, permission).POST(pattern, handler)
}

func permissionPUT(group *ghttp.RouterGroup, pattern, permission string, handler ghttp.HandlerFunc) {
	permissionGroup(group, permission).PUT(pattern, handler)
}

func permissionDELETE(group *ghttp.RouterGroup, pattern, permission string, handler ghttp.HandlerFunc) {
	permissionGroup(group, permission).DELETE(pattern, handler)
}

func (s *Server) registerAPIRoutes() {
	s.WebServer.Group("/api", func(group *ghttp.RouterGroup) {
		group.Group("/auth", func(authGroup *ghttp.RouterGroup) {
			authGroup.POST("/login", s.handleLogin)
			authGroup.POST("/refresh", s.handleRefreshToken)
			authGroup.GET("/tenant-info", s.handleGetTenantBySuffix)
		})

		group.Group("/", func(protected *ghttp.RouterGroup) {
			protected.Middleware(AuthMiddleware(s.Config.Auth.JWTSecret), AuditMiddleware())

			protected.POST("/auth/logout", s.handleLogout)
			protected.GET("/auth/profile", s.handleGetProfile)
			protected.PUT("/auth/password", s.handleChangePassword)

			pluginGroup := protected.Group("/plugins")
			pluginGroup.GET("/", s.handleListPlugins)
			pluginGroup.GET("/:name", s.handleGetPlugin)
			pluginGroup.GET("/:name/schemas", s.handleGetPluginSchemas)
			permissionPOST(pluginGroup, "/:name/config", "plugin:config", s.handleUpdatePluginConfig)
			permissionPOST(pluginGroup, "/:name/discover", "plugin:config", s.handlePluginDiscover)

			systemGroup := protected.Group("/system")
			systemGroup.Middleware(SystemAdminMiddleware())
			systemGroup.GET("/stats", s.handleSystemStats)
			systemGroup.GET("/config", s.handleGetSystemConfig)
			systemGroup.POST("/config", s.handleUpdateSystemConfig)
			systemGroup.GET("/log/config", s.handleGetLogConfig)
			systemGroup.POST("/log/config", s.handleUpdateLogConfig)
			systemGroup.GET("/log/files", s.handleListLogFiles)
			systemGroup.GET("/log/file", s.handleReadLogFile)
			systemGroup.GET("/log/tail", s.handleTailLog)
			systemGroup.GET("/log/download", s.handleDownloadLogFile)
			systemGroup.ALL("/log/stream", s.handleRealtimeLogs)

			tenantProtectedGroup := protected.Group("/")
			tenantProtectedGroup.Middleware(TenantMiddleware())
			permissionPOST(tenantProtectedGroup, "/history/query", "device:list", s.handleQueryHistory)
			permissionDELETE(tenantProtectedGroup, "/history/record", "alarm:handle", s.handleDeleteRecord)
			permissionPOST(tenantProtectedGroup, "/system/upload/image", "device:edit", s.handleUploadImage)
			s.RegisterDeviceRoutes(tenantProtectedGroup)

			protected.Group("/users", func(userGroup *ghttp.RouterGroup) {
				userGroup.Middleware(TenantMiddleware())
				permissionGET(userGroup, "/", "user:list", s.handleListUsers)
				permissionPOST(userGroup, "/", "user:create", s.handleCreateUser)
				permissionPUT(userGroup, "/:id", "user:edit", s.handleUpdateUser)
				permissionDELETE(userGroup, "/:id", "user:delete", s.handleDeleteUser)
				permissionPOST(userGroup, "/:id/reset-password", "user:edit", s.handleResetPassword)
				permissionGET(userGroup, "/:id/positions", "user:list", s.handleGetUserPositions)
				permissionPUT(userGroup, "/:id/positions", "user:edit", s.handleSetUserPositions)
				permissionGET(userGroup, "/:id/roles", "user:list", s.handleGetUserRoles)
				permissionPUT(userGroup, "/:id/roles", "user:edit", s.handleSetUserRoles)
				permissionGET(userGroup, "/:id/projects", "user:list", s.handleGetUserProjects)
				permissionPUT(userGroup, "/:id/projects", "user:edit", s.handleSetUserProjects)
			})

			protected.Group("/roles", func(roleGroup *ghttp.RouterGroup) {
				roleGroup.Middleware(TenantMiddleware())
				permissionGET(roleGroup, "/", "role:list", s.handleGetRoles)
				permissionPOST(roleGroup, "/", "role:create", s.handleCreateRole)
				permissionPUT(roleGroup, "/:id", "role:edit", s.handleUpdateRole)
				permissionDELETE(roleGroup, "/:id", "role:delete", s.handleDeleteRole)
				permissionGET(roleGroup, "/:id/permissions", "role:list", s.handleGetRolePermissions)
				permissionPUT(roleGroup, "/:id/permissions", "role:edit", s.handleSetRolePermissions)
			})

			protected.Group("/permissions", func(permGroup *ghttp.RouterGroup) {
				permissionGET(permGroup, "/", "role:list", s.handleGetSystemPermissions)
			})

			positionGroup := protected.Group("/positions")
			positionGroup.Middleware(TenantMiddleware())
			permissionGET(positionGroup, "/", "position:list", s.handleListPositions)
			permissionPOST(positionGroup, "/", "position:create", s.handleCreatePosition)
			permissionPUT(positionGroup, "/:id", "position:edit", s.handleUpdatePosition)
			permissionDELETE(positionGroup, "/:id", "position:delete", s.handleDeletePosition)
			permissionGET(positionGroup, "/:id/roles", "position:list", s.handleGetPositionRoles)
			permissionPUT(positionGroup, "/:id/roles", "position:edit", s.handleSetPositionRoles)

			appGroup := protected.Group("/apps")
			appGroup.Middleware(TenantMiddleware())
			permissionGET(appGroup, "/", "app:list", s.handleListApps)
			permissionPOST(appGroup, "/", "app:create", s.handleCreateApp)
			permissionPUT(appGroup, "/:id", "app:edit", s.handleUpdateApp)
			permissionDELETE(appGroup, "/:id", "app:delete", s.handleDeleteApp)
			permissionPOST(appGroup, "/:id/reset-key", "app:reset-key", s.handleResetAppKey)

			auditGroup := protected.Group("/audit-logs")
			permissionGET(auditGroup, "/", "audit:list", s.handleListAuditLogs)

			protected.Group("/tenants", func(tenantGroup *ghttp.RouterGroup) {
				tenantGroup.Middleware(SystemAdminMiddleware())
				tenantGroup.GET("/", s.handleGetTenants)
				tenantGroup.POST("/", s.handleCreateTenant)
				tenantGroup.PUT("/:id", s.handleUpdateTenant)
				tenantGroup.DELETE("/:id", s.handleDeleteTenant)
				tenantGroup.POST("/:id/reset-password", s.handleResetTenantPassword)
			})

			protected.Group("/projects", func(projectGroup *ghttp.RouterGroup) {
				projectGroup.Middleware(TenantMiddleware())
				permissionGET(projectGroup, "/", "project:list", s.handleGetProjects)
				permissionPOST(projectGroup, "/", "project:create", s.handleCreateProject)
				permissionPUT(projectGroup, "/:id", "project:edit", s.handleUpdateProject)
				permissionDELETE(projectGroup, "/:id", "project:delete", s.handleDeleteProject)
			})
		})
	})
}
