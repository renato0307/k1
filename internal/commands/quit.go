package commands

import (
	tea "github.com/charmbracelet/bubbletea"
)

// QuitCommand returns a command that quits the application
func QuitCommand() ExecuteFunc {
	return func(ctx CommandContext) tea.Cmd {
		return tea.Quit
	}
}
