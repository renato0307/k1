package screens

import (
	"fmt"

	"github.com/renato0307/k1/internal/ui"
)

// FormatPodStatus creates a formatter that returns plain text (no colors).
// Cell-level coloring is disabled due to bubbles table ANSI truncation bugs.
// TODO: Use row-level styling instead for status indication.
func FormatPodStatus(theme *ui.Theme) func(interface{}) string {
	return func(val interface{}) string {
		status := fmt.Sprint(val)
		return status // Return plain text only
	}
}

// FormatPodReady creates a formatter that returns plain text (no colors).
// Cell-level coloring is disabled due to bubbles table ANSI truncation bugs.
// TODO: Use row-level styling instead for status indication.
func FormatPodReady(theme *ui.Theme) func(interface{}) string {
	return func(val interface{}) string {
		ready := fmt.Sprint(val)
		return ready // Return plain text only
	}
}
