package core

import (
	"errors"
	"strings"

	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// writeConfigMutationError returns true when a guard rejected the write and a
// stable, localizable API error has already been sent to the client.
func writeConfigMutationError(r *ghttp.Request, err error) bool {
	if !errors.Is(err, store.ErrGatewayMaintenance) && !errors.Is(err, store.ErrGatewayReadOnly) && !errors.Is(err, store.ErrSharedTemplateEdit) && !errors.Is(err, store.ErrConfigResourceInUse) && !errors.Is(err, store.ErrGatewayIdentityEdit) {
		return false
	}
	code := "GATEWAY_MAINTENANCE"
	zh := "网关正在本地维护或恢复正常模式，暂不接受平台对产品、驱动和设备的新增、修改或删除。请等待网关恢复正常模式后重试。其他功能不受影响。"
	en := "The gateway is in local maintenance or returning to normal mode. The platform cannot create, edit or delete its products, drivers and devices until normal mode returns. Other functions remain available."
	if errors.Is(err, store.ErrGatewayReadOnly) {
		code = "GATEWAY_READ_ONLY"
		zh = "网关未进入维护模式，不能新增、修改或删除产品、驱动和设备。请在网关顶部进入维护模式后重试。"
		en = "The gateway is not in maintenance mode. Products, drivers and devices cannot be created, edited or deleted. Enter maintenance mode at the top of the gateway console and retry."
	}
	if errors.Is(err, store.ErrSharedTemplateEdit) {
		code = "SHARED_TEMPLATE_EDIT"
		zh = "共享模板不能直接在网关上修改，请编辑本地派生实例。"
		en = "A shared template cannot be edited directly on the gateway. Edit a local instance instead."
	}
	if errors.Is(err, store.ErrGatewayIdentityEdit) {
		code = "GATEWAY_IDENTITY_EDIT"
		zh = "网关身份产品及驱动不能作为现场配置派生。"
		en = "The gateway identity product and driver cannot be forked as site configuration."
	}
	status := 423
	if errors.Is(err, store.ErrConfigResourceInUse) {
		status = 409
		code = "CONFIG_RESOURCE_IN_USE"
		zh = "此配置仍被设备引用，请先解除设备绑定。"
		en = "Devices still reference this configuration. Remove their bindings first."
	}
	message := zh
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		message = en
	}
	r.Response.WriteStatus(status, g.Map{"code": status, "error_code": code, "message": message, "message_zh": zh, "message_en": en})
	return true
}
