package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"autotyper/internal/adb"
	"autotyper/internal/auth"
	"autotyper/internal/web"
)

type LockInfo struct {
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	AdminToken string `json:"admin_token"`
}

func getLockFilePath() string {
	return filepath.Join(os.TempDir(), "autotyper.lock")
}

func getLogFilePath() string {
	return filepath.Join(os.TempDir(), "autotyper.log")
}

func isAlreadyRunning() (bool, int) {
	data, err := os.ReadFile(getLockFilePath())
	if err != nil {
		return false, 0
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return false, 0
	}
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", info.Port))
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return true, info.Port
	}
	return false, 0
}

func writeLockFile(port int, adminToken string) error {
	info := LockInfo{
		PID:        os.Getpid(),
		Port:       port,
		AdminToken: adminToken,
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getLockFilePath(), data, 0600)
}

func removeLockFile() {
	_ = os.Remove(getLockFilePath())
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func setupLogging() func() {
	logFile, err := os.OpenFile(getLogFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return func() {}
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	return func() {
		_ = logFile.Close()
	}
}

func main() {
	cleanupLogging := setupLogging()
	defer cleanupLogging()

	// 1. If an instance is already running, simply focus/reopen the dashboard and exit
	if running, port := isAlreadyRunning(); running {
		dashboardURL := fmt.Sprintf("http://localhost:%d/dashboard", port)
		log.Printf("ℹ️  Auto Typer is already running on port %d. Reopening dashboard: %s", port, dashboardURL)
		openBrowser(dashboardURL)
		return
	}

	log.Println("──────────────────────────────────────────────────────────")
	log.Println("  ⌨️  Auto Typer Server — Starting Background Service...")
	log.Println("──────────────────────────────────────────────────────────")

	// 2. Create Auth Manager
	authMgr, err := auth.New()
	if err != nil {
		log.Fatalf("❌ Failed to initialize auth manager: %v", err)
	}

	// 3. Select Port (default 8080, fallback if in use)
	port := 8080
	for p := 8080; p <= 8090; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p))
		if err == nil {
			_ = ln.Close()
			port = p
			break
		}
	}

	// 4. Save Lock File for stop.exe
	if err := writeLockFile(port, authMgr.AdminToken); err != nil {
		log.Printf("⚠️  Could not write lock file: %v", err)
	}
	defer removeLockFile()

	// 5. Check ADB & Setup Reverse Port Forwarding
	adbStatus := adb.Check()
	if adbStatus.Connected {
		log.Printf("🔌 ADB USB device detected (%s). Setting up reverse tunnel on port %d...", adbStatus.DeviceID, port)
		if err := adb.SetupReverse(port); err != nil {
			log.Printf("⚠️  ADB reverse setup warning: %v", err)
		} else {
			log.Printf("✓ ADB reverse forwarding active: phone can use http://localhost:%d", port)
		}
	} else if adbStatus.Installed {
		log.Printf("ℹ️  ADB is installed, but no device connected. Running in Wi-Fi mode.")
	} else {
		log.Printf("ℹ️  Running in Wi-Fi mode.")
	}
	defer adb.RemoveReverse()

	// 6. Initialize Web Server
	shutdownChan := make(chan struct{}, 1)
	serverInstance := web.NewServer(port, authMgr, shutdownChan)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: serverInstance.Handler(),
	}

	go func() {
		log.Printf("🌐 Auto Typer running at http://localhost:%d", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	dashboardURL := fmt.Sprintf("http://localhost:%d/dashboard", port)
	log.Printf("📱 Pairing Dashboard: %s", dashboardURL)
	log.Printf("🔑 Pairing PIN:        %s", authMgr.PairingPIN)
	log.Println("  💡 Moving mouse to screen corner activates Emergency Stop")
	stopCmd := "./stop"
	if runtime.GOOS == "windows" {
		stopCmd = "stop.exe"
	}
	log.Printf("  🛑 Run %s or click 'Stop Server' in dashboard to shut down", stopCmd)
	log.Println("──────────────────────────────────────────────────────────")

	// Open pairing dashboard automatically
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(dashboardURL)
	}()

	// 7. Handle Shutdown Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Println("\n👋 Received interrupt signal. Shutting down...")
	case <-shutdownChan:
		log.Println("\n🛑 Shutdown requested via stop command or dashboard...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(ctx)
	removeLockFile()
	adb.RemoveReverse()
	log.Println("✅ Auto Typer server cleanly stopped. Goodbye!")
}
