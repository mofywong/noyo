package core

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"noyo/core/platform"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func (s *Server) handleDeviceMediaICEConfig(r *ghttp.Request) {
	if mediaSessionUserID(r) == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	device, err := store.GetDevice(r.Get("code").String())
	if err != nil || device == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !device.Enabled || !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if err := s.checkDeviceTagPermission(r, device.Code); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}
	session, err := s.createPlaybackNetwork(device, mediaSessionUserID(r), true)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 503, "message": err.Error()})
		return
	}
	r.Response.Header().Set("Cache-Control", "no-store")
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"ice_servers": session.ICEServers, "playback_id": session.ID,
		"credential_expires_at": session.CredentialExpiresAt,
		"config_source":         session.ConfigSource, "config_revision": session.ConfigRevision,
	}})
}

// Both SDP and Trickle flows resolve one immutable, server-owned configuration
// before either peer starts gathering. The browser cannot supply its own ICE list.
func (s *Server) createPlaybackNetwork(device *store.Device, userID uint, playbackOnly bool) (*MediaSession, error) {
	servers, err := s.devicePlaybackICEServers(device)
	if err != nil {
		return nil, err
	}
	source, revision := "local", ""
	var expires int64
	if servers == nil {
		provider := s.Manager.MediaNetworkProvider()
		if provider == nil {
			return nil, fmt.Errorf("platform media network provider is unavailable")
		}
		identity, err := newMediaSessionID()
		if err != nil {
			return nil, err
		}
		snapshot, err := provider.CreatePlatformMediaNetwork(fmt.Sprintf("%d-%s", userID, identity), time.Now())
		if err != nil {
			return nil, err
		}
		servers, expires = snapshot.ICEServers, snapshot.CredentialExpiresAt
		source, revision = snapshot.Source, snapshot.Revision
	}
	session, err := s.MediaSessions.CreateWithICE(device.Code, device.ParentCode, userID, servers)
	if err != nil {
		return nil, err
	}
	session.ConfigSource, session.ConfigRevision = source, revision
	session.CredentialExpiresAt, session.PlaybackOnly = expires, playbackOnly
	return session, nil
}

func (s *Server) applyPlaybackNetwork(deviceCode string, userID uint, params map[string]interface{}) error {
	// These are internal signaling fields; never pass browser-supplied values on.
	delete(params, "media_ice_servers")
	delete(params, "media_session_id")
	id, _ := params["playback_id"].(string)
	if id == "" {
		return fmt.Errorf("playback_id is required; request playback configuration first")
	}
	session, err := s.MediaSessions.GetPlayback(id, userID, deviceCode)
	if err != nil {
		return err
	}
	if !session.PlaybackOnly {
		return fmt.Errorf("invalid playback configuration")
	}
	params["media_ice_servers"] = cloneMediaICEServers(session.ICEServers)
	delete(params, "playback_id")
	return nil
}

func (s *Server) devicePlaybackICEServers(device *store.Device) ([]platform.ICEServerConfig, error) {
	if device.ParentCode != "" {
		// Platform web playback always uses the platform's centralized ICE
		// configuration. The resulting snapshot is delivered to the gateway as
		// part of the short-lived playback session.
		return nil, nil
	}
	for _, plugin := range s.Manager.GetPlatformPlugins() {
		if !plugin.IsEnabled() {
			continue
		}
		if provider, ok := plugin.(platform.IWebRTCICEConfigProvider); ok {
			return provider.GetPlaybackICEServers()
		}
	}
	if s.Manager.MediaNetworkProvider() == nil {
		return nil, fmt.Errorf("playback ICE configuration is unavailable")
	}
	return nil, fmt.Errorf("media plugin does not provide direct playback ICE configuration")
}

func (s *Server) handleCreateMediaSession(r *ghttp.Request) {
	userID, device, gatewayCode, router, ok := s.mediaSessionRequestContext(r)
	if !ok {
		return
	}
	session, err := s.createPlaybackNetwork(device, userID, false)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	// This registration is safe to repeat and makes reconnect/reload ordering
	// explicit: candidates are accepted only after a browser owns a session.
	router.SetMediaSignalHandler(s.MediaSessions.Deliver)
	_ = gatewayCode
	r.Response.Header().Set("Cache-Control", "no-store")
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"session_id":            session.ID,
		"ice_servers":           session.ICEServers,
		"expires_at":            session.ExpiresAt.UnixMilli(),
		"credential_expires_at": session.CredentialExpiresAt,
		"config_source":         session.ConfigSource, "config_revision": session.ConfigRevision,
	}})
}

func (s *Server) handleMediaSessionOffer(r *ghttp.Request) {
	userID := mediaSessionUserID(r)
	session, err := s.MediaSessions.GetOwned(r.Get("id").String(), userID)
	if err != nil || session.PlaybackOnly {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Media session not found"})
		return
	}
	// Recheck live device access before starting media; authorization may have
	// changed since this short-lived session was issued.
	device, loadErr := store.GetDevice(session.DeviceCode)
	if loadErr != nil || device == nil || !device.Enabled || !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if err := s.checkDeviceTagPermission(r, device.Code); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if _, err := s.MediaSessions.GetPlayback(session.ID, userID, device.Code); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	var req struct {
		SDPOffer string `json:"sdp_offer"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil || req.SDPOffer == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "sdp_offer is required"})
		return
	}
	result, err := s.DeviceManager.CallDeviceService(session.DeviceCode, "PlayRealTimeStream", map[string]interface{}{
		"sdp_offer":         req.SDPOffer,
		"media_session_id":  session.ID,
		"media_ice_servers": session.ICEServers,
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	router, ok := s.Manager.GetPlugin("cascade").(platform.IMediaSignalRouter)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Cascade media signalling is unavailable"})
		return
	}
	for _, signal := range s.MediaSessions.MarkStarted(session) {
		if err := router.PublishMediaSignal(session.GatewayCode, signal); err != nil {
			r.Response.WriteJson(g.Map{"code": 502, "message": fmt.Sprintf("send queued media signal: %v", err)})
			return
		}
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": result})
}

func (s *Server) handleMediaSessionCandidate(r *ghttp.Request) {
	userID := mediaSessionUserID(r)
	session, err := s.MediaSessions.GetOwned(r.Get("id").String(), userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Media session not found"})
		return
	}
	var req struct {
		Candidate       *platform.ICECandidate `json:"candidate"`
		EndOfCandidates bool                   `json:"end_of_candidates"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil || (req.Candidate == nil && !req.EndOfCandidates) {
		r.Response.WriteJson(g.Map{"code": 400, "message": "candidate or end_of_candidates is required"})
		return
	}
	router, ok := s.Manager.GetPlugin("cascade").(platform.IMediaSignalRouter)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Cascade media signalling is unavailable"})
		return
	}
	signal := platform.MediaSignal{
		SessionID:       session.ID,
		DeviceCode:      session.DeviceCode,
		Candidate:       req.Candidate,
		EndOfCandidates: req.EndOfCandidates,
	}
	if !s.MediaSessions.QueueBrowserSignal(session, signal) {
		r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"queued": true}})
		return
	}
	err = router.PublishMediaSignal(session.GatewayCode, signal)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 502, "message": fmt.Sprintf("send media signal: %v", err)})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleMediaSessionCandidates(r *ghttp.Request) {
	userID := mediaSessionUserID(r)
	session, err := s.MediaSessions.GetOwned(r.Get("id").String(), userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Media session not found"})
		return
	}
	after, _ := strconv.ParseUint(r.GetQuery("after").String(), 10, 64)
	waitMS, _ := strconv.Atoi(r.GetQuery("wait_ms", "1000").String())
	if waitMS < 0 {
		waitMS = 0
	}
	if waitMS > 1500 {
		waitMS = 1500
	}
	deadline := time.Now().Add(time.Duration(waitMS) * time.Millisecond)
	var signals []sequencedMediaSignal
	for {
		signals = s.MediaSessions.SignalsAfter(session, after)
		if len(signals) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	next := after
	if len(signals) > 0 {
		next = signals[len(signals)-1].Sequence
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"signals": signals, "next_sequence": next}})
}

func (s *Server) mediaSessionRequestContext(r *ghttp.Request) (uint, *store.Device, string, platform.IMediaSignalRouter, bool) {
	userID := mediaSessionUserID(r)
	if userID == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return 0, nil, "", nil, false
	}
	device, err := store.GetDevice(r.Get("code").String())
	if err != nil || device == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return 0, nil, "", nil, false
	}
	if !device.Enabled || !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return 0, nil, "", nil, false
	}
	if err := s.checkDeviceTagPermission(r, device.Code); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return 0, nil, "", nil, false
	}
	gatewayCode := device.ParentCode
	cascadePlugin := s.Manager.GetPlugin("cascade")
	router, routerOK := cascadePlugin.(platform.IMediaSignalRouter)
	gatewayRouter, gatewayOK := cascadePlugin.(platform.IGatewayRouter)
	if gatewayCode == "" || !routerOK || !gatewayOK || !gatewayRouter.IsGatewayDevice(gatewayCode) {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Device does not support gateway media signalling"})
		return 0, nil, "", nil, false
	}
	return userID, device, gatewayCode, router, true
}

func mediaSessionUserID(r *ghttp.Request) uint {
	if authCtx := requestAuthContext(r); authCtx != nil {
		return authCtx.UserID
	}
	return 0
}
