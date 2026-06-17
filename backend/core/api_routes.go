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
			authGroup.POST("/app-token", s.handleIssueAppToken)
			authGroup.POST("/app-refresh", s.handleRefreshAppToken)
			authGroup.GET("/tenant-info", s.handleGetTenantBySuffix)
		})

		group.Group("/setup", func(setupGroup *ghttp.RouterGroup) {
			setupGroup.GET("/status", s.handleGetSetupStatus)
			setupGroup.POST("/apply", s.handleApplySetup)
		})

		group.Group("/", func(protected *ghttp.RouterGroup) {
			protected.Middleware(AuthMiddleware(s.Config.Auth.JWTSecret), AuditMiddleware())

			protected.POST("/auth/logout", s.handleLogout)
			protected.GET("/auth/profile", s.handleGetProfile)
			protected.GET("/auth/projects", s.handleGetAccessibleProjects)
			protected.PUT("/auth/password", s.handleChangePassword)

			pluginGroup := protected.Group("/plugins")
			permissionGET(pluginGroup, "/", "plugin:list", s.handleListPlugins)
			permissionGET(pluginGroup, "/:name", "plugin:list", s.handleGetPlugin)
			permissionGET(pluginGroup, "/:name/schemas", "plugin:list", s.handleGetPluginSchemas)
			permissionPOST(pluginGroup, "/:name/config", "plugin:config", s.handleUpdatePluginConfig)
			permissionPOST(pluginGroup, "/:name/discover", "plugin:config", s.handlePluginDiscover)

			systemGroup := protected.Group("/system")
			permissionGET(systemGroup, "/stats", "dashboard:view", s.handleSystemStats)
			permissionGET(systemGroup, "/config", "system:config", s.handleGetSystemConfig)
			permissionPOST(systemGroup, "/config", "system:config", s.handleUpdateSystemConfig)
			permissionGET(systemGroup, "/log/config", "system:logs", s.handleGetLogConfig)
			permissionPOST(systemGroup, "/log/config", "system:logs", s.handleUpdateLogConfig)
			permissionGET(systemGroup, "/log/files", "system:logs", s.handleListLogFiles)
			permissionGET(systemGroup, "/log/file", "system:logs", s.handleReadLogFile)
			permissionGET(systemGroup, "/log/tail", "system:logs", s.handleTailLog)
			permissionGET(systemGroup, "/log/download", "system:logs", s.handleDownloadLogFile)
			permissionGroup(systemGroup, "system:logs").ALL("/log/stream", s.handleRealtimeLogs)

			tenantProtectedGroup := protected.Group("/")
			tenantProtectedGroup.Middleware(TenantMiddleware())
			permissionPOST(tenantProtectedGroup, "/history/query", "device:list", s.handleQueryHistory)
			permissionDELETE(tenantProtectedGroup, "/history/record", "history:delete", s.handleDeleteRecord)
			permissionPOST(tenantProtectedGroup, "/system/upload/image", "device:upload", s.handleUploadImage)
			s.RegisterDeviceRoutes(tenantProtectedGroup)
			s.RegisterRuleRoutes(tenantProtectedGroup)

			protected.Group("/users", func(userGroup *ghttp.RouterGroup) {
				userGroup.Middleware(TenantMiddleware())
				permissionGET(userGroup, "/", "user:list", s.handleListUsers)
				permissionPOST(userGroup, "/", "user:create", s.handleCreateUser)
				permissionPUT(userGroup, "/:id", "user:edit", s.handleUpdateUser)
				permissionDELETE(userGroup, "/:id", "user:delete", s.handleDeleteUser)
				permissionPOST(userGroup, "/:id/reset-password", "user:edit", s.handleResetPassword)
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

			appGroup := protected.Group("/apps")
			appGroup.Middleware(TenantMiddleware())
			permissionGET(appGroup, "/", "app:list", s.handleListApps)
			permissionPOST(appGroup, "/", "app:create", s.handleCreateApp)
			permissionPUT(appGroup, "/:id", "app:edit", s.handleUpdateApp)
			permissionDELETE(appGroup, "/:id", "app:delete", s.handleDeleteApp)
			permissionPOST(appGroup, "/:id/reset-key", "app:reset-key", s.handleResetAppKey)
			permissionGET(appGroup, "/access-options", "app:edit", s.handleGetAppAccessOptions)
			permissionGET(appGroup, "/:id/access", "app:list", s.handleGetAppAccess)
			permissionPUT(appGroup, "/:id/access", "app:edit", s.handleSetAppAccess)

			auditGroup := protected.Group("/audit-logs")
			permissionGET(auditGroup, "/", "audit:list", s.handleListAuditLogs)

			protected.Group("/tenants", func(tenantGroup *ghttp.RouterGroup) {
				permissionGET(tenantGroup, "/permission-options", "tenant:create", s.handleGetTenantPermissionOptions)
				permissionGET(tenantGroup, "/", "tenant:list", s.handleGetTenants)
				permissionPOST(tenantGroup, "/", "tenant:create", s.handleCreateTenant)
				permissionPUT(tenantGroup, "/:id", "tenant:edit", s.handleUpdateTenant)
				permissionDELETE(tenantGroup, "/:id", "tenant:delete", s.handleDeleteTenant)
				permissionPOST(tenantGroup, "/:id/reset-password", "tenant:edit", s.handleResetTenantPassword)
				permissionGET(tenantGroup, "/:id/users", "tenant:list", s.handleGetTenantUsers)
				permissionPOST(tenantGroup, "/:id/change-admin", "tenant:edit", s.handleChangeTenantAdmin)
			})

			protected.Group("/tenant-transfer", func(transferGroup *ghttp.RouterGroup) {
				transferGroup.Middleware(TenantMiddleware())
				permissionPOST(transferGroup, "/admin", "tenant:transfer", s.handleTransferTenantAdmin)
			})

			protected.Group("/projects", func(projectGroup *ghttp.RouterGroup) {
				projectGroup.Middleware(TenantMiddleware())
				permissionGET(projectGroup, "/permission-options", "project:edit", s.handleGetProjectPermissionOptions)
				permissionGET(projectGroup, "/", "project:list", s.handleGetProjects)
				permissionPOST(projectGroup, "/", "project:create", s.handleCreateProject)
				permissionPUT(projectGroup, "/:id", "project:edit", s.handleUpdateProject)
				permissionDELETE(projectGroup, "/:id", "project:delete", s.handleDeleteProject)
			})
		})
	})
}
