package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Colorize applies the given color to the text using lipgloss.
// color is the integer representation from Modrinth.
func Colorize(text string, color int) string {
	// Convert Modrinth color int to hex string
	hexColor := fmt.Sprintf("#%06x", color)

	// Create a lipgloss style with the foreground color
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor))

	// Render the text with the style
	return style.Render(text)
}
