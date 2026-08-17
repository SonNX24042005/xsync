package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
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

// PromptMenu displays an interactive arrow-navigable menu using promptui.
func PromptMenu(title string, options []string, defaultIdx int) int {
	cursorPos := 0
	if defaultIdx >= 0 && defaultIdx < len(options) {
		cursorPos = defaultIdx
	}

	displayItems := make([]string, len(options))
	for i, opt := range options {
		suffix := ""
		if defaultIdx >= 0 && i == defaultIdx {
			suffix = " (Mac dinh)"
		}
		displayItems[i] = fmt.Sprintf("%d) %s%s", i+1, opt, suffix)
	}

	templates := &promptui.SelectTemplates{
		Label:    "{{ . | magenta }}:",
		Active:   "\033[1;36m▸ {{ . | cyan }}\033[0m",
		Inactive: "  {{ . }}",
		Selected: "\033[32m✔\033[0m {{ . | bold }}",
	}

	prompt := promptui.Select{
		Label:        title,
		Items:        displayItems,
		CursorPos:    cursorPos,
		Size:         10,
		Templates:    templates,
		HideSelected: false,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt {
			fmt.Println("\n  [INFO] Huy bo boi nguoi dung.")
			os.Exit(130)
		}
		return promptMenuFallback(title, options, defaultIdx)
	}

	return idx
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
func PromptChecklist(title string, items []ChecklistItem) []string {
	selected := make([]bool, len(items))
	for i, item := range items {
		selected[i] = item.Checked
	}

	cursorPos := 0
	for {
		var menuOptions []string
		for i, item := range items {
			status := "[ ]"
			if selected[i] {
				status = "[X]"
			}
			menuOptions = append(menuOptions, fmt.Sprintf("%s %d) %s", status, i+1, item.Label))
		}
		menuOptions = append(menuOptions, "✔  Xac nhan va tiep tuc")

		templates := &promptui.SelectTemplates{
			Label:    "{{ . | magenta }}:",
			Active:   "\033[1;36m▸ {{ . | cyan }}\033[0m",
			Inactive: "  {{ . }}",
			Selected: "\033[32m✔\033[0m {{ . | bold }}",
		}

		prompt := promptui.Select{
			Label:        title + " (Chon de bat/tat [X], chon 'Xac nhan' khi xong)",
			Items:        menuOptions,
			CursorPos:    cursorPos,
			Size:         len(menuOptions) + 2,
			Templates:    templates,
			HideSelected: true,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			if err == promptui.ErrInterrupt {
				fmt.Println("\n  [INFO] Huy bo boi nguoi dung.")
				os.Exit(130)
			}
			return promptChecklistFallback(title, items)
		}

		if idx == len(items) {
			break
		}

		selected[idx] = !selected[idx]
		cursorPos = idx
	}

	var result []string
	for i, sel := range selected {
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
