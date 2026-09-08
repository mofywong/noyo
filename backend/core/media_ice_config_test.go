package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"noyo/core/platform"
	"noyo/core/store"
	"noyo/core/types"

	"github.com/glebarez/sqlite"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

type playbackICEPlugin struct{ platform.BasePlatformPlugin }

type invalidICEConfig struct {
	Port int `yaml:"port"`
}

func (invalidICEConfig) Validate() error { return fmt.Errorf("invalid old configuration") }

type invalidICEPlugin struct {
	platform.BasePlatformPlugin
	Config invalidICEConfig
}

func TestInvalidOldConfigurationCanStillBeDisabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	previousDB := store.DB
	store.DB = db
	defer func() { store.DB = previousDB }()
	if err := db.AutoMigrate(&store.PluginModel{}); err != nil {
		t.Fatal(err)
	}
	p := &invalidICEPlugin{BasePlatformPlugin: platform.BasePlatformPlugin{Enabled: true, Meta: &types.PluginMeta{Name: "test-ice"}}}
	if err := UpdatePluginConfig(p, map[string]interface{}{"enabled": false}); err != nil {
		t.Fatal(err)
	}
	if p.Enabled {
		t.Fatal("plugin was not disabled")
	}
	if err := UpdatePluginConfig(p, map[string]interface{}{"enabled": true}); err == nil {
		t.Fatal("invalid plugin was enabled")
	}
}

func (*playbackICEPlugin) GetPlaybackICEServers() ([]platform.ICEServerConfig, error) {
	return []platform.ICEServerConfig{{URLs: []string{"turn:example.org"}, Username: "test", Credential: "secret"}}, nil
}

func TestPlatformPlaybackICEConfigUsesCentralizedConfig(t *testing.T) {
	s := &Server{MediaSessions: NewMediaSessionManager()}
	s.Manager = NewPluginManager(s)
	servers, err := s.devicePlaybackICEServers(&store.Device{ParentCode: "gateway"})
	if err != nil || servers != nil {
		t.Fatalf("servers=%v err=%v", servers, err)
	}
}

func TestPlaybackICEConfigRequiresDeviceAccessNotPluginList(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	previousDB := store.DB
	store.DB = db
	defer func() { store.DB = previousDB }()
	if err := db.AutoMigrate(&store.Device{}); err != nil {
		t.Fatal(err)
	}
	for _, d := range []store.Device{{Code: "camera", TenantID: 1, ProjectID: 1, Enabled: true}, {Code: "other", TenantID: 2, ProjectID: 2, Enabled: true}, {Code: "other-project", TenantID: 1, ProjectID: 2, Enabled: true}, {Code: "disabled", TenantID: 1, ProjectID: 1, Enabled: false}} {
		if err := db.Create(&d).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{MediaSessions: NewMediaSessionManager()}
	s.Manager = NewPluginManager(s)
	plugin := &playbackICEPlugin{}
	plugin.Enabled = true
	s.Manager.PlatformPlugins["webrtc"] = plugin
	web := ghttp.GetServer(t.Name())
	web.SetAddr("127.0.0.1:0")
	web.SetDumpRouterMap(false)
	web.Group("/api", func(group *ghttp.RouterGroup) {
		group.Middleware(func(r *ghttp.Request) {
			permissions := map[string]bool{"device:control": true}
			if r.GetHeader("X-No-Control") != "" {
				permissions = map[string]bool{}
			}
			r.SetCtxVar(authContextKey, &AuthContext{UserID: 1, TenantID: 1, ProjectID: 1, IsProjectAdmin: true, AllowedProjectIDs: []uint{1}, UsesProjectLimits: true, PermissionCodes: permissions})
			r.Middleware.Next()
		})
		s.RegisterDeviceRoutes(group)
	})
	if err := web.Start(); err != nil {
		t.Fatal(err)
	}
	defer web.Shutdown()
	for _, tc := range []struct {
		code      string
		noControl bool
		want      int
	}{{"camera", false, 0}, {"other", false, 403}, {"other-project", false, 403}, {"disabled", false, 403}, {"camera", true, 403}} {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/devices/%s/media-ice-config", web.GetListenedPort(), tc.code), nil)
		if tc.noControl {
			req.Header.Set("X-No-Control", "1")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Code int `json:"code"`
			Data struct {
				Servers []platform.ICEServerConfig `json:"ice_servers"`
			} `json:"data"`
		}
		err = json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if err != nil || body.Code != tc.want {
			t.Fatalf("%+v result=%+v err=%v", tc, body, err)
		}
		if tc.want == 0 && (len(body.Data.Servers) != 1 || res.Header.Get("Cache-Control") != "no-store") {
			t.Fatal("missing ICE config or cache protection")
		}
		if tc.want != 0 && len(body.Data.Servers) != 0 {
			t.Fatal("unauthorized ICE credentials leaked")
		}
	}
}
