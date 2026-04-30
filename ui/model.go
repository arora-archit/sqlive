package ui

import (
	"sqlive/query"
	"sqlive/schema"
	"sqlive/suggestion"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// BuildContext tracks what part of the query is being built
type BuildContext string

const (
	BuildSelect      BuildContext = "select"
	BuildWhere       BuildContext = "where"
	BuildOrder       BuildContext = "orderby"
	BuildGroup       BuildContext = "groupby"
	BuildFunctionArg BuildContext = "functionarg" // waiting for an aggregate function's column argument
)

// Pane represents which pane is active
type Pane string

const (
	PaneQuery  Pane = "query"
	PaneResult Pane = "result"
)

// Model represents the Bubble Tea UI model
type Model struct {
	schema           *schema.Schema
	queryModel       *query.Model
	engine           *suggestion.Engine
	textInput        textinput.Model
	suggestions      []suggestion.Suggestion
	selectedIdx      int
	suggestionScroll int
	resultScrollY    int
	resultScrollX    int
	resultCursorRow  int
	resultCursorCol  int
	resultCellMode   bool
	dbPath           string
	width            int
	height           int
	queryResults     *QueryResults

	// Total count (set after execute for pagination info)
	totalRows int

	// Current page offset (rows skipped)
	pageOffset int

	// Page size used for last query (equals the LIMIT in model, or 0 = no limit)
	pageSize int

	executionError  string
	buildContext    BuildContext
	whereColumn     string
	whereOperator   query.Operator
	activePane      Pane
	pendingFunction string // name of aggregate function waiting for its column arg

	// Status/notification message (export confirmation, copy notice, etc.)
	statusMsg string
}

// QueryResults holds the results of an executed query
type QueryResults struct {
	Columns []string
	Rows    [][]string
}

// NewModel creates a new UI model
func NewModel(dbPath string, s *schema.Schema) Model {
	ti := textinput.New()
	ti.Placeholder = "Type your SQL query..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 80

	qm := query.NewModel()
	engine := suggestion.NewEngine(s, qm)
	engine.SetDBPath(dbPath)

	return Model{
		schema:           s,
		queryModel:       qm,
		engine:           engine,
		textInput:        ti,
		suggestions:      []suggestion.Suggestion{},
		selectedIdx:      0,
		suggestionScroll: 0,
		resultScrollY:    0,
		resultScrollX:    0,
		resultCursorRow:  0,
		resultCursorCol:  0,
		resultCellMode:   false,
		dbPath:           dbPath,
		width:            80,
		height:           24,
		buildContext:     BuildSelect,
		activePane:       PaneQuery,
		pageOffset:       0,
		pageSize:         0,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// getMaxSuggestions returns the max suggestions to display based on height
func (m Model) getMaxSuggestions() int {
	available := m.height - 15
	if available < 3 {
		return 3
	}
	if available > 10 {
		return 10
	}
	return available
}

// getMaxResultRows returns the max result rows to display based on height
func (m Model) getMaxResultRows() int {
	available := m.height - 8
	if available < 5 {
		return 5
	}
	return available
}

// currentPageNumber returns the 1-based current page number
func (m Model) currentPageNumber() int {
	if m.pageSize <= 0 {
		return 1
	}
	return m.pageOffset/m.pageSize + 1
}

// totalPages returns the total number of pages (0 if unknown)
func (m Model) totalPages() int {
	if m.pageSize <= 0 || m.totalRows <= 0 {
		return 0
	}
	pages := m.totalRows / m.pageSize
	if m.totalRows%m.pageSize > 0 {
		pages++
	}
	return pages
}
