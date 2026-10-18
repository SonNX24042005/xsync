package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MenuItem struct{ ID, Label string }
type menuTick struct{}
type menuItemsLoaded struct {
	items []MenuItem
	err   error
}
type itemMenu struct {
	title, query, message               string
	items                               []MenuItem
	load                                func() ([]MenuItem, error)
	cursor, width, height               int
	searching, cancelled, done, loading bool
	chosen                              MenuItem
}

func menuTimer() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return menuTick{} })
}
func (m itemMenu) Init() tea.Cmd {
	if m.load != nil {
		return menuTimer()
	}
	return nil
}
func (m itemMenu) visible() []MenuItem {
	var result []MenuItem
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item.Label), strings.ToLower(m.query)) {
			result = append(result, item)
		}
	}
	return result
}
func (m itemMenu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case menuTick:
		if m.load != nil && !m.loading {
			m.loading = true
			load := m.load
			return m, func() tea.Msg { items, err := load(); return menuItemsLoaded{items, err} }
		}
	case menuItemsLoaded:
		old := m.visible()
		id := ""
		if len(old) > 0 && m.cursor < len(old) {
			id = old[m.cursor].ID
		}
		m.loading = false
		if msg.err != nil {
			m.message = msg.err.Error()
		} else {
			m.items = msg.items
			m.message = ""
			m.cursor = 0
			for i, item := range m.visible() {
				if item.ID == id {
					m.cursor = i
					break
				}
			}
		}
		return m, menuTimer()
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			m.cancelled = true
			return m, tea.Quit
		}
		if m.searching {
			switch key {
			case "esc":
				m.searching = false
				m.query = ""
				m.cursor = 0
			case "enter":
				m.searching = false
			case "backspace":
				r := []rune(m.query)
				if len(r) > 0 {
					m.query = string(r[:len(r)-1])
				}
				m.cursor = 0
			default:
				if len(msg.Runes) > 0 {
					m.query += string(msg.Runes)
					m.cursor = 0
				}
			}
			return m, nil
		}
		items := m.visible()
		page := max(1, m.height-7)
		switch key {
		case "q", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "/":
			m.searching = true
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(max(0, len(items)-1), m.cursor+1)
		case "pgup":
			m.cursor = max(0, m.cursor-page)
		case "pgdown":
			m.cursor = min(max(0, len(items)-1), m.cursor+page)
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = max(0, len(items)-1)
		case "r":
			if m.load != nil && !m.loading {
				m.loading = true
				load := m.load
				return m, func() tea.Msg { items, err := load(); return menuItemsLoaded{items, err} }
			}
		case "enter":
			if len(items) > 0 {
				m.chosen = items[m.cursor]
				m.done = true
				return m, tea.Quit
			}
		default:
			if len(msg.Runes) > 0 && msg.Runes[0] == '/' {
				m.searching = true
				m.query = string(msg.Runes[1:])
				m.cursor = 0
			}
		}
	}
	return m, nil
}
func (m itemMenu) View() string {
	if m.done || m.cancelled {
		return ""
	}
	var b strings.Builder
	line := lipgloss.NewStyle().MaxWidth(max(10, m.width-2)).Inline(true)
	b.WriteString(line.Bold(true).Render(cleanTerminal(m.title)) + "\n")
	b.WriteString(line.Render(Text("↑/↓ Chọn · Enter Mở · / Tìm · r Làm mới · Esc Quay lại", "↑/↓ Select · Enter Open · / Search · r Refresh · Esc Back")) + "\n\n")
	items := m.visible()
	page := max(1, m.height-7)
	start := max(0, m.cursor-page+1)
	for i := start; i < min(len(items), start+page); i++ {
		prefix := "  "
		style := line
		if i == m.cursor {
			prefix = "› "
			style = style.Bold(true).Foreground(lipgloss.Color("33"))
		}
		b.WriteString(style.Render(prefix+cleanTerminal(items[i].Label)) + "\n")
	}
	if len(items) == 0 {
		b.WriteString(Text("Không có mục phù hợp", "No matching items") + "\n")
	}
	b.WriteString(fmt.Sprintf("\n%d/%d", min(m.cursor+1, len(items)), len(items)))
	if m.searching || m.query != "" {
		b.WriteString(" · /" + cleanTerminal(m.query))
	}
	if m.message != "" {
		b.WriteString("\n" + line.Render(cleanTerminal(m.message)))
	}
	return b.String()
}
func ChooseItems(title string, items []MenuItem, refresh func() ([]MenuItem, error)) (MenuItem, error) {
	m := itemMenu{title: title, items: items, load: refresh, width: 80, height: 24}
	result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return MenuItem{}, err
	}
	chosen := result.(itemMenu)
	if chosen.cancelled || !chosen.done {
		return MenuItem{}, ErrCancelled
	}
	return chosen.chosen, nil
}
