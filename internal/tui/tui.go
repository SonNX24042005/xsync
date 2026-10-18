package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	activeItemStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))
)

func PrintErr(msg string) {
	fmt.Printf("  \033[31m[ERR]\033[0m  %s\n", msg)
}
