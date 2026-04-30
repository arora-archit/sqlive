package ui

import (
	"fmt"
	"strings"

	"sqlive/suggestion"

	"github.com/charmbracelet/lipgloss"
)

// SQL keyword sets for syntax highlighting
var sqlKeywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "AND": true, "OR": true,
	"ORDER": true, "BY": true, "LIMIT": true, "OFFSET": true, "GROUP": true,
	"HAVING": true, "DISTINCT": true, "INNER": true, "LEFT": true, "RIGHT": true,
	"FULL": true, "CROSS": true, "JOIN": true, "ON": true, "AS": true, "NOT": true,
	"IN": true, "BETWEEN": true, "LIKE": true, "IS": true, "NULL": true,
	"TRUE": true, "FALSE": true, "ASC": true, "DESC": true,
}

var sqlFunctions = map[string]bool{
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	"COALESCE": true, "UPPER": true, "LOWER": true, "LENGTH": true,
}

// highlightSQL colorizes a SQL string for display
func highlightSQL(sql string) string {
	kwStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")).Bold(true) // purple keywords
	fnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9e64")).Bold(true) // orange functions
	opStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))            // red operators
	strStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))           // green strings
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9e64"))           // number
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff"))            // cyan identifiers

	operators := map[string]bool{"=": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true, "*": true}

	tokens := tokenizeRaw(sql)
	var parts []string
	for _, tok := range tokens {
		upper := strings.ToUpper(tok)
		switch {
		case sqlKeywords[upper]:
			parts = append(parts, kwStyle.Render(tok))
		case sqlFunctions[upper]:
			parts = append(parts, fnStyle.Render(tok))
		case operators[tok]:
			parts = append(parts, opStyle.Render(tok))
		case len(tok) >= 2 && tok[0] == '\'':
			parts = append(parts, strStyle.Render(tok))
		case len(tok) > 0 && (tok[0] >= '0' && tok[0] <= '9'):
			parts = append(parts, numStyle.Render(tok))
		default:
			parts = append(parts, idStyle.Render(tok))
		}
	}
	return strings.Join(parts, " ")
}

// renderHeader renders the header with app name and database
func (m Model) renderHeader(width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00D9FF"))

	dbStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89"))

	headerBox := lipgloss.NewStyle().
		Width(width-4).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#bb9af7")).
		Padding(0, 1)

	header := titleStyle.Render(" SQLive") + " " + dbStyle.Render(m.dbPath)
	return headerBox.Render(header)
}

// renderInput renders the input field
func (m Model) renderInput(width int) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")).
		Bold(true)

	inputBox := lipgloss.NewStyle().
		Width(width-4).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#565f89")).
		Padding(0, 1)

	m.textInput.Width = width - 8
	return labelStyle.Render(" Query") + "\n" + inputBox.Render(m.textInput.View())
}

// suggestionContextLabel returns a short label for the current build context.
func (m Model) suggestionContextLabel() string {
	switch m.buildContext {
	case BuildSelect:
		if m.queryModel.Table != "" {
			return "SELECT"
		}
		return "START"
	case BuildWhere:
		return "WHERE"
	case BuildOrder:
		return "ORDER BY"
	case BuildGroup:
		return "GROUP BY"
	case BuildFunctionArg:
		if m.pendingFunction != "" {
			return m.pendingFunction + "( )"
		}
		return "FUNC ARG"
	}
	return ""
}

// renderSuggestions renders the suggestion list.
// availHeight is the total vertical space allocated for the suggestions box.
func (m Model) renderSuggestions(width, availHeight int) string {
	// innerWidth is the usable content width inside the box
	// box uses Width(width-4) + Padding(0,1) → inner = (width-4) - 2 = width-6
	innerWidth := width - 6

	// ── Styles ────────────────────────────────────────────────────────────────
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ece6a")).
		Bold(true)

	ctxPillStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5")).
		Background(lipgloss.Color("#2d3149")).
		Padding(0, 1)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#414868"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Italic(true)

	selArrow := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")).Bold(true).Render("▶")
	noArrow := " "

	// Per-type color + 2-char badge
	type typeInfo struct {
		color lipgloss.Color
		badge string
	}
	typeMap := map[suggestion.SuggestionType]typeInfo{
		suggestion.TypeKeyword:  {lipgloss.Color("#bb9af7"), "KW"},
		suggestion.TypeTable:    {lipgloss.Color("#ff9e64"), "TB"},
		suggestion.TypeColumn:   {lipgloss.Color("#7dcfff"), "CL"},
		suggestion.TypeOperator: {lipgloss.Color("#f7768e"), "OP"},
		suggestion.TypeValue:    {lipgloss.Color("#9ece6a"), "VA"},
		suggestion.TypeFunction: {lipgloss.Color("#e0af68"), "FN"},
	}

	borderColor := lipgloss.Color("#414868")
	if len(m.suggestions) > 0 {
		borderColor = lipgloss.Color("#565f89")
	}

	boxStyle := lipgloss.NewStyle().
		Width(width-4).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	// ── Title row ─────────────────────────────────────────────────────────────
	var b strings.Builder
	ctxLabel := m.suggestionContextLabel()
	title := titleStyle.Render(" Suggestions")
	if ctxLabel != "" {
		title += "  " + ctxPillStyle.Render(" "+ctxLabel+" ")
	}
	b.WriteString(title)
	b.WriteString("\n")

	// ── Empty state ───────────────────────────────────────────────────────────
	if len(m.suggestions) == 0 {
		b.WriteString(dimStyle.Render("  no suggestions"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  keep typing or use ↑↓"))
		return boxStyle.Render(b.String())
	}

	maxDisplay := availHeight - 4 // title(1) + bottom indicator(1) + border(2)
	if maxDisplay < 2 {
		maxDisplay = 2
	}
	start := m.suggestionScroll
	end := start + maxDisplay
	if end > len(m.suggestions) {
		end = len(m.suggestions)
	}

	// ── Scroll up indicator ───────────────────────────────────────────────────
	if start > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more above", start)))
		b.WriteString("\n")
	}

	// ── Suggestion rows ───────────────────────────────────────────────────────
	for i := start; i < end; i++ {
		s := m.suggestions[i]
		ti, ok := typeMap[s.Type]
		if !ok {
			ti = typeInfo{lipgloss.Color("#a9b1d6"), "??"}
		}
		itemStyle := lipgloss.NewStyle().Foreground(ti.color)
		isSelected := i == m.selectedIdx

		// pointer (1 char) + space (1) = 2
		arrow := noArrow + " "
		if isSelected {
			arrow = selArrow + " "
		}

		// badge (2 chars, bold + type color) + space (1) = 3
		badgeStr := itemStyle.Bold(true).Render(ti.badge) + " "

		// text — bold when selected
		var textStr string
		if isSelected {
			textStr = itemStyle.Bold(true).Render(s.Text)
		} else {
			textStr = itemStyle.Render(s.Text)
		}

		// description — only show if there is room (visual chars only, desc is plain)
		// used: 2(arrow) + 3(badge) + len(text) + 1(space) = 6 + len(text)
		descStr := ""
		if s.Description != "" {
			descMaxLen := innerWidth - 6 - len(s.Text)
			if descMaxLen > 3 {
				desc := s.Description
				if len(desc) > descMaxLen {
					desc = desc[:descMaxLen-1] + "…"
				}
				descStr = " " + descStyle.Render(desc)
			}
		}

		b.WriteString(arrow + badgeStr + textStr + descStr)
		b.WriteString("\n")
	}

	// ── Scroll down indicator / total count ───────────────────────────────────
	remaining := len(m.suggestions) - end
	if remaining > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
	} else if len(m.suggestions) > maxDisplay {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ─── %d items ───", len(m.suggestions))))
	}

	return boxStyle.Render(b.String())
}

// renderSQLPreview renders the generated SQL with syntax highlighting
func (m Model) renderSQLPreview(width int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#bb9af7")).
		Bold(true)

	sqlBox := lipgloss.NewStyle().
		Width(width-4).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#bb9af7")).
		Padding(0, 1)

	rawSQL := m.queryModel.ToSQL()
	var displaySQL string
	if rawSQL == "" {
		incompleteStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Italic(true)
		displaySQL = incompleteStyle.Render("incomplete query")
	} else {
		displaySQL = highlightSQL(rawSQL)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(" SQL"))
	b.WriteString("\n")
	b.WriteString(displaySQL)

	return sqlBox.Render(b.String())
}

// renderQueryHelp renders keybindings relevant to the query-building pane
func (m Model) renderQueryHelp(width int) string {
	helpBox := lipgloss.NewStyle().
		Width(width-4).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#414868")).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#8594d4")).
		Bold(true).
		Padding(0, 1)

	actionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8594d4"))

	sep := "  "

	var b strings.Builder
	b.WriteString(keyStyle.Render("↑↓") + " " + actionStyle.Render("navigate") + sep)
	b.WriteString(keyStyle.Render("Enter") + " " + actionStyle.Render("select") + sep)
	b.WriteString(keyStyle.Render("Ctrl+Z") + " " + actionStyle.Render("undo") + sep)
	b.WriteString(keyStyle.Render("Ctrl+E") + " " + actionStyle.Render("execute") + sep)
	b.WriteString(keyStyle.Render("Ctrl+R") + " " + actionStyle.Render("reset"))

	return helpBox.Render(b.String())
}

// renderResultHelp renders keybindings relevant to the result pane
func (m Model) renderResultHelp(width int) string {
	helpBox := lipgloss.NewStyle().
		Width(width-4).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#414868")).
		Padding(0, 1)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#8594d4")).
		Bold(true).
		Padding(0, 1)

	actionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ece6a")).
		Bold(true)

	sep := "  "

	var b strings.Builder
	b.WriteString(keyStyle.Render("↑↓←→") + " " + actionStyle.Render("scroll") + sep)
	b.WriteString(keyStyle.Render("Enter") + " " + actionStyle.Render("cell nav") + sep)
	b.WriteString(keyStyle.Render("Ctrl+Y") + " " + actionStyle.Render("copy cell") + sep)
	b.WriteString(keyStyle.Render("Ctrl+K") + " " + actionStyle.Render("copy row") + "\n")
	b.WriteString(keyStyle.Render("Ctrl+S") + " " + actionStyle.Render("export csv") + sep)
	b.WriteString(keyStyle.Render("Ctrl+J") + " " + actionStyle.Render("export json") + sep)
	b.WriteString(keyStyle.Render("Ctrl+]") + " " + actionStyle.Render("next page") + sep)
	b.WriteString(keyStyle.Render("Ctrl+[") + " " + actionStyle.Render("prev page"))

	if m.statusMsg != "" {
		b.WriteString("\n" + statusStyle.Render(" "+m.statusMsg))
	}

	return helpBox.Render(b.String())
}

// renderResults renders the query results in a table format.
// availHeight is the vertical space available for the results box.
func (m Model) renderResults(width, availHeight int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ece6a")).
		Bold(true)

	if m.executionError != "" {
		errorBox := lipgloss.NewStyle().
			Width(width-4).
			Height(availHeight-4).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#f7768e")).
			Padding(1, 1)

		errorIcon := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f7768e")).
			Bold(true)

		errorText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#db4b4b"))

		var b strings.Builder
		b.WriteString(errorIcon.Render(" Error"))
		b.WriteString("\n\n")
		b.WriteString(errorText.Render(m.executionError))
		return errorBox.Render(b.String())
	}

	if m.queryResults == nil {
		emptyBox := lipgloss.NewStyle().
			Width(width-4).
			Height(availHeight-4).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#565f89")).
			Padding(1, 1).
			Align(lipgloss.Center, lipgloss.Center)

		dimStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Italic(true)

		var helpText string
		if m.activePane == PaneResult {
			helpText = " Execute a query (Ctrl+E) to see results\n\nUse Tab to switch back to query pane"
		} else {
			helpText = " Execute a query (Ctrl+E) to see results"
		}
		return emptyBox.Render(dimStyle.Render(helpText))
	}

	resultBox := lipgloss.NewStyle().
		Width(width-4).
		Height(availHeight-4).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#9ece6a")).
		Padding(1, 1)

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf(" Results (%d rows)", len(m.queryResults.Rows))))
	b.WriteString("\n\n")

	if len(m.queryResults.Rows) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Italic(true)
		b.WriteString(emptyStyle.Render("No rows returned"))
		return resultBox.Render(b.String())
	}

	// Calculate available width for table
	availableWidth := width - 8 // Account for padding and borders

	// Calculate column widths
	colWidths := m.calculateColumnWidths(availableWidth)

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")).
		Bold(true)

	focusedHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7")).
		Bold(true)

	cellStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5"))

	focusedCellStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(lipgloss.Color("#7aa2f7")).
		Bold(true)

	altRowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a9b1d6"))

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89"))

	// Render header with horizontal scroll offset
	var headerParts []string
	startCol := 0
	endCol := len(m.queryResults.Columns)

	// Apply horizontal scroll to skip columns
	if m.resultScrollX > 0 {
		currentWidth := 0
		for i, w := range colWidths {
			if currentWidth >= m.resultScrollX {
				startCol = i
				break
			}
			currentWidth += w + 1
		}
	}

	// Find end column based on available width
	currentWidth := 0
	for i := startCol; i < len(m.queryResults.Columns); i++ {
		if currentWidth+colWidths[i] > availableWidth {
			endCol = i
			break
		}
		currentWidth += colWidths[i] + 1
	}

	// Determine focused cell (global indices, -1 when not in cell mode)
	focusCol := -1
	focusRow := -1
	if m.resultCellMode {
		focusCol = m.resultCursorCol
		focusRow = m.resultCursorRow
	}
	for i := startCol; i < endCol && i < len(m.queryResults.Columns); i++ {
		part := padOrTruncate(m.queryResults.Columns[i], colWidths[i])
		if i == focusCol {
			headerParts = append(headerParts, focusedHeaderStyle.Render(part))
		} else {
			headerParts = append(headerParts, headerStyle.Render(part))
		}
	}
	b.WriteString(strings.Join(headerParts, " "))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", currentWidth)))
	b.WriteString("\n")

	// Calculate max rows that fit in height
	maxRows := availHeight - 8
	if maxRows < 5 {
		maxRows = 5
	}

	// Apply vertical scroll
	startRow := m.resultScrollY
	endRow := startRow + maxRows
	if endRow > len(m.queryResults.Rows) {
		endRow = len(m.queryResults.Rows)
	}

	// Render rows with scrolling
	for i := startRow; i < endRow; i++ {
		row := m.queryResults.Rows[i]
		var rowParts []string
		for j := startCol; j < endCol && j < len(row); j++ {
			cellText := padOrTruncate(row[j], colWidths[j])
			if i == focusRow && j == focusCol {
				// Only highlight the exact focused cell
				rowParts = append(rowParts, focusedCellStyle.Render(cellText))
			} else if i%2 == 0 {
				rowParts = append(rowParts, cellStyle.Render(cellText))
			} else {
				rowParts = append(rowParts, altRowStyle.Render(cellText))
			}
		}
		b.WriteString(strings.Join(rowParts, " "))
		b.WriteString("\n")
	}

	// Show scroll and pagination indicators
	b.WriteString("\n")
	var scrollInfo string
	if m.activePane == PaneResult {
		scrollInfo = fmt.Sprintf(" Rows %d-%d of %d", startRow+1, endRow, len(m.queryResults.Rows))
		if startCol > 0 || endCol < len(m.queryResults.Columns) {
			scrollInfo += fmt.Sprintf(" | Cols %d-%d of %d", startCol+1, endCol, len(m.queryResults.Columns))
		}
		// Pagination
		if m.pageSize > 0 {
			page := m.currentPageNumber()
			totalPg := m.totalPages()
			if totalPg > 0 {
				scrollInfo += fmt.Sprintf(" | Page %d/%d", page, totalPg)
			} else {
				scrollInfo += fmt.Sprintf(" | Page %d (Ctrl+] next Ctrl+[ prev)", page)
			}
		}
		if m.totalRows > 0 {
			scrollInfo += fmt.Sprintf(" | Total: %d", m.totalRows)
		}
		scrollInfo += " | ←→↑↓ scroll PgUp/PgDn"
	} else {
		scrollInfo = fmt.Sprintf(" Showing %d of %d rows", endRow-startRow, len(m.queryResults.Rows))
		if m.totalRows > 0 {
			scrollInfo += fmt.Sprintf(" (total: %d)", m.totalRows)
		}
		scrollInfo += " | Ctrl+P to interact"
	}
	b.WriteString(dimStyle.Render(scrollInfo))

	return resultBox.Render(b.String())
}

// calculateColumnWidths calculates optimal column widths
func (m Model) calculateColumnWidths(availableWidth int) []int {
	if m.queryResults == nil || len(m.queryResults.Columns) == 0 {
		return nil
	}

	colWidths := make([]int, len(m.queryResults.Columns))

	// Start with header widths
	for i, col := range m.queryResults.Columns {
		colWidths[i] = len(col)
	}

	// Check data widths
	for _, row := range m.queryResults.Rows {
		for i, val := range row {
			if i < len(colWidths) && len(val) > colWidths[i] {
				colWidths[i] = len(val)
			}
		}
	}

	// Calculate total width needed
	totalWidth := 0
	for _, w := range colWidths {
		totalWidth += w
	}
	totalWidth += len(colWidths) - 1 // Spaces between columns

	// If total width exceeds available, scale down proportionally
	if totalWidth > availableWidth {
		scale := float64(availableWidth) / float64(totalWidth)
		for i := range colWidths {
			colWidths[i] = int(float64(colWidths[i]) * scale)
			if colWidths[i] < 3 {
				colWidths[i] = 3 // Minimum width
			}
		}
	}

	// If a cell column is focused, give it a little extra width and shrink others
	focus := -1
	if m.resultCellMode && m.resultCursorCol >= 0 && m.resultCursorCol < len(colWidths) {
		focus = m.resultCursorCol
	}
	if focus >= 0 {
		extra := availableWidth / 8 // give focused column ~1/8th of available width
		if extra < 6 {
			extra = 6
		}
		colWidths[focus] += extra
		// Recalculate total and shrink others proportionally if needed
		totalWidth = 0
		for _, w := range colWidths {
			totalWidth += w
		}
		totalWidth += len(colWidths) - 1
		if totalWidth > availableWidth {
			// amount to reduce from non-focused columns
			over := totalWidth - availableWidth
			nonFocusSum := 0
			for i, w := range colWidths {
				if i == focus {
					continue
				}
				nonFocusSum += w
			}
			if nonFocusSum <= 0 {
				// fallback: cap focused column
				colWidths[focus] = availableWidth - (len(colWidths)-1)*3
				if colWidths[focus] < 3 {
					colWidths[focus] = 3
				}
			} else {
				for i := range colWidths {
					if i == focus {
						continue
					}
					reduction := int(float64(colWidths[i]) / float64(nonFocusSum) * float64(over))
					colWidths[i] -= reduction
					if colWidths[i] < 3 {
						colWidths[i] = 3
					}
				}
			}
		}
	}

	return colWidths
}

// padOrTruncate pads or truncates a string to a specific width
func padOrTruncate(s string, width int) string {
	if len(s) > width {
		return s[:width-3] + "..."
	}
	return s + strings.Repeat(" ", width-len(s))
}
