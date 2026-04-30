package sdputil

import (
	"fmt"
	"net"
)

// NetworkContext describes the network position of one side (platform or gateway).
type NetworkContext struct {
	HostIPs []string // All host candidate IPs (LAN IPs)
	LANIP   string   // Primary LAN IP for same-subnet detection
}

// MediaIPDecision holds the result of the media IP resolution.
type MediaIPDecision struct {
	MediaIP        string // The chosen IP to use in SDP offer to camera
	Reason         string // Human-readable reason
	IsSameLAN      bool   // Whether browser and platform are on same LAN
	ConnectionType string // "direct-host", "public"
}

// ResolveMediaIP compares browser ICE candidates with platform/gateway candidates
// and decides the optimal mediaIP for the SDP offer to the camera.
// LAN-only mode: only checks same-subnet detection, no public/srflx logic.
// Decision priority:
//  1. Same-LAN: browser host IP matches platform host IP on same subnet → use LAN IP
//  2. Fallback: use defaultMediaIP
func ResolveMediaIP(browser *ParsedSDPCandidates, platform *NetworkContext, defaultMediaIP, cameraIP string) *MediaIPDecision {
	if browser == nil || platform == nil {
		return &MediaIPDecision{
			MediaIP:        defaultMediaIP,
			Reason:         "nil browser or platform candidates, using default",
			ConnectionType: "public",
		}
	}

	// Priority 1: Same-LAN detection
	for _, bHost := range browser.HostCandidates {
		for _, pHost := range platform.HostIPs {
			if prefix := isSameSubnet(bHost.IP.String(), pHost); prefix > 0 {
				lanIP := platform.LANIP
				if lanIP == "" {
					lanIP = pHost
				}
				return &MediaIPDecision{
					MediaIP:        lanIP,
					Reason:         fmt.Sprintf("same-lan: browser host %s matches platform host %s on /%d", bHost.IP, pHost, prefix),
					IsSameLAN:      true,
					ConnectionType: "direct-host",
				}
			}
		}
	}

	// Fallback
	return &MediaIPDecision{
		MediaIP:        defaultMediaIP,
		Reason:         fmt.Sprintf("fallback: no same-lan match found (%d browser hosts, %d platform hosts), using default %s", len(browser.HostCandidates), len(platform.HostIPs), defaultMediaIP),
		ConnectionType: "public",
	}
}

// isSameSubnet checks whether two IP addresses are on the same subnet,
// trying from most specific (/24) to least specific (/8).
// Returns the matching prefix length, or 0 if not on the same subnet.
func isSameSubnet(ip1, ip2 string) int {
	for _, prefix := range []int{24, 16, 12, 8} {
		if isSameSubnetWithPrefix(ip1, ip2, prefix) {
			return prefix
		}
	}
	return 0
}

func isSameSubnetWithPrefix(ip1, ip2 string, prefixLen int) bool {
	a := net.ParseIP(ip1)
	b := net.ParseIP(ip2)
	if a == nil || b == nil {
		return false
	}
	a4 := a.To4()
	b4 := b.To4()
	if a4 == nil || b4 == nil {
		return false
	}
	mask := net.CIDRMask(prefixLen, 32)
	return a4.Mask(mask).Equal(b4.Mask(mask))
}
