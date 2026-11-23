package keyboard

// KeySimulator defines the interface for typing and clipboard simulation.
type KeySimulator interface {
	WriteRune(r rune) error
	PressEnter() error
	PressTab() error
	PressBackspace() error
	PressForwardDelete() error
	ClearLineIndent() error
	PasteText(text string) error
	CheckFailsafe() bool
}

// New returns the platform-specific KeySimulator implementation.
func New() KeySimulator {
	return newPlatformSimulator()
}
