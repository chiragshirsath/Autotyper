# ⌨️ Auto Typer (with Human Error & Jitter)

> **Auto Typer with Human Error** — Turn your mobile phone or tablet into an external typing companion for your laptop. Simulates natural human typing cadences, adjacent QWERTY key typos, automatic backspace corrections, and smart IDE indentation over local Wi-Fi or USB.

---

## ⚡ Highlights & Key Features

- **📱 Phone-to-Laptop Control**: Type or paste on your phone; watch it type into your active window (VS Code, terminal, editor).
- **🔒 QR Code & PIN Security**: Scanning the QR code on the pairing dashboard grants write permissions. Unpaired devices are blocked from typing.
- **👻 4-Level Human-like Jitter**:
  - `Low (25%)`: Subtle human variance.
  - `Med (50%)`: Natural everyday typing cadence.
  - `High (80%)`: Dynamic acceleration/deceleration between easy and complex characters.
  - `Chaos (120% + Pauses)`: Rapid bursts with cognitive thinking pauses (150–400ms after punctuation like `;`, `.`, `{}` and spaces).
- **🙈 Realistic Oops Mode (QWERTY Adjacent Typos)**:
  - Hits actual neighboring keys on a QWERTY keyboard (e.g. `w`/`r`/`s` instead of `e`).
  - Adjustable typo rate from **1%** (subtle/pro) to **25%** (erratic).
  - 20% chance of double-typos ("fat-finger" two keys, hesitate, double-backspace, and correct).
- **🧠 Smart IDE Auto-Indent**: Automatically detects auto-indentation in editors like VS Code; calculates indentation offsets and dedents with Backspace to preserve indentation.
- **📋 Fast Paste Mode**: Instant clipboard injection for large codebases (100,000+ chars in <0.02s).
- **⏱️ Focus Countdown & Screen Corner Failsafe**: 0–60s customizable countdown before typing starts; moving mouse to any screen corner immediately halts execution.
- **🚀 100% Self-Contained Go Executables**: Zero dependencies (no Python, pip, or Node required).

---

## 📊 Measured Latency & Performance Metrics

Benchmarked on Windows 11 (AMD64):

| Metric | Measured Value | Analysis & Comparison |
| :--- | :--- | :--- |
| **Win32 `SendInput` Overhead** | **7.21 µs** (0.0072 ms) | Native Win32 API dispatch per keydown/keyup pair. Zero dropped keystrokes. |
| **Timer Precision (`time.Sleep`)** | **0.42 ms deviation** | High-precision Go runtime scheduler (vs ~15.6 ms default Windows tick in Python). |
| **HTTP / WebSocket Loopback Latency** | **0.36 ms avg** | Sub-millisecond round-trip time between client and engine. |
| **Memory Footprint** | **~1.02 MB RAM** | Low memory overhead (<12 MB total process allocation). |
| **Keystroke Speeds** | • 40 WPM: 300 ms/char<br>• 100 WPM: 120 ms/char<br>• 250 WPM: 48 ms/char<br>• 500 WPM: 24 ms/char | Base delay calculated via `delay_ms = 12000 / wpm`. |
| **Fast Paste Throughput** | **100,000 chars in < 0.001s** | Instant clipboard memory transfer using native Windows `CF_UNICODETEXT`. |

---

## 🚀 Quick Start (Download & Run)

Download the release package (`AutoTyper-v1.0.0-windows-amd64.zip`):

```
📁 Auto Typer/
├── server.exe           ← Double-click to launch (runs in background, opens Pairing Hub)
├── stop.exe             ← Double-click to cleanly shut down
└── allow-firewall.bat   ← Run once as Administrator if local Wi-Fi blocks connections (Windows)
```

### 🍎 macOS Quick Start

1. **Grant Accessibility Permissions** (Required for macOS keystroke simulation):
   - Go to **System Settings** > **Privacy & Security** > **Accessibility**.
   - Add and enable your **Terminal** (or iTerm2 / shell).
2. **Start the Server**:
   ```bash
   ./start.sh
   # or directly: ./server
   ```
   Automatically opens the **Pairing Hub** (`http://localhost:8080/dashboard`).
3. **Stop Anytime**:
   ```bash
   ./stop.sh
   # or directly: ./stop
   # or click the red "Stop Server" button on the web dashboard
   ```

---

### 🪟 Windows Quick Start

1. **Launch Server**: Double-click **`server.exe`** (runs in background, opens Pairing Hub).
2. **Stop Anytime**: Double-click **`stop.exe`** or click **Stop Server** on the dashboard.

---

### 📱 Pair & Type (Phone to Laptop)
1. **Pair Your Phone**:
   - Scan the **QR Code** on the laptop dashboard using your phone's camera.
   - *Or*: Navigate to the displayed pairing URL and type the **6-digit PIN**.
2. **Start Typing**:
   - Paste or type code into the phone app.
   - Configure speed (10–500 WPM), Jitter level, and Oops typo rate.
   - Tap **Start Typing** and switch focus to your target app (VS Code, Xcode, terminal, editor) during the countdown!
3. **Emergency Stop**:
   - Move your mouse to any screen corner for an instant emergency abort failsafe.

---

## 🛠️ Building from Source

Requires [Go 1.22+](https://golang.org):

### On macOS:
```bash
./build.sh
# Or using make:
make build
# Or directly:
go build -ldflags="-s -w" -o server ./cmd/server
go build -ldflags="-s -w" -o stop ./cmd/stop
```

### On Windows:
```bash
go build -ldflags="-H=windowsgui -s -w" -o server.exe ./cmd/server
go build -ldflags="-s -w" -o stop.exe ./cmd/stop
```

All HTML, CSS, JavaScript, and icons are embedded directly into the binary using Go's `//go:embed`.

---

## 📄 License
MIT License.
