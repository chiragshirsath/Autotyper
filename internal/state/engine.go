package state

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode"

	"autotyper/internal/keyboard"
)

var qwertyNeighbors = map[rune][]rune{
	'q': {'w', 'a', 's'},
	'w': {'q', 'e', 'a', 's', 'd'},
	'e': {'w', 'r', 's', 'd', 'f'},
	'r': {'e', 't', 'd', 'f', 'g'},
	't': {'r', 'y', 'f', 'g', 'h'},
	'y': {'t', 'u', 'g', 'h', 'j'},
	'u': {'y', 'i', 'h', 'j', 'k'},
	'i': {'u', 'o', 'j', 'k', 'l'},
	'o': {'i', 'p', 'k', 'l'},
	'p': {'o', 'l', '['},
	'a': {'q', 'w', 's', 'z'},
	's': {'w', 'e', 'a', 'd', 'z', 'x'},
	'd': {'e', 'r', 's', 'f', 'x', 'c'},
	'f': {'r', 't', 'd', 'g', 'c', 'v'},
	'g': {'t', 'y', 'f', 'h', 'v', 'b'},
	'h': {'y', 'u', 'g', 'j', 'b', 'n'},
	'j': {'u', 'i', 'h', 'k', 'n', 'm'},
	'k': {'i', 'o', 'j', 'l', 'm'},
	'l': {'o', 'p', 'k', ';'},
	'z': {'a', 's', 'x'},
	'x': {'z', 's', 'd', 'c'},
	'c': {'x', 'd', 'f', 'v'},
	'v': {'c', 'f', 'g', 'b'},
	'b': {'v', 'g', 'h', 'n'},
	'n': {'b', 'h', 'j', 'm'},
	'm': {'n', 'j', 'k'},
}

// Engine orchestrates keyboard typing, settings, and multi-device coordination.
type Engine struct {
	mu           sync.RWMutex
	settings     Settings
	simulator    keyboard.KeySimulator
	broadcaster  func(msg WSMessage)
	cancelActive context.CancelFunc
	isPunching   bool
}

// NewEngine creates a new typing engine.
func NewEngine(broadcaster func(msg WSMessage)) *Engine {
	return &Engine{
		settings: Settings{
			Text:               "",
			WPM:                100,
			FocusDelaySec:      3,
			PreserveFormatting: true,
			SmartIndent:        false, // Disabled by default to prevent backspacing over newlines in VS Code
			PasteMode:          false,
			JitterMode:         false,
			JitterLevel:        2, // Medium
			OopsMode:           false,
			ErrorRate:          3, // 3%
		},
		simulator:   keyboard.New(),
		broadcaster: broadcaster,
	}
}

// GetSettings returns a copy of current shared settings.
func (e *Engine) GetSettings() Settings {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.settings
}

// UpdateSettings updates the shared state and broadcasts changes.
func (e *Engine) UpdateSettings(msg WSMessage, senderIP string) {
	e.mu.Lock()
	if msg.Text != "" || msg.Type == "update_code" {
		e.settings.Text = msg.Text
	}
	if msg.WPM > 0 {
		e.settings.WPM = msg.WPM
	}
	if msg.FocusDelaySec >= 0 {
		e.settings.FocusDelaySec = msg.FocusDelaySec
	}
	if msg.PreserveFormatting != nil {
		e.settings.PreserveFormatting = *msg.PreserveFormatting
	}
	if msg.SmartIndent != nil {
		e.settings.SmartIndent = *msg.SmartIndent
	}
	if msg.PasteMode != nil {
		e.settings.PasteMode = *msg.PasteMode
	}
	if msg.JitterMode != nil {
		e.settings.JitterMode = *msg.JitterMode
	}
	if msg.JitterLevel > 0 {
		e.settings.JitterLevel = msg.JitterLevel
	}
	if msg.OopsMode != nil {
		e.settings.OopsMode = *msg.OopsMode
	}
	if msg.ErrorRate > 0 {
		e.settings.ErrorRate = msg.ErrorRate
	}
	snapshot := e.settings
	e.mu.Unlock()

	log.Printf("🔄 Code updated (%d chars) from %s", len(snapshot.Text), senderIP)

	e.broadcaster(WSMessage{
		Type:               "code_updated",
		Text:               snapshot.Text,
		WPM:                snapshot.WPM,
		FocusDelaySec:      snapshot.FocusDelaySec,
		PreserveFormatting: &snapshot.PreserveFormatting,
		SmartIndent:        &snapshot.SmartIndent,
		PasteMode:          &snapshot.PasteMode,
		JitterMode:         &snapshot.JitterMode,
		JitterLevel:        snapshot.JitterLevel,
		OopsMode:           &snapshot.OopsMode,
		ErrorRate:          snapshot.ErrorRate,
		Sender:             senderIP,
	})
}

// Stop halts any active punching job.
func (e *Engine) Stop() {
	e.mu.Lock()
	if e.cancelActive != nil {
		e.cancelActive()
		e.cancelActive = nil
	}
	e.isPunching = false
	e.mu.Unlock()

	e.broadcaster(WSMessage{Type: "stopped"})
	log.Println("⏹ Punching stopped by user")
}

// StartPunch starts the typing job in background.
func (e *Engine) StartPunch(msg WSMessage, senderIP string) error {
	e.mu.Lock()
	if e.cancelActive != nil {
		e.cancelActive()
	}

	if msg.Text != "" {
		e.settings.Text = msg.Text
	}
	if msg.WPM > 0 {
		e.settings.WPM = msg.WPM
	}
	if msg.PreserveFormatting != nil {
		e.settings.PreserveFormatting = *msg.PreserveFormatting
	}
	if msg.SmartIndent != nil {
		e.settings.SmartIndent = *msg.SmartIndent
	}
	if msg.PasteMode != nil {
		e.settings.PasteMode = *msg.PasteMode
	}
	if msg.JitterMode != nil {
		e.settings.JitterMode = *msg.JitterMode
	}
	if msg.JitterLevel > 0 {
		e.settings.JitterLevel = msg.JitterLevel
	}
	if msg.OopsMode != nil {
		e.settings.OopsMode = *msg.OopsMode
	}
	if msg.ErrorRate > 0 {
		e.settings.ErrorRate = msg.ErrorRate
	}

	cfg := e.settings
	ctx, cancel := context.WithCancel(context.Background())
	e.cancelActive = cancel
	e.isPunching = true
	e.mu.Unlock()

	if strings.TrimSpace(cfg.Text) == "" {
		cancel()
		return fmt.Errorf("no text to punch")
	}

	go e.runPunch(ctx, cfg, senderIP)
	return nil
}

func (e *Engine) runPunch(ctx context.Context, cfg Settings, senderIP string) {
	defer func() {
		e.mu.Lock()
		e.isPunching = false
		e.mu.Unlock()
	}()

	text := cfg.Text
	totalRunes := len([]rune(text))

	// Fast Paste Mode
	if cfg.PasteMode {
		log.Printf("📋 Fast pasting %d chars via clipboard (triggered by %s)", totalRunes, senderIP)
		if err := e.simulator.PasteText(text); err != nil {
			e.broadcaster(WSMessage{Type: "error", Message: err.Error()})
			return
		}
		e.broadcaster(WSMessage{Type: "progress", Current: totalRunes, Total: totalRunes})
		e.broadcaster(WSMessage{Type: "complete"})
		log.Println("✅ Paste complete!")
		return
	}

	baseDelayMs := 12000 / cfg.WPM
	if baseDelayMs < 1 {
		baseDelayMs = 1
	}

	var modes []string
	if cfg.SmartIndent {
		modes = append(modes, "SMART_INDENT")
	}
	if cfg.JitterMode {
		levelDesc := "MED"
		switch cfg.JitterLevel {
		case 1:
			levelDesc = "LOW"
		case 2:
			levelDesc = "MED"
		case 3:
			levelDesc = "HIGH"
		case 4:
			levelDesc = "CHAOS"
		}
		modes = append(modes, fmt.Sprintf("JITTER(%s)", levelDesc))
	}
	if cfg.OopsMode {
		modes = append(modes, fmt.Sprintf("OOPS(%d%%)", cfg.ErrorRate))
	}
	modesStr := ""
	if len(modes) > 0 {
		modesStr = fmt.Sprintf(" [%s]", strings.Join(modes, "+"))
	}
	log.Printf("⌨️  Typing %d chars at %d WPM (base delay %dms)%s", totalRunes, cfg.WPM, baseDelayMs, modesStr)

	// Smart IDE Auto-Indent Mode
	if cfg.SmartIndent && cfg.PreserveFormatting {
		e.typeSmartIndent(ctx, cfg, baseDelayMs, totalRunes)
	} else {
		e.typeNormal(ctx, cfg, baseDelayMs, totalRunes)
	}
}

func (e *Engine) typeSmartIndent(ctx context.Context, cfg Settings, baseDelayMs, totalRunes int) {
	lines := strings.Split(strings.ReplaceAll(cfg.Text, "\r\n", "\n"), "\n")
	curIDEIndent := 0
	const tabSize = 4
	typedRunes := 0

	for lineIdx, line := range lines {
		select {
		case <-ctx.Done():
			e.broadcaster(WSMessage{Type: "stopped"})
			return
		default:
		}

		if e.simulator.CheckFailsafe() {
			log.Println("🛡️  Emergency stop: Mouse moved to screen corner!")
			e.broadcaster(WSMessage{Type: "error", Message: "Emergency failsafe triggered (mouse in corner)"})
			return
		}

		strippedLine := strings.TrimLeft(line, " \t")
		leadingWS := line[:len(line)-len(strippedLine)]
		targetIndent := 0
		for _, ch := range leadingWS {
			if ch == '\t' {
				targetIndent += tabSize
			} else {
				targetIndent++
			}
		}

		if lineIdx == 0 {
			if len(leadingWS) > 0 {
				for _, wsChar := range leadingWS {
					if wsChar == '\t' {
						_ = e.simulator.PressTab()
					} else {
						_ = e.simulator.WriteRune(wsChar)
					}
				}
				curIDEIndent = targetIndent
				typedRunes += len([]rune(leadingWS))
			}
		} else {
			_ = e.simulator.PressEnter()
			typedRunes++

			// Wait for IDE auto-indent calculation
			time.Sleep(35 * time.Millisecond)

			prevLine := lines[lineIdx-1]
			cleanPrev := strings.TrimRight(strings.Split(strings.Split(prevLine, "#")[0], "//")[0], " \t")
			if strings.HasSuffix(cleanPrev, ":") || strings.HasSuffix(cleanPrev, "{") {
				curIDEIndent += tabSize
			}

			if len(strippedLine) > 0 {
				if targetIndent < curIDEIndent {
					dedents := (curIDEIndent - targetIndent) / tabSize
					remSpaces := (curIDEIndent - targetIndent) % tabSize
					for i := 0; i < dedents; i++ {
						time.Sleep(25 * time.Millisecond)
						_ = e.simulator.PressBackspace()
					}
					for i := 0; i < remSpaces; i++ {
						time.Sleep(15 * time.Millisecond)
						_ = e.simulator.PressBackspace()
					}
					curIDEIndent = targetIndent
				} else if targetIndent > curIDEIndent {
					extra := targetIndent - curIDEIndent
					for i := 0; i < extra; i++ {
						_ = e.simulator.WriteRune(' ')
					}
					curIDEIndent = targetIndent
				}
			}

			typedRunes += len([]rune(leadingWS))
			e.broadcaster(WSMessage{Type: "progress", Current: min(totalRunes, typedRunes), Total: totalRunes})
		}

		// Type characters in the line
		for _, ch := range strippedLine {
			select {
			case <-ctx.Done():
				e.broadcaster(WSMessage{Type: "stopped"})
				return
			default:
			}

			if e.simulator.CheckFailsafe() {
				log.Println("🛡️  Emergency stop: Mouse in corner!")
				e.broadcaster(WSMessage{Type: "error", Message: "Emergency failsafe triggered"})
				return
			}

			e.typeSingleRune(ch, cfg.OopsMode, cfg.ErrorRate)

			typedRunes++
			if typedRunes%5 == 0 || typedRunes == totalRunes {
				e.broadcaster(WSMessage{Type: "progress", Current: min(totalRunes, typedRunes), Total: totalRunes})
			}

			e.applyDelay(baseDelayMs, cfg.JitterMode, cfg.JitterLevel, ch)
		}
	}

	e.broadcaster(WSMessage{Type: "complete"})
	log.Println("✅ Typing complete!")
}

func (e *Engine) typeNormal(ctx context.Context, cfg Settings, baseDelayMs, totalRunes int) {
	runes := []rune(cfg.Text)
	for i, r := range runes {
		select {
		case <-ctx.Done():
			e.broadcaster(WSMessage{Type: "stopped"})
			return
		default:
		}

		if e.simulator.CheckFailsafe() {
			log.Println("🛡️  Emergency stop: Mouse in corner!")
			e.broadcaster(WSMessage{Type: "error", Message: "Emergency failsafe triggered"})
			return
		}

		if r == '\r' {
			continue
		} else if r == '\n' {
			if cfg.PreserveFormatting {
				_ = e.simulator.PressEnter()
			} else {
				_ = e.simulator.WriteRune(' ')
			}
		} else if r == '\t' {
			if cfg.PreserveFormatting {
				_ = e.simulator.PressTab()
			} else {
				for s := 0; s < 4; s++ {
					_ = e.simulator.WriteRune(' ')
				}
			}
		} else {
			e.typeSingleRune(r, cfg.OopsMode, cfg.ErrorRate)
		}

		current := i + 1
		if current%5 == 0 || current == totalRunes {
			e.broadcaster(WSMessage{Type: "progress", Current: current, Total: totalRunes})
		}

		e.applyDelay(baseDelayMs, cfg.JitterMode, cfg.JitterLevel, r)
	}

	e.broadcaster(WSMessage{Type: "complete"})
	log.Println("✅ Typing complete!")
}

func (e *Engine) typeSingleRune(r rune, oopsMode bool, errorRate int) {
	if oopsMode && unicode.IsLetter(r) {
		rate := float64(errorRate) / 100.0
		if rate <= 0 {
			rate = 0.03
		}
		if rand.Float64() < rate {
			lower := unicode.ToLower(r)
			var wrong rune
			if neighbors, ok := qwertyNeighbors[lower]; ok && len(neighbors) > 0 {
				wrong = neighbors[rand.Intn(len(neighbors))]
			} else {
				wrong = 'a' + rune(rand.Intn(26))
			}
			if unicode.IsUpper(r) {
				wrong = unicode.ToUpper(wrong)
			}
			if wrong == r {
				wrong = 'x'
			}

			// 20% chance of double typo (typing two wrong characters)
			doubleTypo := rand.Float64() < 0.20
			var secondWrong rune
			if doubleTypo {
				secondWrong = 'a' + rune(rand.Intn(26))
				if unicode.IsUpper(r) {
					secondWrong = unicode.ToUpper(secondWrong)
				}
			}

			// 1. Type mistake
			_ = e.simulator.WriteRune(wrong)
			if doubleTypo {
				time.Sleep(time.Duration(40+rand.Intn(60)) * time.Millisecond)
				_ = e.simulator.WriteRune(secondWrong)
			}

			// 2. Realistic human delay before noticing mistake (120-250ms)
			time.Sleep(time.Duration(120+rand.Intn(130)) * time.Millisecond)

			// 3. Backspace to fix
			if doubleTypo {
				_ = e.simulator.PressBackspace()
				time.Sleep(time.Duration(50+rand.Intn(60)) * time.Millisecond)
			}
			_ = e.simulator.PressBackspace()

			// 4. Brief hesitation before typing the correct character
			time.Sleep(time.Duration(60+rand.Intn(100)) * time.Millisecond)
		}
	}
	_ = e.simulator.WriteRune(r)
}

func (e *Engine) applyDelay(baseDelayMs int, jitterMode bool, jitterLevel int, currentRune rune) {
	if baseDelayMs <= 0 {
		return
	}
	delay := float64(baseDelayMs)
	if jitterMode {
		var varianceFactor float64
		switch jitterLevel {
		case 1:
			varianceFactor = 0.25 // Subtle
		case 2:
			varianceFactor = 0.50 // Moderate natural
		case 3:
			varianceFactor = 0.85 // High human variance
		case 4:
			varianceFactor = 1.25 // Chaos / Burst with cognitive thinking pauses
		default:
			varianceFactor = 0.50
		}

		stddev := delay * varianceFactor
		jittered := rand.NormFloat64()*stddev + delay
		if jittered < 1.0 {
			jittered = 1.0
		}
		delay = jittered

		// High / Chaos level cognitive pauses
		if jitterLevel >= 3 {
			// Cognitive pause after punctuation
			if strings.ContainsRune(";.,{}()[]:=<>", currentRune) && rand.Float64() < 0.30 {
				delay += float64(150 + rand.Intn(250))
			} else if currentRune == ' ' && rand.Float64() < 0.15 {
				delay += float64(80 + rand.Intn(180))
			}
		}
	}
	time.Sleep(time.Duration(delay * float64(time.Millisecond)))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
