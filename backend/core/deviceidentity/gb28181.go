package deviceidentity

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"strings"

	"noyo/core/store"
)

const gb28181CodePrefix = "g"

// GB28181DeviceCode returns a short, stable, platform-side identity for one
// SIP endpoint at one ingress. The raw SIP ID is intentionally kept only in
// the device configuration and is never embedded in the device code.
func GB28181DeviceCode(sipID, gatewayCode string) string {
	sipID = strings.TrimSpace(sipID)
	gatewayCode = strings.TrimSpace(gatewayCode)

	scope := "direct"
	if gatewayCode != "" {
		scope = "gateway"
	}
	sum := sha256.Sum256([]byte(scope + "\x00" + gatewayCode + "\x00" + sipID))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10])
	return gb28181CodePrefix + strings.ToLower(encoded)
}

// GB28181IngressParentCode returns the visible parent for an ingress. Direct
// registrations deliberately have no parent; gateway registrations belong to
// the gateway that actually delivered them.
func GB28181IngressParentCode(gatewayCode string) string {
	return strings.TrimSpace(gatewayCode)
}

// GB28181SIPID returns the configured protocol address used for SIP and
// gateway commands. Device codes are platform-only identities and must never
// be sent to a camera as a SIP address.
func GB28181SIPID(device *store.Device) string {
	if device == nil {
		return ""
	}
	var config map[string]interface{}
	if json.Unmarshal([]byte(device.Config), &config) == nil {
		if sipID, ok := config["sip_id"].(string); ok && strings.TrimSpace(sipID) != "" {
			return strings.TrimSpace(sipID)
		}
	}
	return ""
}

// SetGB28181SIPID stores the protocol address without discarding vendor or
// driver-specific configuration already attached to the device.
func SetGB28181SIPID(device *store.Device, sipID string) {
	if device == nil {
		return
	}
	config := make(map[string]interface{})
	_ = json.Unmarshal([]byte(device.Config), &config)
	config["sip_id"] = strings.TrimSpace(sipID)
	if encoded, err := json.Marshal(config); err == nil {
		device.Config = string(encoded)
	}
}
