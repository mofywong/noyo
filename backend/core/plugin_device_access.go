package core

import (
	"noyo/core/store"

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

// CanWriteDeviceForPlugin adds the device API's tag write boundary and, for
// applications, checks the route permission against the device's real project.
func CanWriteDeviceForPlugin(r *ghttp.Request, deviceCode string) bool {
	return CanReadDeviceForPlugin(r, deviceCode) && (&Server{}).checkDeviceTagPermission(r, deviceCode) == nil
}
