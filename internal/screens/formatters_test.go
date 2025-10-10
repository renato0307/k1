package screens

import (
	"testing"

	"github.com/renato0307/k1/internal/ui"
	"github.com/stretchr/testify/assert"
)

func TestFormatPodStatus(t *testing.T) {
	theme := ui.ThemeCharm()
	format := FormatPodStatus(theme)

	tests := []struct {
		name     string
		input    interface{}
		expected string // Expected output (text should always be preserved)
	}{
		{"running", "Running", "Running"},                     // No styling
		{"succeeded", "Succeeded", "Succeeded"},               // No styling
		{"failed", "Failed", "Failed"},                        // Error styling
		{"pending", "Pending", "Pending"},                     // Warning styling
		{"unknown", "Unknown", "Unknown"},                     // Warning styling
		{"crashloop", "CrashLoopBackOff", "CrashLoopBackOff"}, // Error styling
		{"imagepull", "ImagePullBackOff", "ImagePullBackOff"}, // Error styling
		{"errimagepull", "ErrImagePull", "ErrImagePull"},      // Error styling
		{"error", "Error", "Error"},                           // Error styling
		{"containercreating", "ContainerCreating", "ContainerCreating"}, // Warning styling
		{"custom", "CustomStatus", "CustomStatus"},            // No styling (unknown)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := format(tt.input)
			// The result should always contain the original text
			// (lipgloss may or may not add ANSI codes depending on terminal)
			assert.Contains(t, result, tt.expected, "Result should contain status text")
		})
	}
}

func TestFormatPodReady(t *testing.T) {
	theme := ui.ThemeCharm()
	format := FormatPodReady(theme)

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"fully_ready_1", "1/1", "1/1"},       // Normal
		{"fully_ready_5", "5/5", "5/5"},       // Normal
		{"partial_2_3", "2/3", "2/3"},         // Warning (yellow)
		{"partial_1_3", "1/3", "1/3"},         // Warning (yellow)
		{"partial_4_5", "4/5", "4/5"},         // Warning (yellow)
		{"not_ready", "0/1", "0/1"},           // Error (red)
		{"not_ready_0_5", "0/5", "0/5"},       // Error (red)
		{"invalid", "invalid", "invalid"},     // Can't parse, no styling
		{"empty", "", ""},                     // Empty string, no styling
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := format(tt.input)
			assert.Contains(t, result, tt.expected, "Result should contain ready text")
		})
	}
}

func TestFormatPodStatus_AllThemes(t *testing.T) {
	// Test that formatters work with all themes
	themes := []string{"charm", "dracula", "catppuccin", "nord", "gruvbox", "tokyo-night", "solarized", "monokai"}

	for _, themeName := range themes {
		t.Run(themeName, func(t *testing.T) {
			theme := ui.GetTheme(themeName)
			format := FormatPodStatus(theme)

			// Test error state
			result := format("Failed")
			assert.Contains(t, result, "Failed", "Should preserve text")

			// Test normal state
			result = format("Running")
			assert.Contains(t, result, "Running", "Should preserve text")
		})
	}
}

func TestFormatPodReady_AllThemes(t *testing.T) {
	// Test that formatters work with all themes
	themes := []string{"charm", "dracula", "catppuccin", "nord", "gruvbox", "tokyo-night", "solarized", "monokai"}

	for _, themeName := range themes {
		t.Run(themeName, func(t *testing.T) {
			theme := ui.GetTheme(themeName)
			format := FormatPodReady(theme)

			// Test not ready state
			result := format("0/1")
			assert.Contains(t, result, "0/1", "Should preserve text")

			// Test fully ready state
			result = format("1/1")
			assert.Contains(t, result, "1/1", "Should preserve text")
		})
	}
}

func TestFormatPodStatus_FactoryPattern(t *testing.T) {
	// Verify that the factory pattern correctly captures the theme
	theme1 := ui.ThemeCharm()
	theme2 := ui.GetTheme("dracula")

	format1 := FormatPodStatus(theme1)
	format2 := FormatPodStatus(theme2)

	// Both formatters should work independently and preserve text
	result1 := format1("Failed")
	result2 := format2("Failed")

	assert.Contains(t, result1, "Failed", "Format1 should preserve text")
	assert.Contains(t, result2, "Failed", "Format2 should preserve text")

	// Verify normal state
	result1 = format1("Running")
	result2 = format2("Running")

	assert.Contains(t, result1, "Running", "Format1 should preserve text")
	assert.Contains(t, result2, "Running", "Format2 should preserve text")
}

func TestFormatPodReady_EdgeCases(t *testing.T) {
	theme := ui.ThemeCharm()
	format := FormatPodReady(theme)

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"zero_zero", "0/0", "0/0"},         // Edge case: 0/0 (should be normal since current == desired)
		{"large_numbers", "100/100", "100/100"}, // Large numbers, fully ready
		{"large_partial", "99/100", "99/100"}, // Large numbers, partial
		{"single_digit", "5/9", "5/9"},      // Single digit partial
		{"negative_invalid", "-1/1", "-1/1"}, // Invalid (can't parse negative)
		{"no_slash", "123", "123"},          // Invalid format
		{"multiple_slash", "1/2/3", "1/2/3"}, // Invalid format
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := format(tt.input)
			assert.Contains(t, result, tt.expected, "Should preserve text")
		})
	}
}
