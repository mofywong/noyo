package core

import (
	"noyo/core/store"
	"strconv"

	"github.com/gogf/gf/v2/net/ghttp"
)

// CanReadDeviceForPlugin applies the same tenant, project and tag permissions
// as the device API to plugin endpoints carrying camera evidence or results.
func CanReadDeviceForPlugin(r *ghttp.Request, deviceCode string) bool {
	if auth := RequestAuthContext(r); auth != nil && auth.SubjectType == "app" {
		device, err := store.GetDevice(deviceCode)
		permission := r.GetCtxVar("required_permission").String()
		if err != nil || (permission != "" && !auth.HasProjectPermission(permission, device.ProjectID)) {
			return false
		}
	}
	return canAccessDeviceCode(r, deviceCode)
}

// CanInspectDeviceAsUser revalidates a scheduled plan's grant without turning
// its service executor into that user. Scope and tag grants can be revoked.
func CanInspectDeviceAsUser(auth *AuthContext, tenantID, projectID uint, code string) bool {
	if auth == nil || !auth.HasProjectPermission("inspection:execute", projectID) || !auth.HasProjectPermission("device:control", projectID) || !auth.HasProjectPermission("device:list", projectID) {
		return false
	}
	device, err := store.GetDevice(code)
	if err != nil || !device.Enabled || device.TenantID != tenantID || device.ProjectID != projectID || (!auth.IsSystemAdmin && !auth.CanManageProject(projectID)) {
		return false
	}
	scope := store.GlobalAccessScope()
	if auth.TenantID != 0 {
		scope = store.AccessScope{Type: "tenant", ID: strconv.FormatUint(uint64(auth.TenantID), 10)}
	}
	read, err := canReadDeviceByTagPermission(auth, scope, code)
	if err != nil || !read {
		return false
	}
	write, err := canWriteDeviceByTagPermission(auth, scope, code)
	return err == nil && write
}

// CanWriteDeviceForPlugin adds the device API's tag write boundary and, for
// applications, checks the route permission against the device's real project.
func CanWriteDeviceForPlugin(r *ghttp.Request, deviceCode string) bool {
	return CanReadDeviceForPlugin(r, deviceCode) && (&Server{}).checkDeviceTagPermission(r, deviceCode) == nil
}
