package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type LockInfo struct {
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	AdminToken string `json:"admin_token"`
}

func getLockFilePath() string {
	return filepath.Join(os.TempDir(), "autotyper.lock")
}

func main() {
	fmt.Println("──────────────────────────────────────────────────────────")
	fmt.Println("  🛑 Auto Typer — Stop Utility")
	fmt.Println("──────────────────────────────────────────────────────────")

	lockPath := getLockFilePath()
	data, err := os.ReadFile(lockPath)

	if err != nil {
		fmt.Println("ℹ️  No active server lock file found.")
		fmt.Println("Checking for running background server processes...")
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/F", "/IM", "server.exe").Run()
			_ = exec.Command("adb", "reverse", "--remove-all").Run()
		} else {
			_ = exec.Command("pkill", "-f", "server").Run()
			_ = exec.Command("adb", "reverse", "--remove-all").Run()
		}
		fmt.Println("✅ Cleanup complete.")
		pauseBeforeExit()
		return
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		fmt.Printf("⚠️  Corrupt lock file: %v. Cleaning up...\n", err)
		_ = os.Remove(lockPath)
		pauseBeforeExit()
		return
	}

	fmt.Printf("📡 Stopping Auto Typer server (PID: %d, Port: %d)...\n", info.PID, info.Port)

	// Send HTTP graceful shutdown request
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/shutdown?token=%s", info.Port, info.AdminToken)
	req, _ := http.NewRequest("POST", url, nil)
	resp, err := client.Do(req)

	if err == nil && resp.StatusCode == http.StatusOK {
		fmt.Println("✓ Graceful shutdown signal delivered successfully.")
	} else {
		if err != nil {
			fmt.Printf("⚠️  HTTP shutdown failed (%v), terminating process directly...\n", err)
		} else {
			fmt.Printf("⚠️  HTTP shutdown returned status %d, terminating process directly...\n", resp.StatusCode)
		}
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/F", "/PID", strconv.Itoa(info.PID)).Run()
		} else {
			proc, err := os.FindProcess(info.PID)
			if err == nil {
				_ = proc.Kill()
			}
		}
	}

	// Clean up ADB reverse tunnels
	fmt.Println("🔌 Cleaning up ADB reverse port forwards...")
	_ = exec.Command("adb", "reverse", "--remove-all").Run()

	// Remove lock file
	_ = os.Remove(lockPath)

	fmt.Println("──────────────────────────────────────────────────────────")
	fmt.Println("✅ Auto Typer server stopped successfully!")
	fmt.Println("──────────────────────────────────────────────────────────")

	pauseBeforeExit()
}

func pauseBeforeExit() {
	time.Sleep(1500 * time.Millisecond)
}
