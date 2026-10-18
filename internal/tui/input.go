package tui

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type inputModel struct {
	title, value, message string
	validate              func(string) error
	cursor, width         int
	done, cancelled       bool
}

func (m inputModel) Init() tea.Cmd { return nil }

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		runes := []rune(m.value)
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if m.validate != nil {
				if err := m.validate(m.value); err != nil {
					m.message = err.Error()
					return m, nil
				}
			}
			m.done = true
			return m, tea.Quit
		case "left":
			m.cursor = max(0, m.cursor-1)
		case "right":
			m.cursor = min(len(runes), m.cursor+1)
		case "home", "ctrl+a":
			m.cursor = 0
		case "end", "ctrl+e":
			m.cursor = len(runes)
		case "ctrl+u":
			m.value, m.cursor = "", 0
		case "backspace":
			if m.cursor > 0 {
				m.value = string(append(runes[:m.cursor-1], runes[m.cursor:]...))
				m.cursor--
			}
		case "delete":
			if m.cursor < len(runes) {
				m.value = string(append(runes[:m.cursor], runes[m.cursor+1:]...))
			}
		default:
			if len(msg.Runes) > 0 {
				text := []rune(strings.Map(func(r rune) rune {
					if unicode.IsControl(r) {
						return -1
					}
					return r
				}, string(msg.Runes)))
				m.value = string(runes[:m.cursor]) + string(text) + string(runes[m.cursor:])
				m.cursor += len(text)
			}
		}
	}
	return m, nil
}

func (m inputModel) View() string {
	if m.done || m.cancelled {
		return ""
	}
	style := lipgloss.NewStyle().MaxWidth(max(10, m.width-2)).Inline(true)
	runes := []rune(m.value)
	value := string(runes[:m.cursor]) + "│" + string(runes[m.cursor:])
	return style.Bold(true).Render(cleanTerminal(m.title)) + "\n\n" + style.Render(cleanTerminal(value)) + "\n\n" +
		style.Render(cleanTerminal(m.message)) + "\n\n" +
		style.Render(Text("Enter lưu · Ctrl+U xóa hết · Esc quay lại", "Enter save · Ctrl+U clear · Esc back"))
}

func InputText(title, initial string, validate func(string) error) (string, error) {
	m := inputModel{title: title, value: initial, cursor: len([]rune(initial)), width: 80, validate: validate}
	result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	final := result.(inputModel)
	if final.cancelled || !final.done {
		return "", ErrCancelled
	}
	return final.value, nil
}
