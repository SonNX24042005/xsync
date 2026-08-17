package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Lipgloss styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")) // Magenta

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")). // Gray
			Italic(true)

	activeItemStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")) // Bright Cyan

	inactiveItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")) // Off-white

	checkedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("82")) // Bright Green

	uncheckedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // Dark Gray

	confirmActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("82")) // Bright Green

	confirmInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)

const (
	CReset  = "\033[0m"
	CBold   = "\033[1m"
	CInfo   = "\033[36m"   // Cyan
	COk     = "\033[32m"   // Green
	CWarn   = "\033[33m"   // Yellow
	CErr    = "\033[31m"   // Red
	CAccent = "\033[35m"   // Magenta
)

func PrintHeader(title string) {
	fmt.Println(headerStyle.Render(fmt.Sprintf("\n=== %s ===", title)))
}

func PrintInfo(msg string) {
	fmt.Printf("  %s[INFO]%s %s\n", CInfo, CReset, msg)
}

func PrintOk(msg string) {
	fmt.Printf("  %s[OK]%s   %s\n", COk, CReset, msg)
}

func PrintWarn(msg string) {
	fmt.Printf("  %s[WARN]%s %s\n", CWarn, CReset, msg)
}

func PrintErr(msg string) {
	fmt.Printf("  %s[ERR]%s  %s\n", CErr, CReset, msg)
}

func PrintKV(key, val string) {
	fmt.Printf("  %s%-20s%s %s\n", CInfo, key, CReset, val)
}

// -----------------------------------------------------------------------------
// Bubbletea Menu Model
// -----------------------------------------------------------------------------

type menuModel struct {
	title      string
	options    []string
	cursor     int
	defaultIdx int
	selected   int
	quitting   bool
}

func initialMenuModel(title string, options []string, defaultIdx int) menuModel {
	cursor := 0
	if defaultIdx >= 0 && defaultIdx < len(options) {
		cursor = defaultIdx
	}
	return menuModel{
		title:      title,
		options:    options,
		cursor:     cursor,
		defaultIdx: defaultIdx,
		selected:   -1,
	}
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.options) - 1
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "enter":
			m.selected = m.cursor
			return m, tea.Quit
		default:
			// Allow quick number selection
			if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '9' {
				num := int(msg.String()[0] - '1')
				if num < len(m.options) {
					m.selected = num
					return m, tea.Quit
				}
			}
		}
	}
	return m, nil
}

func (m menuModel) View() string {
	if m.selected >= 0 || m.quitting {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render(fmt.Sprintf("=== %s ===", m.title)))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("  (Dung phim mui ten ↑ / ↓ de di chuyen, Enter de chon)"))
	sb.WriteString("\n\n")

	for i, opt := range m.options {
		suffix := ""
		if m.defaultIdx >= 0 && i == m.defaultIdx {
			suffix = " (Mac dinh)"
		}

		if m.cursor == i {
			line := fmt.Sprintf("  ▸ %d) %s%s", i+1, opt, suffix)
			sb.WriteString(activeItemStyle.Render(line))
		} else {
			line := fmt.Sprintf("    %d) %s%s", i+1, opt, suffix)
			sb.WriteString(inactiveItemStyle.Render(line))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// PromptMenu displays an interactive arrow-navigable menu using Bubble Tea.
func PromptMenu(title string, options []string, defaultIdx int) int {
	p := tea.NewProgram(initialMenuModel(title, options, defaultIdx))
	m, err := p.Run()
	if err != nil {
		return promptMenuFallback(title, options, defaultIdx)
	}

	model := m.(menuModel)
	if model.quitting || model.selected < 0 {
		fmt.Println("\n  [INFO] Huy bo boi nguoi dung.")
		os.Exit(130)
	}

	opt := options[model.selected]
	fmt.Printf("\n  %s✔ Da chon:%s %s%s%s\n", COk, CReset, CBold, opt, CReset)
	return model.selected
}

func promptMenuFallback(title string, options []string, defaultIdx int) int {
	PrintHeader(title)
	for i, opt := range options {
		suffix := ""
		if defaultIdx >= 0 && i == defaultIdx {
			suffix = " (Mac dinh)"
		}
		fmt.Printf("  %d) %s%s\n", i+1, opt, suffix)
	}

	hint := ""
	if defaultIdx >= 0 {
		hint = fmt.Sprintf(" [Mac dinh: %d]", defaultIdx+1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("  Nhap lua chon (1-%d)%s: ", len(options), hint)
		if !scanner.Scan() {
			fmt.Println("\n  [INFO] Huy bo boi nguoi dung.")
			os.Exit(130)
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			if defaultIdx >= 0 {
				return defaultIdx
			}
			continue
		}

		val, err := strconv.Atoi(input)
		if err != nil {
			PrintWarn("Vui long nhap mot so.")
			continue
		}

		idx := val - 1
		if idx >= 0 && idx < len(options) {
			return idx
		}
		PrintWarn(fmt.Sprintf("Lua chon khong hop le. Nhap tu 1 den %d.", len(options)))
	}
}

// -----------------------------------------------------------------------------
// Bubbletea Checklist Model
// -----------------------------------------------------------------------------

type ChecklistItem struct {
	Key     string
	Label   string
	Checked bool
}

type checklistModel struct {
	title    string
	items    []ChecklistItem
	selected []bool
	cursor   int
	quitting bool
	done     bool
}

func initialChecklistModel(title string, items []ChecklistItem) checklistModel {
	selected := make([]bool, len(items))
	for i, it := range items {
		selected[i] = it.Checked
	}
	return checklistModel{
		title:    title,
		items:    items,
		selected: selected,
		cursor:   0,
	}
}

func (m checklistModel) Init() tea.Cmd {
	return nil
}

func (m checklistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	totalRows := len(m.items) + 1 // items + confirm button

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = totalRows - 1
			}
		case "down", "j":
			if m.cursor < totalRows-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case " ":
			if m.cursor < len(m.items) {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "enter":
			if m.cursor == len(m.items) {
				m.done = true
				return m, tea.Quit
			}
			m.selected[m.cursor] = !m.selected[m.cursor]
		default:
			if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '9' {
				idx := int(msg.String()[0] - '1')
				if idx < len(m.items) {
					m.selected[idx] = !m.selected[idx]
					m.cursor = idx
				}
			}
		}
	}
	return m, nil
}

func (m checklistModel) View() string {
	if m.done || m.quitting {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render(fmt.Sprintf("=== %s ===", m.title)))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("  (Dung ↑ / ↓ de di chuyen, Space de bat/tat, Enter tren [Xac nhan] de luu)"))
	sb.WriteString("\n\n")

	for i, item := range m.items {
		status := uncheckedStyle.Render("[ ]")
		if m.selected[i] {
			status = checkedStyle.Render("[X]")
		}

		if m.cursor == i {
			line := fmt.Sprintf("  ▸ %s %d) %s", status, i+1, item.Label)
			sb.WriteString(activeItemStyle.Render(line))
		} else {
			line := fmt.Sprintf("    %s %d) %s", status, i+1, item.Label)
			sb.WriteString(inactiveItemStyle.Render(line))
		}
		sb.WriteString("\n")
	}

	// Confirm button
	sb.WriteString("\n")
	if m.cursor == len(m.items) {
		sb.WriteString(confirmActiveStyle.Render("  ▸ [ ✔  Xac nhan va tiep tuc ]"))
	} else {
		sb.WriteString(confirmInactiveStyle.Render("    [ ✔  Xac nhan va tiep tuc ]"))
	}
	sb.WriteString("\n")

	return sb.String()
}

// PromptChecklist displays an interactive toggleable checklist using Bubble Tea.
func PromptChecklist(title string, items []ChecklistItem) []string {
	p := tea.NewProgram(initialChecklistModel(title, items))
	m, err := p.Run()
	if err != nil {
		return promptChecklistFallback(title, items)
	}

	model := m.(checklistModel)
	if model.quitting {
		fmt.Println("\n  [INFO] Huy bo boi nguoi dung.")
		os.Exit(130)
	}

	var result []string
	for i, sel := range model.selected {
		if sel {
			result = append(result, items[i].Key)
		}
	}
	return result
}

func promptChecklistFallback(title string, items []ChecklistItem) []string {
	selected := make([]bool, len(items))
	for i, item := range items {
		selected[i] = item.Checked
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		PrintHeader(title)
		for i, item := range items {
			status := "[ ]"
			if selected[i] {
				status = "[X]"
			}
			fmt.Printf("  %s %d) %s\n", status, i+1, item.Label)
		}
		fmt.Println("\n  Nhap so de chon/bo chon (vi du: '1 2'), hoac an Enter de xac nhan.")
		fmt.Print("  Lua chon: ")

		if !scanner.Scan() {
			fmt.Println("\n  [INFO] Huy bo boi nguoi dung.")
			os.Exit(130)
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			break
		}

		fields := strings.Fields(input)
		for _, f := range fields {
			if idx, err := strconv.Atoi(f); err == nil {
				idx = idx - 1
				if idx >= 0 && idx < len(items) {
					selected[idx] = !selected[idx]
				}
			}
		}
	}

	var result []string
	for i, sel := range selected {
		if sel {
			result = append(result, items[i].Key)
		}
	}
	return result
}

func getSystemEditor() string {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor
	}
	for _, candidate := range []string{"nano", "vim", "vi"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return "nano"
}

// EditTextInEditor launches system editor on a temp file with initial content and returns updated content.
func EditTextInEditor(content, suffix string) (string, error) {
	tempFile, err := os.CreateTemp("", "*"+suffix)
	if err != nil {
		return content, err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return content, err
	}
	tempFile.Close()

	editor := getSystemEditor()
	cmd := exec.Command(editor, tempPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		PrintErr(fmt.Sprintf("Editor %s gap loi. Su dung noi dung mac dinh.", editor))
		return content, err
	}

	data, err := os.ReadFile(tempPath)
	if err != nil {
		return content, err
	}
	return string(data), nil
}

// OpenFileInEditor opens a file with the system editor.
func OpenFileInEditor(filePath string) error {
	editor := getSystemEditor()
	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
