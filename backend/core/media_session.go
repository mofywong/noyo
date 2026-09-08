package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"noyo/core/platform"
)

const (
	mediaSessionLifetime = 2 * time.Minute
	mediaSignalLimit     = 128
)

// MediaSession is a short-lived browser-to-gateway signalling session. It is
// intentionally in-memory: it contains temporary TURN credentials and must
// not survive a process restart.
type MediaSession struct {
	ID                  string
	DeviceCode          string
	GatewayCode         string
	UserID              uint
	ICEServers          []platform.ICEServerConfig
	CreatedAt           time.Time
	ExpiresAt           time.Time
	CredentialExpiresAt int64
	ConfigSource        string
	ConfigRevision      string
	PlaybackOnly        bool

	mu                    sync.Mutex
	nextSeq               uint64
	signals               []sequencedMediaSignal
	started               bool
	pendingBrowserSignals []platform.MediaSignal
}

type sequencedMediaSignal struct {
	Sequence uint64               `json:"sequence"`
	Signal   platform.MediaSignal `json:"signal"`
}

type MediaSessionManager struct {
	mu       sync.Mutex
	sessions map[string]*MediaSession
	now      func() time.Time
}

func NewMediaSessionManager() *MediaSessionManager {
	return &MediaSessionManager{
		sessions: make(map[string]*MediaSession),
		now:      time.Now,
	}
}

func (m *MediaSessionManager) CreateWithICE(deviceCode, gatewayCode string, userID uint, servers []platform.ICEServerConfig) (*MediaSession, error) {
	if deviceCode == "" || userID == 0 {
		return nil, fmt.Errorf("device and user are required for a media session")
	}
	id, err := newMediaSessionID()
	if err != nil {
		return nil, err
	}
	now := m.now()
	session := &MediaSession{
		ID:          id,
		DeviceCode:  deviceCode,
		GatewayCode: gatewayCode,
		UserID:      userID,
		ICEServers:  cloneMediaICEServers(servers),
		CreatedAt:   now,
		ExpiresAt:   now.Add(mediaSessionLifetime),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeExpiredLocked(now)
	owned := 0
	for _, existing := range m.sessions {
		if existing.UserID == userID {
			owned++
		}
	}
	if owned >= 128 || len(m.sessions) >= 8192 {
		return nil, fmt.Errorf("too many pending media sessions")
	}
	m.sessions[id] = session
	return session, nil
}

func cloneMediaICEServers(servers []platform.ICEServerConfig) []platform.ICEServerConfig {
	result := make([]platform.ICEServerConfig, len(servers))
	for i, server := range servers {
		result[i] = server
		result[i].URLs = append([]string(nil), server.URLs...)
	}
	return result
}

func (m *MediaSessionManager) GetPlayback(id string, userID uint, deviceCode string) (*MediaSession, error) {
	session, err := m.GetOwned(id, userID)
	if err != nil || session.DeviceCode != deviceCode {
		return nil, fmt.Errorf("playback configuration not found or expired")
	}
	if session.CredentialExpiresAt != 0 && session.CredentialExpiresAt <= m.now().Add(10*time.Second).UnixMilli() {
		return nil, fmt.Errorf("playback credentials expired; request a new configuration")
	}
	return session, nil
}

func (m *MediaSessionManager) GetOwned(id string, userID uint) (*MediaSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeExpiredLocked(m.now())
	session := m.sessions[id]
	if session == nil || session.UserID != userID {
		return nil, fmt.Errorf("media session not found")
	}
	return session, nil
}

func (m *MediaSessionManager) Deliver(signal platform.MediaSignal) {
	if signal.SessionID == "" {
		return
	}
	m.mu.Lock()
	m.removeExpiredLocked(m.now())
	session := m.sessions[signal.SessionID]
	m.mu.Unlock()
	if session == nil {
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	session.nextSeq++
	session.signals = append(session.signals, sequencedMediaSignal{Sequence: session.nextSeq, Signal: signal})
	if len(session.signals) > mediaSignalLimit {
		session.signals = append([]sequencedMediaSignal(nil), session.signals[len(session.signals)-mediaSignalLimit:]...)
	}
}

func (m *MediaSessionManager) SignalsAfter(session *MediaSession, after uint64) []sequencedMediaSignal {
	session.mu.Lock()
	defer session.mu.Unlock()
	result := make([]sequencedMediaSignal, 0)
	for _, signal := range session.signals {
		if signal.Sequence > after {
			result = append(result, signal)
		}
	}
	return result
}

// QueueBrowserSignal holds early browser candidates until the gateway has
// created its PeerConnection. Trickle gathering often starts before the offer
// request reaches the gateway, so publishing immediately would lose exactly
// the host candidate that makes same-LAN playback fastest.
func (m *MediaSessionManager) QueueBrowserSignal(session *MediaSession, signal platform.MediaSignal) (ready bool) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.started {
		return true
	}
	if len(session.pendingBrowserSignals) < mediaSignalLimit {
		session.pendingBrowserSignals = append(session.pendingBrowserSignals, signal)
	}
	return false
}

func (m *MediaSessionManager) MarkStarted(session *MediaSession) []platform.MediaSignal {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.started {
		return nil
	}
	session.started = true
	pending := append([]platform.MediaSignal(nil), session.pendingBrowserSignals...)
	session.pendingBrowserSignals = nil
	return pending
}

func (m *MediaSessionManager) removeExpiredLocked(now time.Time) {
	for id, session := range m.sessions {
		if !now.Before(session.ExpiresAt) {
			delete(m.sessions, id)
		}
	}
}

func newMediaSessionID() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate media session id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
