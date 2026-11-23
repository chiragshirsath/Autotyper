package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DeviceSession holds information about a connected device.
type DeviceSession struct {
	ID         string    `json:"id"`
	RemoteAddr string    `json:"remote_addr"`
	UserAgent  string    `json:"user_agent"`
	Connected  time.Time `json:"connected_at"`
	Authorized bool      `json:"authorized"`
}

// Manager handles pairing authentication and authorization.
type Manager struct {
	mu           sync.RWMutex
	PairingToken string `json:"pairing_token"`
	PairingPIN   string `json:"pairing_pin"`
	AdminToken   string `json:"-"`
	sessions     map[string]*DeviceSession
}

// New creates a new Auth Manager with random tokens and PIN.
func New() (*Manager, error) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("crypto rand failed: %w", err)
	}
	adminBytes := make([]byte, 16)
	if _, err := rand.Read(adminBytes); err != nil {
		return nil, fmt.Errorf("crypto rand failed: %w", err)
	}

	pinNum, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return nil, fmt.Errorf("crypto rand PIN failed: %w", err)
	}
	pin := fmt.Sprintf("%06d", pinNum.Int64()+100000)

	return &Manager{
		PairingToken: hex.EncodeToString(tokenBytes),
		PairingPIN:   pin,
		AdminToken:   hex.EncodeToString(adminBytes),
		sessions:     make(map[string]*DeviceSession),
	}, nil
}

// IsAuthorized checks if the provided token or PIN matches the pairing credentials.
func (m *Manager) IsAuthorized(input string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cleaned := strings.TrimSpace(input)
	if cleaned == "" {
		return false
	}
	return cleaned == m.PairingToken || cleaned == m.PairingPIN
}

// IsAdminAuthorized checks if the provided admin token matches.
func (m *Manager) IsAdminAuthorized(input string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cleaned := strings.TrimSpace(input)
	if cleaned == "" {
		return false
	}
	return cleaned == m.AdminToken
}

// RegisterSession registers a client connection and its authorization status.
func (m *Manager) RegisterSession(id, remoteAddr, userAgent string, authorized bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = &DeviceSession{
		ID:         id,
		RemoteAddr: remoteAddr,
		UserAgent:  userAgent,
		Connected:  time.Now(),
		Authorized: authorized,
	}
}

// UpdateSessionAuth updates the authorization status of an active session.
func (m *Manager) UpdateSessionAuth(id string, authorized bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, exists := m.sessions[id]; exists {
		s.Authorized = authorized
	}
}

// UnregisterSession removes a client connection.
func (m *Manager) UnregisterSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// GetSessions returns a snapshot of currently connected devices.
func (m *Manager) GetSessions() []*DeviceSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*DeviceSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// ExtractClientIP extracts the client IP from the request.
func ExtractClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return ip
	}
	return r.RemoteAddr
}
