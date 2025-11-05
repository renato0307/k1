package testutil

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestProgram wraps a Bubble Tea program for testing
type TestProgram struct {
	program *tea.Program
	output  *bytes.Buffer
	input   *fakeInput
	t       *testing.T
}

// fakeInput implements io.Reader for simulating keyboard input
type fakeInput struct {
	data chan byte
}

func newFakeInput() *fakeInput {
	return &fakeInput{data: make(chan byte, 1024)}
}

func (f *fakeInput) Read(p []byte) (n int, err error) {
	select {
	case b := <-f.data:
		p[0] = b
		return 1, nil
	case <-time.After(50 * time.Millisecond):
		return 0, io.EOF
	}
}

// NewTestProgram creates a new test program with controlled I/O
func NewTestProgram(t *testing.T, model tea.Model, width, height int) *TestProgram {
	t.Helper()

	output := &bytes.Buffer{}
	input := newFakeInput()

	p := tea.NewProgram(
		model,
		tea.WithInput(input),
		tea.WithOutput(output),
	)

	tp := &TestProgram{
		program: p,
		output:  output,
		input:   input,
		t:       t,
	}

	// Start the program in the background
	go func() {
		if _, err := p.Run(); err != nil {
			t.Logf("Program error: %v", err)
		}
	}()

	// Give the program time to start
	time.Sleep(50 * time.Millisecond)

	// Send initial window size
	tp.Send(tea.WindowSizeMsg{Width: width, Height: height})

	return tp
}

// Send sends a message to the program
func (tp *TestProgram) Send(msg tea.Msg) {
	tp.program.Send(msg)
	time.Sleep(50 * time.Millisecond) // Give time for message to process
}

// Type simulates typing a string
func (tp *TestProgram) Type(s string) {
	for _, r := range s {
		tp.Send(tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{r},
		})
	}
}

// SendKey sends a specific key press
func (tp *TestProgram) SendKey(key tea.KeyType) {
	tp.Send(tea.KeyMsg{Type: key})
}

// Output returns the current output buffer content
func (tp *TestProgram) Output() string {
	return tp.output.String()
}

// WaitForOutput waits for specific text to appear in output
func (tp *TestProgram) WaitForOutput(needle string, timeout time.Duration) bool {
	tp.t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(tp.Output(), needle) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// AssertContains checks if output contains expected text
func (tp *TestProgram) AssertContains(expected string) {
	tp.t.Helper()

	output := tp.Output()
	if !strings.Contains(output, expected) {
		tp.t.Errorf("Output does not contain %q\nGot:\n%s", expected, output)
	}
}

// AssertNotContains checks if output does NOT contain text
func (tp *TestProgram) AssertNotContains(notExpected string) {
	tp.t.Helper()

	output := tp.Output()
	if strings.Contains(output, notExpected) {
		tp.t.Errorf("Output should not contain %q\nGot:\n%s", notExpected, output)
	}
}

// Quit stops the program
func (tp *TestProgram) Quit() {
	tp.program.Quit()
}

// WaitForScreen waits for a specific screen title to appear
func (tp *TestProgram) WaitForScreen(screenName string, timeout time.Duration) bool {
	tp.t.Helper()
	return tp.WaitForOutput(screenName, timeout)
}

// TypeCommand types a command palette command (including the "/" prefix)
func (tp *TestProgram) TypeCommand(cmd string) {
	tp.Type("/" + cmd)
}

// SendCtrl sends a ctrl+key combination
func (tp *TestProgram) SendCtrl(key rune) {
	tp.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{key},
		Alt:   true, // Bubble Tea uses Alt for ctrl combinations
	})
}

// WaitForConfirmation waits for confirmation dialog to appear
func (tp *TestProgram) WaitForConfirmation(timeout time.Duration) bool {
	tp.t.Helper()
	// Look for common confirmation text patterns
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output := tp.Output()
		if strings.Contains(output, "confirm") ||
			strings.Contains(output, "Confirm") ||
			strings.Contains(output, "Are you sure") {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// WaitForMessage waits for a success/error/info message to appear
func (tp *TestProgram) WaitForMessage(messageType string, timeout time.Duration) bool {
	tp.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output := tp.Output()
		switch strings.ToLower(messageType) {
		case "success":
			if strings.Contains(output, "✓") || strings.Contains(output, "Success") {
				return true
			}
		case "error":
			if strings.Contains(output, "Error") || strings.Contains(output, "Failed") {
				return true
			}
		case "info":
			if strings.Contains(output, "Info") || strings.Contains(output, "ℹ") {
				return true
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// GetOutput is an alias for Output() for consistency
func (tp *TestProgram) GetOutput() string {
	return tp.Output()
}
