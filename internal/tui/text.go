package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

type textModel struct {
	title                 string
	lines                 []string
	offset, width, height int
	horizontal            int
}

func (m textModel) Init() tea.Cmd { return nil }
func (m textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "enter", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.offset = max(0, m.offset-1)
		case "down", "j":
			m.offset = min(max(0, len(m.lines)-1), m.offset+1)
		case "pgup":
			m.offset = max(0, m.offset-max(1, m.height-5))
		case "pgdown":
			m.offset = min(max(0, len(m.lines)-1), m.offset+max(1, m.height-5))
		case "home":
			m.offset = 0
		case "end":
			m.offset = max(0, len(m.lines)-max(1, m.height-5))
		case "left":
			m.horizontal = max(0, m.horizontal-10)
		case "right":
			m.horizontal += 10
		}
	}
	return m, nil
}
func (m textModel) View() string {
	style := lipgloss.NewStyle().MaxWidth(max(10, m.width-2)).Inline(true)
	var b strings.Builder
	b.WriteString(style.Bold(true).Render(cleanTerminal(m.title)) + "\n\n")
	for _, line := range m.lines[m.offset:min(len(m.lines), m.offset+max(1, m.height-5))] {
		runes := []rune(line)
		line = ""
		if m.horizontal < len(runes) {
			line = string(runes[m.horizontal:])
		}
		b.WriteString(style.Render(line) + "\n")
	}
	b.WriteString("\n" + style.Render(Text("↑/↓ Dọc · ←/→ Ngang · PgUp/PgDn · Esc Quay lại", "↑/↓ Vertical · ←/→ Horizontal · PgUp/PgDn · Esc Back")))
	return b.String()
}
func ShowText(title, text string) error {
	_, err := tea.NewProgram(textModel{title: title, lines: sanitizedLines(text), width: 80, height: 24}, tea.WithAltScreen()).Run()
	return err
}

func sanitizedLines(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = cleanTerminal(line)
	}
	return lines
}
