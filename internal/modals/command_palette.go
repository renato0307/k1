package modals

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"timoneiro/internal/types"
)

type operationItem struct {
	op types.Operation
}

func (i operationItem) FilterValue() string { return i.op.Name }
func (i operationItem) Title() string {
	if i.op.Shortcut != "" {
		return fmt.Sprintf("%s [%s]", i.op.Name, i.op.Shortcut)
	}
	return i.op.Name
}
func (i operationItem) Description() string { return i.op.Description }

type CommandPaletteModal struct {
	list       list.Model
	operations []types.Operation
	width      int
	height     int
}

func NewCommandPaletteModal(operations []types.Operation) *CommandPaletteModal {
	items := make([]list.Item, len(operations))
	for i, op := range operations {
		items[i] = operationItem{op: op}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Command Palette"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &CommandPaletteModal{
		list:       l,
		operations: operations,
	}
}

func (m *CommandPaletteModal) Init() tea.Cmd {
	return nil
}

func (m *CommandPaletteModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(operationItem); ok {
				return m, tea.Batch(
					item.op.Execute(),
					func() tea.Msg {
						return types.ToggleCommandPaletteMsg{}
					},
				)
			}
		case "esc":
			return m, func() tea.Msg {
				return types.ToggleCommandPaletteMsg{}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *CommandPaletteModal) View() string {
	// Use 80% of terminal height for modal
	modalHeight := int(float64(m.height) * 0.8)
	if modalHeight < 10 {
		modalHeight = 10
	}

	// Fixed width for modal
	modalWidth := 60

	// List size = modal size - border and padding (4 lines/chars for padding)
	listWidth := modalWidth - 6
	listHeight := modalHeight - 4

	m.list.SetSize(listWidth, listHeight)

	return modalStyle.Width(modalWidth).Height(modalHeight).Render(m.list.View())
}

func (m *CommandPaletteModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *CommandPaletteModal) UpdateOperations(operations []types.Operation) {
	m.operations = operations
	items := make([]list.Item, len(operations))
	for i, op := range operations {
		items[i] = operationItem{op: op}
	}
	m.list.SetItems(items)
}

func (m *CommandPaletteModal) CenteredView(termWidth, termHeight int) string {
	m.width = termWidth
	m.height = termHeight
	return m.View()
}
