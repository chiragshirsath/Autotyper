package state

// Settings represents the shared state across all devices.
type Settings struct {
	Text               string `json:"text"`
	WPM                int    `json:"wpm"`
	FocusDelaySec      int    `json:"focus_delay_sec"`
	PreserveFormatting bool   `json:"preserve_formatting"`
	SmartIndent        bool   `json:"smart_indent"`
	PasteMode          bool   `json:"paste_mode"`
	JitterMode         bool   `json:"jitter_mode"`
	JitterLevel        int    `json:"jitter_level"` // 1=Low (25%), 2=Medium (50%), 3=High (80%), 4=Erratic / Thinking Pauses (120%)
	OopsMode           bool   `json:"oops_mode"`
	ErrorRate          int    `json:"error_rate"` // 1 to 20 percent (default 3)
}

// WSMessage defines the protocol for WebSocket events.
type WSMessage struct {
	Type               string    `json:"type"`
	Text               string    `json:"text,omitempty"`
	WPM                int       `json:"wpm,omitempty"`
	FocusDelaySec      int       `json:"focus_delay_sec,omitempty"`
	PreserveFormatting *bool     `json:"preserve_formatting,omitempty"`
	SmartIndent        *bool     `json:"smart_indent,omitempty"`
	PasteMode          *bool     `json:"paste_mode,omitempty"`
	JitterMode         *bool     `json:"jitter_mode,omitempty"`
	JitterLevel        int       `json:"jitter_level,omitempty"`
	OopsMode           *bool     `json:"oops_mode,omitempty"`
	ErrorRate          int       `json:"error_rate,omitempty"`
	DelayMs            int       `json:"delay_ms,omitempty"`
	Current            int       `json:"current,omitempty"`
	Total              int       `json:"total,omitempty"`
	Message            string    `json:"message,omitempty"`
	State              *Settings `json:"state,omitempty"`
	Sender             string    `json:"sender,omitempty"`
	Authorized         bool      `json:"authorized,omitempty"`
	Token              string    `json:"token,omitempty"`
}
