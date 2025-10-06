package commands

import (
	"fmt"

	"github.com/atotto/clipboard"
)

// CopyToClipboard copies text to system clipboard and returns a user-friendly message
func CopyToClipboard(text string) (string, error) {
	if err := clipboard.WriteAll(text); err != nil {
		return "", fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return fmt.Sprintf("Command copied to clipboard: %s", text), nil
}
