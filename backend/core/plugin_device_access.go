package core

import "github.com/gogf/gf/v2/net/ghttp"

// CanReadDeviceForPlugin applies the same tenant, project and tag permissions
// as the device API to plugin endpoints carrying camera evidence or results.
func CanReadDeviceForPlugin(r *ghttp.Request, deviceCode string) bool {
	return canAccessDeviceCode(r, deviceCode)
}
