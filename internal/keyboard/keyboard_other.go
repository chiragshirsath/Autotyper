//go:build !windows && !darwin

package keyboard

import (
	"fmt"
)

type dummySimulator struct{}

func newPlatformSimulator() KeySimulator {
	return &dummySimulator{}
}

func (d *dummySimulator) WriteRune(r rune) error {
	return fmt.Errorf("keyboard simulation is only implemented on Windows in this build")
}

func (d *dummySimulator) PressEnter() error {
	return nil
}

func (d *dummySimulator) PressTab() error {
	return nil
}

func (d *dummySimulator) PressBackspace() error {
	return nil
}

func (d *dummySimulator) PasteText(text string) error {
	return fmt.Errorf("clipboard paste is only implemented on Windows in this build")
}

func (d *dummySimulator) CheckFailsafe() bool {
	return false
}
