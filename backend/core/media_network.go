package core

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
	"noyo/core/platform"
	"noyo/core/store"
)

const mediaNetworkKey = "media_network"

var mediaNetworkMu sync.Mutex

// MediaNetworkConfig is deployment-owned. It is never returned directly by an API.
type MediaNetworkConfig struct {
	StunURLs             string `json:"stun_urls"`
	TurnURLs             string `json:"turn_urls"`
	AuthMode             string `json:"auth_mode"`
	TurnUsername         string `json:"turn_username"`
	TurnPassword         string `json:"turn_password"`
	TurnSecret           string `json:"turn_secret"`
	CredentialTTLSeconds int    `json:"credential_ttl_seconds"`
	Revision             string `json:"revision"`
}

func (c *MediaNetworkConfig) Normalize() {
	c.StunURLs = strings.TrimSpace(c.StunURLs)
	c.TurnURLs = strings.TrimSpace(c.TurnURLs)
	if c.AuthMode == "" {
		c.AuthMode = "rest"
	}
	if c.CredentialTTLSeconds == 0 {
		c.CredentialTTLSeconds = 3600
	}
}

func (c MediaNetworkConfig) Validate() error {
	if c.AuthMode != "rest" && c.AuthMode != "static" {
		return fmt.Errorf("auth_mode must be rest or static")
	}
	if c.CredentialTTLSeconds < 60 || c.CredentialTTLSeconds > 86400 {
		return fmt.Errorf("credential_ttl_seconds must be between 60 and 86400")
	}
	if err := platform.ValidateICEURLs(c.StunURLs, false); err != nil {
		return err
	}
	if err := platform.ValidateICEURLs(c.TurnURLs, true); err != nil {
		return err
	}
	if strings.TrimSpace(c.TurnURLs) != "" {
		if c.AuthMode == "rest" && c.TurnSecret == "" {
			return fmt.Errorf("TURN shared secret is required")
		}
		if c.AuthMode == "static" && (c.TurnUsername == "" || c.TurnPassword == "") {
			return fmt.Errorf("TURN username and password are required")
		}
	}
	return nil
}

func (c MediaNetworkConfig) ICEServers(now time.Time, identity string) ([]platform.ICEServerConfig, int64, error) {
	c.Normalize()
	if err := c.Validate(); err != nil {
		return nil, 0, err
	}
	username, password := c.TurnUsername, c.TurnPassword
	var expires int64
	if c.TurnURLs != "" && c.AuthMode == "rest" {
		deadline := now.Add(time.Duration(c.CredentialTTLSeconds) * time.Second)
		expires = deadline.UnixMilli()
		username = fmt.Sprintf("%d:%s", deadline.Unix(), identity)
		mac := hmac.New(sha1.New, []byte(c.TurnSecret))
		_, _ = mac.Write([]byte(username))
		password = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	servers, err := platform.BuildICEServerConfigs(c.StunURLs, c.TurnURLs, username, password)
	return servers, expires, err
}

func (c MediaNetworkConfig) View(source string) map[string]interface{} {
	return map[string]interface{}{
		"stun_urls": c.StunURLs, "turn_urls": c.TurnURLs, "auth_mode": c.AuthMode,
		"turn_username": c.TurnUsername, "turn_password_configured": c.TurnPassword != "",
		"turn_secret_configured": c.TurnSecret != "", "credential_ttl_seconds": c.CredentialTTLSeconds,
		"revision": c.Revision, "source": source,
	}
}

// An environment override is a complete deployment configuration, never a mix
// of persisted passwords and environment server addresses.
func mediaNetworkEnvironment() (MediaNetworkConfig, bool, error) {
	keys := []string{"NOYO_WEBRTC_STUN_URLS", "NOYO_WEBRTC_TURN_URLS", "NOYO_WEBRTC_TURN_REST_SECRET", "NOYO_WEBRTC_TURN_TTL_SECONDS"}
	var configured bool
	for _, key := range keys {
		if _, present := os.LookupEnv(key); present {
			configured = true
		}
	}
	c := MediaNetworkConfig{StunURLs: os.Getenv(keys[0]), TurnURLs: os.Getenv(keys[1]), TurnSecret: os.Getenv(keys[2]), Revision: "environment"}
	if raw := os.Getenv(keys[3]); raw != "" {
		var err error
		c.CredentialTTLSeconds, err = strconv.Atoi(raw)
		if err != nil {
			return c, configured, fmt.Errorf("invalid NOYO_WEBRTC_TURN_TTL_SECONDS")
		}
	}
	c.Normalize()
	return c, configured, nil
}

func LoadMediaNetworkConfig() (MediaNetworkConfig, string, error) {
	if env, configured, err := mediaNetworkEnvironment(); configured {
		return env, "environment", err
	}
	c := MediaNetworkConfig{}
	if store.DB == nil {
		return c, "database", fmt.Errorf("media configuration storage unavailable")
	}
	var row store.SystemConfig
	err := store.DB.Where("key = ?", mediaNetworkKey).First(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return c, "database", err
	}
	if err == nil {
		if err := json.Unmarshal([]byte(row.Value), &c); err != nil {
			return c, "database", fmt.Errorf("invalid stored media configuration")
		}
	}
	c.Normalize()
	return c, "database", nil
}

func loadMediaNetworkConfig() (MediaNetworkConfig, string, error) {
	return LoadMediaNetworkConfig()
}

type mediaNetworkUpdate struct {
	MediaNetworkConfig
	ClearTurnSecret   bool `json:"clear_turn_secret"`
	ClearTurnPassword bool `json:"clear_turn_password"`
}

func saveMediaNetworkConfig(req mediaNetworkUpdate) (MediaNetworkConfig, error) {
	mediaNetworkMu.Lock()
	defer mediaNetworkMu.Unlock()
	previous, source, err := loadMediaNetworkConfig()
	if err != nil {
		return MediaNetworkConfig{}, err
	}
	if source == "environment" {
		return MediaNetworkConfig{}, fmt.Errorf("media network configuration is managed by environment variables")
	}
	c := req.MediaNetworkConfig
	if c.TurnSecret == "" && !req.ClearTurnSecret {
		c.TurnSecret = previous.TurnSecret
	}
	if c.TurnPassword == "" && !req.ClearTurnPassword {
		c.TurnPassword = previous.TurnPassword
	}
	if req.ClearTurnSecret {
		c.TurnSecret = ""
	}
	if req.ClearTurnPassword {
		c.TurnPassword = ""
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		return MediaNetworkConfig{}, err
	}
	c.Revision, err = newMediaSessionID()
	if err != nil {
		return MediaNetworkConfig{}, err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return MediaNetworkConfig{}, err
	}
	row := store.SystemConfig{Key: mediaNetworkKey, Value: string(data)}
	// The row contains shared secrets. Use a discard logger for this statement so
	// debug SQL logging can never interpolate the JSON into process logs.
	silentDB := store.DB.Session(&gorm.Session{Logger: gormlogger.Discard})
	if err := silentDB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"})}).Create(&row).Error; err != nil {
		return MediaNetworkConfig{}, fmt.Errorf("save media network configuration failed")
	}
	return c, nil
}
