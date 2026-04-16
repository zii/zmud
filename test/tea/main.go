package main

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

func main() {
	var style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingTop(1).
		MarginLeft(4).
		Width(10).Border(lipgloss.RoundedBorder())

	fmt.Println(style.Render("Hello, Lip Gloss!"))
}
