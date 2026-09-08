package core

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"noyo/core/platform"
	"noyo/core/store"
)

func TestMediaNetworkCredentialsAndValidation(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cfg := MediaNetworkConfig{StunURLs: "stun:example.org", TurnURLs: "turn:example.org?transport=tcp", TurnSecret: "shared-secret"}
	cfg.Normalize()
	if cfg.AuthMode != "rest" || cfg.CredentialTTLSeconds != 3600 {
		t.Fatalf("defaults=%+v", cfg)
	}
	servers, expires, err := cfg.ICEServers(now, "user-1-session-a")
	if err != nil || len(servers) != 2 || expires != now.Add(time.Hour).UnixMilli() {
		t.Fatalf("servers=%v expires=%d err=%v", servers, expires, err)
	}
	mac := hmac.New(sha1.New, []byte(cfg.TurnSecret))
	mac.Write([]byte(servers[1].Username))
	if servers[1].Credential != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("invalid TURN signature")
	}
	other, _, _ := cfg.ICEServers(now, "user-2-session-b")
	if other[1].Username == servers[1].Username {
		t.Fatal("session identities share credentials")
	}
	for _, url := range []string{"http://example.org", "turn:", "turn://example.org", "turn:user:pass@example.org", "turn:example.org?transport=invalid", "turn:example.org:70000"} {
		bad := cfg
		bad.TurnURLs = url
		if bad.Validate() == nil {
			t.Fatalf("accepted invalid TURN URL %q", url)
		}
	}
	for _, ttl := range []int{-1, 59, 86401} {
		bad := cfg
		bad.CredentialTTLSeconds = ttl
		if bad.Validate() == nil {
			t.Fatalf("accepted TTL %d", ttl)
		}
	}
	empty := MediaNetworkConfig{}
	empty.Normalize()
	ice, expiry, err := empty.ICEServers(now, "id")
	if err != nil || ice == nil || len(ice) != 0 || expiry != 0 {
		t.Fatalf("host-only=%v %d %v", ice, expiry, err)
	}
}

func setupMediaNetworkDB(t *testing.T) *gorm.DB {
	t.Helper()
	for _, key := range []string{"NOYO_WEBRTC_STUN_URLS", "NOYO_WEBRTC_TURN_URLS", "NOYO_WEBRTC_TURN_REST_SECRET", "NOYO_WEBRTC_TURN_TTL_SECONDS"} {
		old, present := os.LookupEnv(key)
		os.Unsetenv(key)
		t.Cleanup(func() {
			if present {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	previous := store.DB
	store.DB = db
	t.Cleanup(func() { store.DB = previous; sqlDB.Close() })
	if err := db.AutoMigrate(&store.SystemConfig{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMediaNetworkPersistenceSecretRetentionAndEnvironment(t *testing.T) {
	db := setupMediaNetworkDB(t)
	first, err := saveMediaNetworkConfig(mediaNetworkUpdate{MediaNetworkConfig: MediaNetworkConfig{TurnURLs: "turn:relay.example", TurnSecret: "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, source, err := loadMediaNetworkConfig()
	if err != nil || loaded.TurnSecret != "secret" || source != "database" || loaded.Revision == "" {
		t.Fatalf("load failed %v %s", err, source)
	}
	updated, err := saveMediaNetworkConfig(mediaNetworkUpdate{MediaNetworkConfig: MediaNetworkConfig{TurnURLs: "turn:relay2.example"}})
	if err != nil || updated.TurnSecret != "secret" || updated.Revision == first.Revision {
		t.Fatal("blank field failed to preserve stored secret or update revision")
	}
	if _, err := saveMediaNetworkConfig(mediaNetworkUpdate{MediaNetworkConfig: MediaNetworkConfig{TurnURLs: "turn:relay2.example"}, ClearTurnSecret: true}); err == nil {
		t.Fatal("accepted enabled TURN without secret")
	}
	cleared, err := saveMediaNetworkConfig(mediaNetworkUpdate{ClearTurnSecret: true})
	if err != nil || cleared.TurnSecret != "" {
		t.Fatal("explicit clear failed")
	}
	t.Setenv("NOYO_WEBRTC_STUN_URLS", "stun:deployment.example")
	env, source, err := loadMediaNetworkConfig()
	if err != nil || source != "environment" || env.StunURLs != "stun:deployment.example" || env.TurnSecret != "" {
		t.Fatal("environment should replace whole config")
	}
	if _, err := saveMediaNetworkConfig(mediaNetworkUpdate{}); err == nil {
		t.Fatal("environment-managed config was editable")
	}
	os.Unsetenv("NOYO_WEBRTC_STUN_URLS")
	if err := db.Exec("CREATE TRIGGER reject_media_update BEFORE UPDATE ON system_configs BEGIN SELECT RAISE(ABORT, 'read only'); END").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := saveMediaNetworkConfig(mediaNetworkUpdate{MediaNetworkConfig: MediaNetworkConfig{StunURLs: "stun:unsaved.example"}}); err == nil {
		t.Fatal("save failure not reported")
	}
	after, _, err := loadMediaNetworkConfig()
	if err != nil || after.StunURLs != cleared.StunURLs {
		t.Fatal("failed save changed effective config")
	}
}

func TestMediaNetworkAdminAPIAndRedaction(t *testing.T) {
	setupMediaNetworkDB(t)
	s := &Server{}
	web := ghttp.GetServer(t.Name())
	web.SetAddr("127.0.0.1:0")
	web.SetDumpRouterMap(false)
	web.Group("/api/system", func(g *ghttp.RouterGroup) {
		g.Middleware(func(r *ghttp.Request) {
			r.SetCtxVar(authContextKey, &AuthContext{UserID: 1, IsSystemAdmin: r.GetHeader("X-Admin") == "yes", PermissionCodes: map[string]bool{"system:config": true}})
			r.Middleware.Next()
		})
		permissionGET(g, "/media-network", "system:config", s.handleGetMediaNetwork)
		permissionPUT(g, "/media-network", "system:config", s.handleUpdateMediaNetwork)
	})
	if err := web.Start(); err != nil {
		t.Fatal(err)
	}
	defer web.Shutdown()
	for _, tc := range []struct {
		method string
		admin  bool
		body   string
		want   int
	}{
		{"GET", false, "", 403}, {"PUT", false, `{"turn_secret":"forbidden"}`, 403},
		{"PUT", true, `{"turn_urls":"turn:example.org","turn_secret":"private-shared-secret"}`, 0},
		{"GET", true, "", 0},
	} {
		req, _ := http.NewRequest(tc.method, fmt.Sprintf("http://127.0.0.1:%d/api/system/media-network", web.GetListenedPort()), bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		if tc.admin {
			req.Header.Set("X-Admin", "yes")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		err = json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if err != nil || body.Code != tc.want {
			t.Fatalf("%+v result=%+v err=%v", tc, body, err)
		}
		if bytes.Contains(body.Data, []byte("private-shared-secret")) || bytes.Contains(body.Data, []byte(`"turn_secret":`)) {
			t.Fatal("API leaked secret")
		}
		if tc.want == 0 && res.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("missing no-store")
		}
	}
}

func TestPlaybackInjectionUsesOwnedSnapshotNotBrowserCredentials(t *testing.T) {
	s := &Server{MediaSessions: NewMediaSessionManager()}
	session, _ := s.MediaSessions.CreateWithICE("camera", "gateway", 1, []platform.ICEServerConfig{})
	session.PlaybackOnly = true
	params := map[string]interface{}{"playback_id": session.ID, "media_ice_servers": "untrusted", "media_session_id": "forged"}
	if err := s.applyPlaybackNetwork("camera", 1, params); err != nil {
		t.Fatal(err)
	}
	servers, ok := params["media_ice_servers"].([]platform.ICEServerConfig)
	if !ok || servers == nil || len(servers) != 0 {
		t.Fatal("explicit empty snapshot not forwarded")
	}
	if _, exists := params["media_session_id"]; exists {
		t.Fatal("untrusted trickle session forwarded")
	}
	for _, tc := range []struct {
		user   uint
		device string
		id     string
	}{{2, "camera", session.ID}, {1, "other", session.ID}, {1, "camera", ""}} {
		params := map[string]interface{}{"playback_id": tc.id, "media_ice_servers": "forged"}
		if err := s.applyPlaybackNetwork(tc.device, tc.user, params); err == nil {
			t.Fatal("unauthorized playback accepted")
		}
		if _, exists := params["media_ice_servers"]; exists {
			t.Fatal("untrusted ICE survived rejection")
		}
	}
}

func TestMediaNetworkSecretsNeverAppearInView(t *testing.T) {
	cfg := MediaNetworkConfig{TurnSecret: "shared-secret", TurnPassword: "static-password"}
	data, _ := json.Marshal(cfg.View("database"))
	if strings.Contains(string(data), "shared-secret") || strings.Contains(string(data), "static-password") {
		t.Fatal("secret leaked")
	}
	if !strings.Contains(string(data), "turn_secret_configured\":true") {
		t.Fatalf("missing secret indicator %s", data)
	}
}

func TestPlaybackSnapshotBoundToUserDeviceAndExpiry(t *testing.T) {
	m := NewMediaSessionManager()
	now := time.Unix(1700000000, 0)
	m.now = func() time.Time { return now }
	servers := []platform.ICEServerConfig{{URLs: []string{"stun:original.example"}}}
	s, err := m.CreateWithICE("camera", "", 1, servers)
	if err != nil {
		t.Fatal(err)
	}
	servers[0].URLs[0] = "stun:changed.example"
	if s.ICEServers[0].URLs[0] != "stun:original.example" {
		t.Fatal("snapshot mutated")
	}
	for _, tc := range []struct {
		user   uint
		device string
	}{{2, "camera"}, {1, "other"}} {
		if _, err := m.GetPlayback(s.ID, tc.user, tc.device); err == nil {
			t.Fatal("foreign snapshot accepted")
		}
	}
	if _, err := m.GetPlayback(s.ID, 1, "camera"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Minute)
	if _, err := m.GetPlayback(s.ID, 1, "camera"); err == nil {
		t.Fatal("expired snapshot accepted")
	}
}
