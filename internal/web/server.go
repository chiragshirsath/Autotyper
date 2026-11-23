package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"autotyper/internal/adb"
	"autotyper/internal/auth"
	"autotyper/internal/state"

	"github.com/gorilla/websocket"
	"github.com/skip2/go-qrcode"
)

//go:embed assets/index.html
var indexHTML []byte

//go:embed assets/dashboard.html
var dashboardHTML []byte

//go:embed assets/static/*
var staticEmbed embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow cross-origin connection over USB/LAN
	},
	ReadBufferSize:  1024 * 64,
	WriteBufferSize: 1024 * 64,
}

type clientConn struct {
	ws         *websocket.Conn
	mu         sync.Mutex
	authorized bool
	ip         string
}

func (c *clientConn) send(msg state.WSMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(msg)
}

// Server serves HTTP and WebSocket endpoints.
type Server struct {
	port         int
	auth         *auth.Manager
	engine       *state.Engine
	clientsMu    sync.RWMutex
	clients      map[*clientConn]bool
	shutdownChan chan struct{}
	localIPs     []string
}

// NewServer initializes the Web & WebSocket server.
func NewServer(port int, authMgr *auth.Manager, shutdownChan chan struct{}) *Server {
	s := &Server{
		port:         port,
		auth:         authMgr,
		clients:      make(map[*clientConn]bool),
		shutdownChan: shutdownChan,
		localIPs:     findLocalIPs(),
	}

	s.engine = state.NewEngine(s.Broadcast)
	return s
}

// Broadcast sends a WebSocket message to all connected clients.
func (s *Server) Broadcast(msg state.WSMessage) {
	s.clientsMu.RLock()
	var targets []*clientConn
	for c := range s.clients {
		targets = append(targets, c)
	}
	s.clientsMu.RUnlock()

	for _, c := range targets {
		_ = c.send(msg)
	}
}

// Handler returns the HTTP mux for the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static assets
	staticSub, err := fs.Sub(staticEmbed, "assets/static")
	if err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	}

	// Routes
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/qr.png", s.handleQRCode)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)
	mux.HandleFunc("/ws", s.handleWebSocket)

	return mux
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(dashboardHTML)
}

func (s *Server) handleQRCode(w http.ResponseWriter, r *http.Request) {
	customIP := r.URL.Query().Get("ip")
	host := "localhost"
	if customIP != "" {
		host = customIP
	} else if len(s.localIPs) > 0 {
		host = s.localIPs[0]
	}
	pairingURL := fmt.Sprintf("http://%s:%d/?token=%s", host, s.port, s.auth.PairingToken)

	png, err := qrcode.Encode(pairingURL, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write(png)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	adbStatus := adb.Check()
	if adbStatus.Connected && !adb.IsReverseActive(s.port) {
		_ = adb.SetupReverse(s.port)
	}
	data := map[string]interface{}{
		"port":          s.port,
		"pairing_token": s.auth.PairingToken,
		"pairing_pin":   s.auth.PairingPIN,
		"local_ips":     s.localIPs,
		"adb":           adbStatus,
		"devices":       s.auth.GetSessions(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	clientIP := auth.ExtractClientIP(r)

	isLocal := strings.HasPrefix(clientIP, "127.0.0.1") || strings.HasPrefix(clientIP, "::1") || clientIP == "localhost"
	if !isLocal && !s.auth.IsAdminAuthorized(token) && !s.auth.IsAuthorized(token) {
		http.Error(w, "Forbidden: unauthorized shutdown request", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})

	go func() {
		time.Sleep(200 * time.Millisecond)
		select {
		case s.shutdownChan <- struct{}{}:
		default:
		}
	}()
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	authorized := s.auth.IsAuthorized(token)
	clientIP := auth.ExtractClientIP(r)
	userAgent := r.UserAgent()

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed: %v", err)
		return
	}

	sessionID := fmt.Sprintf("%s-%d", clientIP, time.Now().UnixNano())
	s.auth.RegisterSession(sessionID, clientIP, userAgent, authorized)

	client := &clientConn{
		ws:         ws,
		authorized: authorized,
		ip:         clientIP,
	}

	s.clientsMu.Lock()
	s.clients[client] = true
	numClients := len(s.clients)
	s.clientsMu.Unlock()

	authIcon := "🔒"
	if !authorized {
		authIcon = "⚠️ Unpaired"
	}
	log.Printf("📱 Device connected from %s [%s] (Total connected: %d)", clientIP, authIcon, numClients)

	// Send initial synchronization state
	settings := s.engine.GetSettings()
	_ = client.send(state.WSMessage{
		Type:       "sync_state",
		Authorized: authorized,
		State:      &settings,
	})

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, client)
		remaining := len(s.clients)
		s.clientsMu.Unlock()

		s.auth.UnregisterSession(sessionID)
		_ = ws.Close()

		log.Printf("📱 Device disconnected (%s). Remaining: %d", clientIP, remaining)
		if remaining == 0 {
			s.engine.Stop()
		}
	}()

	for {
		var msg state.WSMessage
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "update_code":
			if !client.authorized {
				_ = client.send(state.WSMessage{
					Type:    "error",
					Message: "Unauthorized: Please scan the QR code on the laptop screen or enter the pairing PIN",
				})
				continue
			}
			s.engine.UpdateSettings(msg, clientIP)

		case "punch":
			if !client.authorized {
				_ = client.send(state.WSMessage{
					Type:    "error",
					Message: "Unauthorized: Please scan the QR code on the laptop screen or enter the pairing PIN to start typing",
				})
				continue
			}
			if err := s.engine.StartPunch(msg, clientIP); err != nil {
				_ = client.send(state.WSMessage{
					Type:    "error",
					Message: err.Error(),
				})
			}

		case "stop":
			s.engine.Stop()

		case "ping":
			_ = client.send(state.WSMessage{Type: "pong"})
		}
	}
}

func findLocalIPs() []string {
	var ips []string
	seen := make(map[string]bool)

	// 1. Primary outbound LAN IP (routes through the active Wi-Fi / Ethernet default gateway)
	primary := getPrimaryOutboundIP()
	if primary != "" && !seen[primary] {
		ips = append(ips, primary)
		seen[primary] = true
	}

	// 2. Scan physical network interfaces, filtering out virtual / container / hypervisor switches
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			name := strings.ToLower(iface.Name)
			if strings.Contains(name, "vethernet") ||
				strings.Contains(name, "wsl") ||
				strings.Contains(name, "hyper-v") ||
				strings.Contains(name, "virtualbox") ||
				strings.Contains(name, "vmware") ||
				strings.Contains(name, "docker") ||
				strings.Contains(name, "loopback") ||
				iface.Flags&net.FlagUp == 0 ||
				iface.Flags&net.FlagLoopback != 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, a := range addrs {
				if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					if ip4 := ipNet.IP.To4(); ip4 != nil {
						ipStr := ip4.String()
						// Ignore link-local APIPA addresses
						if strings.HasPrefix(ipStr, "169.254.") {
							continue
						}
						if !seen[ipStr] {
							ips = append(ips, ipStr)
							seen[ipStr] = true
						}
					}
				}
			}
		}
	}

	return ips
}

func getPrimaryOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
		if ok && localAddr.IP.To4() != nil {
			return localAddr.IP.String()
		}
	}
	return ""
}
