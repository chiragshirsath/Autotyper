//go:build darwin

package keyboard

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <unistd.h>

static int checkAccessibility(int prompt) {
    if (prompt) {
        const void *keys[] = { kAXTrustedCheckOptionPrompt };
        const void *values[] = { kCFBooleanTrue };
        CFDictionaryRef options = CFDictionaryCreate(
            kCFAllocatorDefault,
            keys,
            values,
            1,
            &kCFTypeDictionaryKeyCallBacks,
            &kCFTypeDictionaryValueCallBacks
        );
        Boolean trusted = AXIsProcessTrustedWithOptions(options);
        CFRelease(options);
        return trusted ? 1 : 0;
    }
    return AXIsProcessTrusted() ? 1 : 0;
}

static int sendUnicodeRune(uint32_t r) {
    UniChar buffer[2];
    int len = 0;
    if (r <= 0xFFFF) {
        buffer[0] = (UniChar)r;
        len = 1;
    } else {
        r -= 0x10000;
        buffer[0] = (UniChar)(0xD800 + (r >> 10));
        buffer[1] = (UniChar)(0xDC00 + (r & 0x3FF));
        len = 2;
    }

    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, 0, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, 0, false);

    CGEventKeyboardSetUnicodeString(keyDown, len, buffer);
    CGEventKeyboardSetUnicodeString(keyUp, len, buffer);

    CGEventPost(kCGHIDEventTap, keyDown);
    CFRelease(keyDown);

    usleep(1200); // 1.2ms pause to ensure window server event separation

    CGEventPost(kCGHIDEventTap, keyUp);
    CFRelease(keyUp);

    if (source) CFRelease(source);
    return 0;
}

static int sendKeyCode(CGKeyCode keyCode) {
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, keyCode, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, keyCode, false);

    CGEventPost(kCGHIDEventTap, keyDown);
    CFRelease(keyDown);

    usleep(1200);

    CGEventPost(kCGHIDEventTap, keyUp);
    CFRelease(keyUp);

    if (source) CFRelease(source);
    return 0;
}

static int sendCmdV(void) {
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    CGKeyCode vKey = 9; // kVK_ANSI_V
    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, vKey, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, vKey, false);

    CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyUp, kCGEventFlagMaskCommand);

    CGEventPost(kCGHIDEventTap, keyDown);
    CFRelease(keyDown);

    usleep(5000); // 5ms for Command+V recognition

    CGEventPost(kCGHIDEventTap, keyUp);
    CFRelease(keyUp);

    if (source) CFRelease(source);
    return 0;
}

static int sendCmdDelete(void) {
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    CGKeyCode deleteKey = 51; // kVK_Delete (backspace)
    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, deleteKey, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, deleteKey, false);

    CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
    CGEventSetFlags(keyUp, kCGEventFlagMaskCommand);

    CGEventPost(kCGHIDEventTap, keyDown);
    CFRelease(keyDown);

    usleep(2500); // 2.5ms for Command+Delete to clear indentation

    CGEventPost(kCGHIDEventTap, keyUp);
    CFRelease(keyUp);

    if (source) CFRelease(source);
    return 0;
}

static int sendForwardDelete(void) {
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    CGKeyCode fwdKey = 117; // kVK_ForwardDelete
    CGEventRef keyDown = CGEventCreateKeyboardEvent(source, fwdKey, true);
    CGEventRef keyUp = CGEventCreateKeyboardEvent(source, fwdKey, false);

    CGEventPost(kCGHIDEventTap, keyDown);
    CFRelease(keyDown);

    usleep(1500);

    CGEventPost(kCGHIDEventTap, keyUp);
    CFRelease(keyUp);

    if (source) CFRelease(source);
    return 0;
}

static int checkFailsafe(int cornerThreshold) {
    CGEventRef event = CGEventCreate(NULL);
    if (!event) return 0;
    CGPoint loc = CGEventGetLocation(event);
    CFRelease(event);

    CGDirectDisplayID mainDisplay = CGMainDisplayID();
    size_t width = CGDisplayPixelsWide(mainDisplay);
    size_t height = CGDisplayPixelsHigh(mainDisplay);

    int inTopLeft = (loc.x <= cornerThreshold && loc.y <= cornerThreshold);
    int inTopRight = (loc.x >= (double)(width - cornerThreshold) && loc.y <= cornerThreshold);
    int inBottomLeft = (loc.x <= cornerThreshold && loc.y >= (double)(height - cornerThreshold));
    int inBottomRight = (loc.x >= (double)(width - cornerThreshold) && loc.y >= (double)(height - cornerThreshold));

    return (inTopLeft || inTopRight || inBottomLeft || inBottomRight) ? 1 : 0;
}
*/
import "C"

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	macKeyReturn    = 36 // kVK_Return
	macKeyTab       = 48 // kVK_Tab
	macKeyBackspace = 51 // kVK_Delete
)

type macSimulator struct {
	warnOnce sync.Once
}

func newPlatformSimulator() KeySimulator {
	sim := &macSimulator{}
	if !sim.isTrusted() {
		log.Println("──────────────────────────────────────────────────────────")
		log.Println("⚠️  [macOS Accessibility]: Auto Typer requires Accessibility")
		log.Println("   permissions to simulate keystrokes on Mac.")
		log.Println("👉 System Settings > Privacy & Security > Accessibility")
		log.Println("   and enable permissions for your Terminal / Auto Typer.")
		log.Println("──────────────────────────────────────────────────────────")
		// Prompt macOS native permission request dialog if available
		_ = C.checkAccessibility(1)
	}
	return sim
}

func (m *macSimulator) isTrusted() bool {
	return C.checkAccessibility(0) == 1
}

func (m *macSimulator) ensurePermissions() error {
	if !m.isTrusted() {
		m.warnOnce.Do(func() {
			_ = C.checkAccessibility(1)
		})
		return fmt.Errorf("accessibility permissions not granted. Enable in macOS System Settings > Privacy & Security > Accessibility")
	}
	return nil
}

func (m *macSimulator) WriteRune(r rune) error {
	if err := m.ensurePermissions(); err != nil {
		return err
	}
	C.sendUnicodeRune(C.uint32_t(r))
	return nil
}

func (m *macSimulator) PressEnter() error {
	if err := m.ensurePermissions(); err != nil {
		return err
	}
	C.sendKeyCode(macKeyReturn)
	return nil
}

func (m *macSimulator) PressTab() error {
	if err := m.ensurePermissions(); err != nil {
		return err
	}
	C.sendKeyCode(macKeyTab)
	return nil
}

func (m *macSimulator) PressBackspace() error {
	if err := m.ensurePermissions(); err != nil {
		return err
	}
	C.sendKeyCode(macKeyBackspace)
	return nil
}

func (m *macSimulator) PressForwardDelete() error {
	if err := m.ensurePermissions(); err != nil {
		return err
	}
	C.sendForwardDelete()
	return nil
}

func (m *macSimulator) ClearLineIndent() error {
	if err := m.ensurePermissions(); err != nil {
		return err
	}
	C.sendCmdDelete()
	return nil
}

func (m *macSimulator) PasteText(text string) error {
	if err := m.ensurePermissions(); err != nil {
		return err
	}

	// 1. Copy text to macOS clipboard using native pbcopy
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy failed: %w", err)
	}

	time.Sleep(20 * time.Millisecond)

	// 2. Trigger Command + V
	C.sendCmdV()
	return nil
}

func (m *macSimulator) CheckFailsafe() bool {
	return C.checkFailsafe(10) == 1
}
