package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m Model) View() string {
	// Split pane layout: left for query building, right for results
	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth

	// Left pane: Header, Input, Suggestions, SQL Preview, Help
	leftPane := m.renderLeftPane(leftWidth)

	// Right pane: Query Results
	rightPane := m.renderRightPane(rightWidth)

	// Join panes side by side
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

// renderLeftPane renders the left side of the split pane
func (m Model) renderLeftPane(width int) string {
	// Account for pane border (1) + padding left/right (2) = 3 chars overhead
	innerWidth := width - 3

	header := m.renderHeader(innerWidth)
	input := m.renderInput(innerWidth)
	help := m.renderQueryHelp(innerWidth)

	// Reserve space for header, input, help, and 3 newline separators
	headerH := lipgloss.Height(header)
	inputH := lipgloss.Height(input)
	helpH := lipgloss.Height(help)
	availSugH := m.height - headerH - inputH - helpH - 3
	if availSugH < 3 {
		availSugH = 3
	}
	suggestions := m.renderSuggestions(innerWidth, availSugH)

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(input)
	b.WriteString("\n")
	b.WriteString(suggestions)
	b.WriteString("\n")
	b.WriteString(help)

	content := b.String()

	// Highlight active pane
	borderColor := "#414868"
	if m.activePane == PaneQuery {
		borderColor = "#7aa2f7"
	}

	paneStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Height(m.height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		BorderRight(true).
		Padding(0, 1)

	return paneStyle.Render(content)
}

// renderRightPane renders the right side of the split pane
func (m Model) renderRightPane(width int) string {
	// Highlight active pane
	borderColor := "#414868"
	if m.activePane == PaneResult {
		borderColor = "#7aa2f7"
	}

	// Account for pane border (1) + padding left/right (2) = 3 chars overhead
	innerWidth := width - 3

	paneStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Height(m.height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		BorderLeft(true).
		Padding(0, 1)

	// Measure fixed components first, give the remainder to the results box.
	sqlPreview := m.renderSQLPreview(innerWidth)
	resultHelp := m.renderResultHelp(innerWidth)
	sqlH := lipgloss.Height(sqlPreview)
	helpH := lipgloss.Height(resultHelp)
	// 2 newline separators between the three sections
	availH := m.height - sqlH - helpH - 2
	if availH < 4 {
		availH = 4
	}
	results := m.renderResults(innerWidth, availH)

	var content strings.Builder
	content.WriteString(sqlPreview)
	content.WriteString("\n")
	content.WriteString(results)
	content.WriteString("\n")
	content.WriteString(resultHelp)

	return paneStyle.Render(content.String())
}
