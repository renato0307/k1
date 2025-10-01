package modals

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"timoneiro/internal/types"
)

var (
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(50)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))
)

type screenItem struct {
	id    string
	title string
}

func (i screenItem) FilterValue() string { return i.title }
func (i screenItem) Title() string       { return i.title }
func (i screenItem) Description() string { return i.id }

type ScreenPickerModal struct {
	list   list.Model
	width  int
	height int
}

func NewScreenPickerModal(screens []types.Screen) *ScreenPickerModal {
	items := make([]list.Item, len(screens))
	for i, screen := range screens {
		items[i] = screenItem{
			id:    screen.ID(),
			title: screen.Title(),
		}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Select Screen"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &ScreenPickerModal{
		list: l,
	}
}

func (m *ScreenPickerModal) Init() tea.Cmd {
	return nil
}

func (m *ScreenPickerModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(screenItem); ok {
				return m, func() tea.Msg {
					return types.ScreenSwitchMsg{ScreenID: item.id}
				}
			}
		case "esc":
			return m, func() tea.Msg {
				return types.ToggleScreenPickerMsg{}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *ScreenPickerModal) View() string {
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

func (m *ScreenPickerModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ScreenPickerModal) CenteredView(termWidth, termHeight int) string {
	m.width = termWidth
	m.height = termHeight
	return m.View()
}
