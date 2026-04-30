package ui

import (
	"sqlive/suggestion"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ClearStatus removes any status/notification message
func (m *Model) ClearStatus() {
	m.statusMsg = ""
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "ctrl+e":
			// Execute query
			if m.queryModel.IsValid() {
				m.executeQuery()
				// Auto-switch to result pane after execution
				m.activePane = PaneResult
				m.textInput.Blur()
			}
			return m, nil

		case "ctrl+r":
			// Clear/reset query
			m.queryModel.Reset()
			m.textInput.SetValue("")
			m.queryResults = nil
			m.executionError = ""
			m.statusMsg = ""
			m.buildContext = BuildSelect
			m.whereColumn = ""
			m.whereOperator = ""
			m.pageOffset = 0
			m.pageSize = 0
			m.totalRows = 0
			m.updateSuggestions()
			return m, nil

		case "ctrl+s":
			// Export results as CSV (result pane only)
			if m.activePane == PaneResult {
				m.statusMsg = m.exportCSV()
				return m, nil
			}

		case "ctrl+j":
			// Export results as JSON (result pane only)
			if m.activePane == PaneResult {
				m.statusMsg = m.exportJSON()
				return m, nil
			}

		case "ctrl+]":
			// Next page (result pane only)
			if m.activePane == PaneResult && m.queryModel.IsValid() {
				m.executeNextPage()
				return m, nil
			}

		case "ctrl+[":
			// Previous page (result pane only)
			if m.activePane == PaneResult && m.queryModel.IsValid() {
				m.executePrevPage()
				return m, nil
			}

		case "ctrl+backspace", "ctrl+z":
			// Delete last word/token (undo last selection)
			currentInput := m.textInput.Value()
			if currentInput == "" {
				return m, nil
			}

			// Trim trailing spaces
			currentInput = strings.TrimRight(currentInput, " ")

			// Find the last space
			lastSpace := strings.LastIndex(currentInput, " ")
			if lastSpace == -1 {
				// No space found, clear everything
				m.textInput.SetValue("")
			} else {
				// Remove last word
				m.textInput.SetValue(currentInput[:lastSpace+1])
			}

			m.textInput.CursorEnd()
			m.updateSuggestions()
			return m, nil

		case "up":
			// Navigate suggestions up or move within results
			if m.activePane == PaneResult && m.queryResults != nil {
				if m.resultCellMode {
					if m.resultCursorRow > 0 {
						m.resultCursorRow--
					}
					// Ensure focused row is visible
					if m.resultCursorRow < m.resultScrollY {
						m.resultScrollY = m.resultCursorRow
					}
				} else {
					if m.resultScrollY > 0 {
						m.resultScrollY--
					}
				}
				return m, nil
			}

			if len(m.suggestions) > 0 {
				m.selectedIdx--
				if m.selectedIdx < 0 {
					m.selectedIdx = len(m.suggestions) - 1
				}
				// Adjust scroll
				if m.selectedIdx < m.suggestionScroll {
					m.suggestionScroll = m.selectedIdx
				}
			}
			return m, nil

		case "down":
			// Navigate suggestions down or move within results
			if m.activePane == PaneResult && m.queryResults != nil {
				if m.resultCellMode {
					if m.resultCursorRow < len(m.queryResults.Rows)-1 {
						m.resultCursorRow++
					}
					// Ensure focused row is visible
					pageSize := m.getMaxResultRows()
					if m.resultCursorRow >= m.resultScrollY+pageSize {
						m.resultScrollY = m.resultCursorRow - pageSize + 1
					}
				} else {
					maxScroll := len(m.queryResults.Rows) - m.getMaxResultRows()
					if maxScroll < 0 {
						maxScroll = 0
					}
					if m.resultScrollY < maxScroll {
						m.resultScrollY++
					}
				}
				return m, nil
			}

			if len(m.suggestions) > 0 {
				m.selectedIdx++
				if m.selectedIdx >= len(m.suggestions) {
					m.selectedIdx = 0
				}
				// Adjust scroll
				maxVisible := m.getMaxSuggestions()
				if m.selectedIdx >= m.suggestionScroll+maxVisible {
					m.suggestionScroll = m.selectedIdx - maxVisible + 1
				}
				if m.selectedIdx == 0 {
					m.suggestionScroll = 0
				}
			}
			return m, nil

		case "left":
			// Move left in results or scroll
			if m.activePane == PaneResult && m.queryResults != nil {
				if m.resultCellMode {
					if m.resultCursorCol > 0 {
						m.resultCursorCol--
					}
					// Ensure focused column is visible
					rightWidth := m.width - m.width/2
					availableWidth := rightWidth - 8
					colWidths := m.calculateColumnWidths(availableWidth)
					cum := 0
					for i := 0; i < m.resultCursorCol && i < len(colWidths); i++ {
						cum += colWidths[i] + 1
					}
					if cum < m.resultScrollX {
						m.resultScrollX = cum
					}
				} else {
					if m.resultScrollX > 0 {
						m.resultScrollX -= 5
						if m.resultScrollX < 0 {
							m.resultScrollX = 0
						}
					}
				}
				return m, nil
			}

		case "right":
			// Move right in results or scroll
			if m.activePane == PaneResult && m.queryResults != nil {
				if m.resultCellMode {
					if m.resultCursorCol < len(m.queryResults.Columns)-1 {
						m.resultCursorCol++
					}
					// Ensure focused column is visible
					rightWidth := m.width - m.width/2
					availableWidth := rightWidth - 8
					colWidths := m.calculateColumnWidths(availableWidth)
					cum := 0
					for i := 0; i < m.resultCursorCol && i < len(colWidths); i++ {
						cum += colWidths[i] + 1
					}
					colW := 0
					if m.resultCursorCol < len(colWidths) {
						colW = colWidths[m.resultCursorCol]
					}
					if cum+colW > m.resultScrollX+availableWidth {
						m.resultScrollX = cum + colW - availableWidth
						if m.resultScrollX < 0 {
							m.resultScrollX = 0
						}
					}
				} else {
					m.resultScrollX += 5
				}
				return m, nil
			}

		case "pageup":
			// Page up in results
			if m.activePane == PaneResult && m.queryResults != nil {
				pageSize := m.getMaxResultRows()
				m.resultScrollY -= pageSize
				if m.resultScrollY < 0 {
					m.resultScrollY = 0
				}
				return m, nil
			}

		case "pagedown":
			// Page down in results
			if m.activePane == PaneResult && m.queryResults != nil {
				pageSize := m.getMaxResultRows()
				maxScroll := len(m.queryResults.Rows) - pageSize
				if maxScroll < 0 {
					maxScroll = 0
				}
				m.resultScrollY += pageSize
				if m.resultScrollY > maxScroll {
					m.resultScrollY = maxScroll
				}
				return m, nil
			}

		case "home":
			// Jump to top/left
			if m.activePane == PaneResult && m.queryResults != nil {
				m.resultScrollY = 0
				m.resultScrollX = 0
				return m, nil
			}

		case "end":
			// Jump to bottom
			if m.activePane == PaneResult && m.queryResults != nil {
				maxScroll := len(m.queryResults.Rows) - m.getMaxResultRows()
				if maxScroll < 0 {
					maxScroll = 0
				}
				m.resultScrollY = maxScroll
				return m, nil
			}

		case "ctrl+p":
			// Tab switches between panes
			if m.activePane == PaneQuery {
				m.activePane = PaneResult
				m.textInput.Blur()
			} else {
				m.activePane = PaneQuery
				m.textInput.Focus()
			}
			return m, nil

		case "ctrl+y":
			// Copy focused cell to clipboard (result pane cell mode)
			if m.activePane == PaneResult && m.resultCellMode {
				m.statusMsg = m.copyCellToClipboard()
				return m, nil
			}

		case "ctrl+k":
			// Copy focused row to clipboard
			if m.activePane == PaneResult && m.resultCellMode {
				m.statusMsg = m.copyRowToClipboard()
				return m, nil
			}

		case "tab", "enter":
			// Enter/Tab: accept suggestion in query pane or toggle result cell navigation
			if m.activePane == PaneQuery && len(m.suggestions) > 0 && m.selectedIdx < len(m.suggestions) {
				m.acceptSuggestion(m.suggestions[m.selectedIdx])
				m.updateSuggestions()
			}
			if m.activePane == PaneResult && m.queryResults != nil {
				// Toggle cell navigation mode
				m.resultCellMode = !m.resultCellMode
				// Clamp cursor positions
				if m.resultCursorRow >= len(m.queryResults.Rows) {
					m.resultCursorRow = len(m.queryResults.Rows) - 1
				}
				if m.resultCursorRow < 0 {
					m.resultCursorRow = 0
				}
				if m.resultCursorCol >= len(m.queryResults.Columns) {
					m.resultCursorCol = len(m.queryResults.Columns) - 1
				}
				if m.resultCursorCol < 0 {
					m.resultCursorCol = 0
				}
				// Ensure focused cell is visible
				rightWidth := m.width - m.width/2
				availableWidth := rightWidth - 8
				colWidths := m.calculateColumnWidths(availableWidth)
				cum := 0
				for i := 0; i < m.resultCursorCol && i < len(colWidths); i++ {
					cum += colWidths[i] + 1
				}
				colW := 0
				if m.resultCursorCol < len(colWidths) {
					colW = colWidths[m.resultCursorCol]
				}
				if cum < m.resultScrollX {
					m.resultScrollX = cum
				} else if cum+colW > m.resultScrollX+availableWidth {
					m.resultScrollX = cum + colW - availableWidth
					if m.resultScrollX < 0 {
						m.resultScrollX = 0
					}
				}
				// Ensure row visible
				pageSize := m.getMaxResultRows()
				if m.resultCursorRow < m.resultScrollY {
					m.resultScrollY = m.resultCursorRow
				} else if m.resultCursorRow >= m.resultScrollY+pageSize {
					m.resultScrollY = m.resultCursorRow - pageSize + 1
				}
			}
			return m, nil

		default:
			// Update text input only if in query pane
			if m.activePane == PaneQuery {
				m.textInput, cmd = m.textInput.Update(msg)
				m.updateSuggestions()
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	if m.activePane == PaneQuery {
		m.textInput, cmd = m.textInput.Update(msg)
		m.updateSuggestions()
	}
	return m, cmd
}

// acceptSuggestion inserts the selected suggestion into the input
func (m *Model) acceptSuggestion(s suggestion.Suggestion) {
	// Record usage for future ranking
	m.engine.RecordUsage(s.Text)
	currentInput := m.textInput.Value()
	cursorPos := m.textInput.Position()

	// Find the start of the current token
	start := cursorPos
	for start > 0 && currentInput[start-1] != ' ' {
		start--
	}

	// ── Case 1: bare aggregate function (e.g. SUM) ──────────────────────────────
	// Insert "FUNC(" so the open bracket appears in the text.
	if s.Type == suggestion.TypeFunction && !strings.Contains(s.Text, "(") {
		newInput := currentInput[:start] + s.Text + "("
		if cursorPos < len(currentInput) {
			newInput += currentInput[cursorPos:]
		}
		m.textInput.SetValue(newInput)
		m.textInput.SetCursor(start + len(s.Text) + 1)
		m.parseInput()
		// Override AFTER parseInput so the state survives its reset
		m.buildContext = BuildFunctionArg
		m.pendingFunction = s.Text
		return
	}

	// ── Case 2: column chosen as function argument ────────────────────────────────
	// Detect by inspecting the current token directly: if it contains "(" but no ")"
	// it is an open function call regardless of buildContext state.
	currentToken := currentInput[start:cursorPos]
	parenIdx := strings.Index(currentToken, "(")
	if parenIdx > 0 && !strings.Contains(currentToken, ")") && s.Type == suggestion.TypeColumn {
		fnName := currentToken[:parenIdx]
		newInput := currentInput[:start] + fnName + "(" + s.Text + ") "
		if cursorPos < len(currentInput) {
			newInput += currentInput[cursorPos:]
		}
		m.pendingFunction = ""
		m.buildContext = BuildSelect
		m.textInput.SetValue(newInput)
		m.textInput.SetCursor(start + len(fnName) + 1 + len(s.Text) + 2)
		m.parseInput()
		return
	}

	// ── Default: replace current token with suggestion text ─────────────────────
	newInput := currentInput[:start] + s.Text + " "
	if cursorPos < len(currentInput) {
		newInput += currentInput[cursorPos:]
	}

	// Special handling for '' - position cursor between quotes
	if s.Text == "''" {
		newInput = currentInput[:start] + "''"
		if cursorPos < len(currentInput) {
			newInput += " " + currentInput[cursorPos:]
		}
		m.textInput.SetValue(newInput)
		m.textInput.SetCursor(start + 1) // Position between quotes
	} else {
		m.textInput.SetValue(newInput)
		m.textInput.SetCursor(start + len(s.Text) + 1)
	}

	// Update query model based on suggestion
	m.updateQueryModel(s)

	// Parse the full input to capture any typed values
	m.parseInput()
}

// updateSuggestions refreshes the suggestion list
func (m *Model) updateSuggestions() {
	input := m.textInput.Value()
	cursorPos := m.textInput.Position()

	m.suggestions = m.engine.GetSuggestions(input, cursorPos)

	// Reset selection if suggestions changed
	if m.selectedIdx >= len(m.suggestions) {
		m.selectedIdx = 0
	}
	m.suggestionScroll = 0

	// Parse input to update query model with typed values
	m.parseInput()
}
