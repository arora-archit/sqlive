package ui

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	_ "github.com/mattn/go-sqlite3"
)

// executeQuery runs the current query and stores results.
// It also fetches the total row count for pagination display.
func (m *Model) executeQuery() {
	m.queryResults = nil
	m.executionError = ""
	m.resultScrollY = 0
	m.resultScrollX = 0
	m.totalRows = 0

	sqlQuery := m.queryModel.ToSQL()
	if sqlQuery == "" {
		m.executionError = "Query is incomplete"
		return
	}

	db, err := sql.Open("sqlite3", m.dbPath)
	if err != nil {
		m.executionError = fmt.Sprintf("Failed to open database: %v", err)
		return
	}
	defer db.Close()

	// Fetch total row count (stripping LIMIT/OFFSET for the count query)
	if m.queryModel.Table != "" {
		countSQL := buildCountQuery(m.queryModel)
		if countSQL != "" {
			var total int
			if err := db.QueryRow(countSQL).Scan(&total); err == nil {
				m.totalRows = total
			}
		}
	}

	rows, err := db.Query(sqlQuery)
	if err != nil {
		m.executionError = fmt.Sprintf("Query error: %v", err)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		m.executionError = fmt.Sprintf("Failed to get columns: %v", err)
		return
	}

	var results [][]string
	for rows.Next() {
		columnValues := make([]interface{}, len(columns))
		columnPointers := make([]interface{}, len(columns))
		for i := range columnValues {
			columnPointers[i] = &columnValues[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			m.executionError = fmt.Sprintf("Failed to scan row: %v", err)
			return
		}
		row := make([]string, len(columns))
		for i, val := range columnValues {
			if val == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%v", val)
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		m.executionError = fmt.Sprintf("Error iterating rows: %v", err)
		return
	}

	// Record page state
	m.pageOffset = m.queryModel.Offset
	m.pageSize = m.queryModel.Limit

	m.queryResults = &QueryResults{
		Columns: columns,
		Rows:    results,
	}
}

// executeNextPage advances the query by one page
func (m *Model) executeNextPage() {
	if m.pageSize <= 0 {
		return
	}
	newOffset := m.pageOffset + m.pageSize
	if m.totalRows > 0 && newOffset >= m.totalRows {
		return
	}
	m.queryModel.SetOffset(newOffset)
	m.executeQuery()
}

// executePrevPage moves the query back by one page
func (m *Model) executePrevPage() {
	if m.pageSize <= 0 {
		return
	}
	newOffset := m.pageOffset - m.pageSize
	if newOffset < 0 {
		newOffset = 0
	}
	m.queryModel.SetOffset(newOffset)
	m.executeQuery()
}

// buildCountQuery constructs a SELECT COUNT(*) equivalent of the current query
// (ignoring LIMIT, OFFSET, ORDER BY, and column list).
func buildCountQuery(qm interface{ ToSQL() string }) string {
	// We rely on the fact that ToSQL always starts with "SELECT [DISTINCT] ... FROM ..."
	sql := qm.ToSQL()
	if sql == "" {
		return ""
	}
	upper := strings.ToUpper(sql)

	// Find FROM position
	fromIdx := strings.Index(upper, " FROM ")
	if fromIdx < 0 {
		return ""
	}

	fromOnward := sql[fromIdx:] // " FROM table WHERE ..."

	// Strip LIMIT / OFFSET / ORDER BY from the end
	for _, clause := range []string{" ORDER BY ", " LIMIT ", " OFFSET "} {
		if idx := strings.Index(strings.ToUpper(fromOnward), clause); idx >= 0 {
			fromOnward = fromOnward[:idx]
		}
	}

	return "SELECT COUNT(*)" + fromOnward
}

// exportCSV writes query results to a timestamped CSV file in the current directory
func (m *Model) exportCSV() string {
	if m.queryResults == nil {
		return "No results to export"
	}

	filename := fmt.Sprintf("sqlive_export_%s.csv", time.Now().Format("20060102_150405"))
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Sprintf("Failed to create file: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(m.queryResults.Columns); err != nil {
		return fmt.Sprintf("CSV write error: %v", err)
	}
	for _, row := range m.queryResults.Rows {
		if err := w.Write(row); err != nil {
			return fmt.Sprintf("CSV write error: %v", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Sprintf("CSV flush error: %v", err)
	}
	return fmt.Sprintf("Exported %d rows to %s", len(m.queryResults.Rows), filename)
}

// exportJSON writes query results to a timestamped JSON file
func (m *Model) exportJSON() string {
	if m.queryResults == nil {
		return "No results to export"
	}

	filename := fmt.Sprintf("sqlive_export_%s.json", time.Now().Format("20060102_150405"))
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Sprintf("Failed to create file: %v", err)
	}
	defer f.Close()

	// Build slice of maps for JSON encoding
	records := make([]map[string]string, 0, len(m.queryResults.Rows))
	for _, row := range m.queryResults.Rows {
		rec := make(map[string]string, len(m.queryResults.Columns))
		for i, col := range m.queryResults.Columns {
			if i < len(row) {
				rec[col] = row[i]
			}
		}
		records = append(records, rec)
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(records); err != nil {
		return fmt.Sprintf("JSON encode error: %v", err)
	}
	return fmt.Sprintf("Exported %d rows to %s", len(m.queryResults.Rows), filename)
}

// copyCellToClipboard copies the currently focused cell value to the clipboard
func (m *Model) copyCellToClipboard() string {
	if m.queryResults == nil {
		return "No results"
	}
	row := m.resultCursorRow
	col := m.resultCursorCol
	if row < 0 || row >= len(m.queryResults.Rows) {
		return "No cell selected"
	}
	if col < 0 || col >= len(m.queryResults.Columns) {
		return "No cell selected"
	}
	value := m.queryResults.Rows[row][col]
	if err := clipboard.WriteAll(value); err != nil {
		return fmt.Sprintf("Clipboard error: %v", err)
	}
	return fmt.Sprintf("Copied: %s", truncateMsg(value, 30))
}

// copyRowToClipboard copies the currently focused row as tab-separated values
func (m *Model) copyRowToClipboard() string {
	if m.queryResults == nil {
		return "No results"
	}
	row := m.resultCursorRow
	if row < 0 || row >= len(m.queryResults.Rows) {
		return "No row selected"
	}
	value := strings.Join(m.queryResults.Rows[row], "\t")
	if err := clipboard.WriteAll(value); err != nil {
		return fmt.Sprintf("Clipboard error: %v", err)
	}
	return fmt.Sprintf("Copied row %d (%d cells)", row+1, len(m.queryResults.Rows[row]))
}

func truncateMsg(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
