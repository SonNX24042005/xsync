package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
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
	fmt.Printf("\n%s=== %s ===%s\n", CAccent, title, CReset)
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

// PromptMenu displays an interactive arrow-navigable menu.
// Up/Down arrows or j/k to navigate, Enter to select, 1-9 for quick shortcut.
func PromptMenu(title string, options []string, defaultIdx int) int {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return promptMenuFallback(title, options, defaultIdx)
	}

	selectedIdx := 0
	if defaultIdx >= 0 && defaultIdx < len(options) {
		selectedIdx = defaultIdx
	}

	PrintHeader(title)
	fmt.Printf("  %s(Dung phim mui ten ↑ / ↓ de di chuyen, Enter de chon)%s\n\n", CInfo, CReset)

	// Render lines initially
	renderMenuLines(options, selectedIdx, defaultIdx, false)

	oldState, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		return promptMenuFallback(title, options, defaultIdx)
	}
	defer func() {
		_ = term.Restore(int(syscall.Stdin), oldState)
		fmt.Print("\033[?25h") // Show cursor
	}()

	fmt.Print("\033[?25l") // Hide cursor

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		if n == 1 {
			b := buf[0]
			if b == 3 { // Ctrl+C
				_ = term.Restore(int(syscall.Stdin), oldState)
				fmt.Print("\033[?25h\n\n  [INFO] Huy bo boi nguoi dung.\n")
				os.Exit(130)
			}
			if b == 13 || b == 10 { // Enter
				break
			}
			if b == 'k' || b == 'K' {
				selectedIdx = (selectedIdx - 1 + len(options)) % len(options)
				renderMenuLines(options, selectedIdx, defaultIdx, true)
			} else if b == 'j' || b == 'J' {
				selectedIdx = (selectedIdx + 1) % len(options)
				renderMenuLines(options, selectedIdx, defaultIdx, true)
			} else if b >= '1' && b <= '9' {
				num := int(b - '1')
				if num < len(options) {
					selectedIdx = num
					break
				}
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65: // Up Arrow
				selectedIdx = (selectedIdx - 1 + len(options)) % len(options)
				renderMenuLines(options, selectedIdx, defaultIdx, true)
			case 66: // Down Arrow
				selectedIdx = (selectedIdx + 1) % len(options)
				renderMenuLines(options, selectedIdx, defaultIdx, true)
			}
		}
	}

	_ = term.Restore(int(syscall.Stdin), oldState)
	fmt.Print("\033[?25h\n")
	return selectedIdx
}

func renderMenuLines(options []string, selectedIdx, defaultIdx int, moveUp bool) {
	if moveUp {
		fmt.Printf("\033[%dF", len(options))
	}
	for i, opt := range options {
		suffix := ""
		if defaultIdx >= 0 && i == defaultIdx {
			suffix = " (Mac dinh)"
		}

		if i == selectedIdx {
			fmt.Printf("\033[K  %s%s▸ %d) %s%s%s\n", CBold, CInfo, i+1, opt, suffix, CReset)
		} else {
			fmt.Printf("\033[K    %d) %s%s\n", i+1, opt, suffix)
		}
	}
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

type ChecklistItem struct {
	Key     string
	Label   string
	Checked bool
}

// PromptChecklist displays an interactive toggleable checklist.
// Up/Down arrows to move cursor, Space to toggle, Enter to submit.
func PromptChecklist(title string, items []ChecklistItem) []string {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return promptChecklistFallback(title, items)
	}

	selected := make([]bool, len(items))
	for i, item := range items {
		selected[i] = item.Checked
	}

	cursorIdx := 0

	PrintHeader(title)
	fmt.Printf("  %s(Dung ↑ / ↓ de di chuyen, Phim cach (Space) de chon/bo chon, Enter de xac nhan)%s\n\n", CInfo, CReset)

	renderChecklistLines(items, selected, cursorIdx, false)

	oldState, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		return promptChecklistFallback(title, items)
	}
	defer func() {
		_ = term.Restore(int(syscall.Stdin), oldState)
		fmt.Print("\033[?25h")
	}()

	fmt.Print("\033[?25l")

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		if n == 1 {
			b := buf[0]
			if b == 3 { // Ctrl+C
				_ = term.Restore(int(syscall.Stdin), oldState)
				fmt.Print("\033[?25h\n\n  [INFO] Huy bo boi nguoi dung.\n")
				os.Exit(130)
			}
			if b == 13 || b == 10 { // Enter
				break
			}
			if b == ' ' { // Space: toggle
				selected[cursorIdx] = !selected[cursorIdx]
				renderChecklistLines(items, selected, cursorIdx, true)
			} else if b == 'k' || b == 'K' {
				cursorIdx = (cursorIdx - 1 + len(items)) % len(items)
				renderChecklistLines(items, selected, cursorIdx, true)
			} else if b == 'j' || b == 'J' {
				cursorIdx = (cursorIdx + 1) % len(items)
				renderChecklistLines(items, selected, cursorIdx, true)
			} else if b >= '1' && b <= '9' {
				idx := int(b - '1')
				if idx < len(items) {
					selected[idx] = !selected[idx]
					cursorIdx = idx
					renderChecklistLines(items, selected, cursorIdx, true)
				}
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65: // Up Arrow
				cursorIdx = (cursorIdx - 1 + len(items)) % len(items)
				renderChecklistLines(items, selected, cursorIdx, true)
			case 66: // Down Arrow
				cursorIdx = (cursorIdx + 1) % len(items)
				renderChecklistLines(items, selected, cursorIdx, true)
			}
		}
	}

	_ = term.Restore(int(syscall.Stdin), oldState)
	fmt.Print("\033[?25h\n")

	var result []string
	for i, sel := range selected {
		if sel {
			result = append(result, items[i].Key)
		}
	}
	return result
}

func renderChecklistLines(items []ChecklistItem, selected []bool, cursorIdx int, moveUp bool) {
	if moveUp {
		fmt.Printf("\033[%dF", len(items))
	}
	for i, item := range items {
		status := "[ ]"
		if selected[i] {
			status = fmt.Sprintf("[%sX%s]", COk, CReset)
		}

		if i == cursorIdx {
			fmt.Printf("\033[K  %s%s▸ %s %d) %s%s\n", CBold, CInfo, status, i+1, item.Label, CReset)
		} else {
			fmt.Printf("\033[K    %s %d) %s\n", status, i+1, item.Label)
		}
	}
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
