package suggestion

import (
	"database/sql"
	"fmt"
	"sort"
	"sqlive/query"
	"sqlive/schema"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Suggestion represents a single suggestion item
type Suggestion struct {
	Text        string
	Description string
	Type        SuggestionType
	Score       int // relevance score for ranking
}

// SuggestionType categorizes suggestions
type SuggestionType string

const (
	TypeKeyword  SuggestionType = "keyword"
	TypeTable    SuggestionType = "table"
	TypeColumn   SuggestionType = "column"
	TypeOperator SuggestionType = "operator"
	TypeValue    SuggestionType = "value"
	TypeFunction SuggestionType = "function"
)

// Context represents the current input context
type Context string

const (
	CtxBeginning     Context = "beginning"
	CtxAfterSelect   Context = "after_select"
	CtxAfterTable    Context = "after_table"
	CtxAfterFrom     Context = "after_from"
	CtxAfterWhere    Context = "after_where"
	CtxAfterColumn   Context = "after_column"
	CtxAfterOperator Context = "after_operator"
	CtxAfterOrderBy  Context = "after_orderby"
	CtxAfterGroupBy  Context = "after_groupby"
	CtxAfterOrderDir Context = "after_orderdir"
	CtxAfterLimit    Context = "after_limit"
	CtxAfterHaving   Context = "after_having"
	CtxAfterFunction Context = "after_function" // waiting for column argument to an agg function
)

// knownAggregateFunctions lists bare function names that require a column argument
var knownAggregateFunctions = map[string]bool{
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	"LENGTH": true, "UPPER": true, "LOWER": true, "COALESCE": true,
}

// Engine provides context-aware suggestions
type Engine struct {
	schema *schema.Schema
	model  *query.Model
	dbPath string

	// usage counters for recency ranking
	usageCounts map[string]int
}

// NewEngine creates a new suggestion engine
func NewEngine(s *schema.Schema, m *query.Model) *Engine {
	return &Engine{
		schema:      s,
		model:       m,
		usageCounts: make(map[string]int),
	}
}

// SetDBPath provides the engine with the database path for value autocomplete
func (e *Engine) SetDBPath(path string) {
	e.dbPath = path
}

// RecordUsage increments the usage counter for a text item (for ranking)
func (e *Engine) RecordUsage(text string) {
	e.usageCounts[strings.ToLower(text)]++
}

// GetSuggestions returns ranked suggestions based on current input and cursor position
func (e *Engine) GetSuggestions(input string, cursorPos int) []Suggestion {
	tokens := tokenize(input)
	ctx := e.detectContext(tokens, cursorPos, input)
	partial := e.getCurrentToken(input, cursorPos)
	// When the partial token is "FUNC(partial" (open bracket, no close),
	// trim the function-name prefix so column matching works normally.
	if parenIdx := strings.Index(partial, "("); parenIdx >= 0 && !strings.Contains(partial, ")") {
		partial = partial[parenIdx+1:]
	}
	suggestions := e.generateSuggestions(ctx, partial, tokens)
	return filterAndRank(suggestions, partial, e.usageCounts)
}

// tokenize splits input into tokens, respecting single-quoted strings and
// double-quoted identifiers so that values with spaces are kept whole.
func tokenize(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return []string{}
	}

	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			current.WriteByte(ch)
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			current.WriteByte(ch)
		case ch == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToUpper(current.String()))
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, strings.ToUpper(current.String()))
	}
	return tokens
}

// detectContext determines what kind of suggestions to show
func (e *Engine) detectContext(tokens []string, cursorPos int, input string) Context {
	if len(tokens) == 0 {
		return CtxBeginning
	}

	// Walk backwards to find the most recent structural keyword
	lastKeyword := ""
	lastKeywordIdx := -1
	for i := len(tokens) - 1; i >= 0; i-- {
		switch tokens[i] {
		case "SELECT", "FROM", "WHERE", "ORDER", "BY", "LIMIT", "GROUP", "HAVING":
			lastKeyword = tokens[i]
			lastKeywordIdx = i
			goto done
		}
	}
done:

	// HAVING context
	if lastKeyword == "HAVING" {
		return CtxAfterHaving
	}

	// WHERE context — detect column / operator / value progression
	if lastKeyword == "WHERE" {
		tokensAfterWhere := tokens[lastKeywordIdx+1:]
		if len(tokensAfterWhere) == 0 {
			return CtxAfterWhere
		}
		last := tokensAfterWhere[len(tokensAfterWhere)-1]
		if isMultiWordOperatorSuffix(last, tokensAfterWhere) {
			return CtxAfterOperator
		}
		if isOperator(last) {
			return CtxAfterOperator
		}
		// After AND / OR — start new condition, suggest columns
		if last == "AND" || last == "OR" {
			return CtxAfterWhere
		}
		if !isKeyword(last) && !isOperator(last) {
			// Could be column (suggest operator) or value (already past operator)
			return CtxAfterColumn
		}
		return CtxAfterWhere
	}

	switch lastKeyword {
	case "":
		return CtxBeginning
	case "SELECT":
		if e.model.Table != "" {
			// If the last complete token is a bare aggregate function name,
			// the user needs to supply its column argument.
			if len(tokens) > 0 {
				last := tokens[len(tokens)-1]
				// Match "SUM" (bare) or "SUM(" / "SUM(partial" (open paren, no close)
				if parenIdx := strings.Index(last, "("); parenIdx > 0 {
					funcName := last[:parenIdx]
					if knownAggregateFunctions[funcName] && !strings.Contains(last, ")") {
						return CtxAfterFunction
					}
				} else if knownAggregateFunctions[last] && !strings.Contains(last, "(") {
					return CtxAfterFunction
				}
			}
			return CtxAfterTable
		}
		return CtxAfterSelect
	case "FROM":
		return CtxAfterFrom
	case "BY":
		// Determine if ORDER BY or GROUP BY
		if lastKeywordIdx > 0 {
			prev := tokens[lastKeywordIdx-1]
			if prev == "GROUP" {
				return CtxAfterGroupBy
			}
			if prev == "ORDER" {
				// Check if there's a column after BY — if so, suggest ASC/DESC
				tokensAfterBy := tokens[lastKeywordIdx+1:]
				if e.model.Table != "" && len(tokensAfterBy) > 0 {
					table := e.schema.GetTable(e.model.Table)
					if table != nil {
						lastToken := tokensAfterBy[len(tokensAfterBy)-1]
						for _, col := range table.Columns {
							if strings.ToUpper(col.Name) == lastToken {
								return CtxAfterOrderDir
							}
						}
					}
				}
				return CtxAfterOrderBy
			}
		}
		return CtxAfterOrderBy
	case "GROUP":
		return CtxAfterGroupBy
	case "LIMIT":
		return CtxAfterLimit
	default:
		if e.model.Table != "" {
			return CtxAfterTable
		}
		return CtxAfterSelect
	}
}

// getCurrentToken gets the partial token at the cursor position
func (e *Engine) getCurrentToken(input string, cursorPos int) string {
	if cursorPos > len(input) {
		cursorPos = len(input)
	}
	start := cursorPos
	for start > 0 && input[start-1] != ' ' {
		start--
	}
	end := cursorPos
	for end < len(input) && input[end] != ' ' {
		end++
	}
	return strings.TrimSpace(input[start:end])
}

// generateSuggestions creates suggestions based on context
func (e *Engine) generateSuggestions(ctx Context, partial string, tokens []string) []Suggestion {
	switch ctx {
	case CtxBeginning:
		return []Suggestion{
			{Text: "SELECT", Description: "Start a SELECT query", Type: TypeKeyword},
		}

	case CtxAfterSelect:
		var suggestions []Suggestion
		suggestions = append(suggestions, Suggestion{Text: "DISTINCT", Description: "Select distinct values", Type: TypeKeyword})
		for _, name := range e.schema.GetTableNames() {
			table := e.schema.GetTable(name)
			suggestions = append(suggestions, Suggestion{
				Text:        name,
				Description: fmt.Sprintf("%d columns", len(table.Columns)),
				Type:        TypeTable,
			})
		}
		return suggestions

	case CtxAfterTable:
		if e.model.Table == "" {
			return []Suggestion{}
		}

		suggestions := []Suggestion{
			{Text: "*", Description: "All columns", Type: TypeKeyword},
		}

		table := e.schema.GetTable(e.model.Table)
		if table != nil {
			for _, col := range table.Columns {
				suggestions = append(suggestions, Suggestion{
					Text:        col.Name,
					Description: col.Type,
					Type:        TypeColumn,
				})
			}
		}

		// Aggregate functions
		aggFns := []struct{ name, desc string }{
			{"COUNT(*)", "Count all rows"},
			{"COUNT", "Count non-null values"},
			{"SUM", "Sum of values"},
			{"AVG", "Average of values"},
			{"MIN", "Minimum value"},
			{"MAX", "Maximum value"},
		}
		for _, fn := range aggFns {
			suggestions = append(suggestions, Suggestion{Text: fn.name, Description: fn.desc, Type: TypeFunction})
		}

		// Clause keywords
		suggestions = append(suggestions,
			Suggestion{Text: "WHERE", Description: "Filter results", Type: TypeKeyword},
			Suggestion{Text: "GROUP BY", Description: "Group results", Type: TypeKeyword},
			Suggestion{Text: "HAVING", Description: "Filter grouped results", Type: TypeKeyword},
			Suggestion{Text: "ORDER BY", Description: "Order results", Type: TypeKeyword},
			Suggestion{Text: "LIMIT", Description: "Limit rows returned", Type: TypeKeyword},
		)

		return suggestions

	case CtxAfterFrom:
		var suggestions []Suggestion
		for _, name := range e.schema.GetTableNames() {
			table := e.schema.GetTable(name)
			suggestions = append(suggestions, Suggestion{
				Text:        name,
				Description: fmt.Sprintf("%d columns", len(table.Columns)),
				Type:        TypeTable,
			})
		}
		return suggestions

	case CtxAfterWhere:
		if e.model.Table == "" {
			return []Suggestion{}
		}
		table := e.schema.GetTable(e.model.Table)
		if table == nil {
			return []Suggestion{}
		}
		var suggestions []Suggestion
		for _, col := range table.Columns {
			suggestions = append(suggestions, Suggestion{
				Text:        col.Name,
				Description: col.Type,
				Type:        TypeColumn,
			})
		}
		return suggestions

	case CtxAfterColumn:
		return []Suggestion{
			{Text: "=", Description: "Equal to", Type: TypeOperator},
			{Text: "!=", Description: "Not equal to", Type: TypeOperator},
			{Text: "<", Description: "Less than", Type: TypeOperator},
			{Text: "<=", Description: "Less than or equal", Type: TypeOperator},
			{Text: ">", Description: "Greater than", Type: TypeOperator},
			{Text: ">=", Description: "Greater than or equal", Type: TypeOperator},
			{Text: "LIKE", Description: "Pattern match (% wildcard)", Type: TypeOperator},
			{Text: "NOT LIKE", Description: "Inverse pattern match", Type: TypeOperator},
			{Text: "IN", Description: "Value in a set", Type: TypeOperator},
			{Text: "NOT IN", Description: "Value not in a set", Type: TypeOperator},
			{Text: "BETWEEN", Description: "Value in range (inclusive)", Type: TypeOperator},
			{Text: "IS NULL", Description: "Value is NULL", Type: TypeOperator},
			{Text: "IS NOT NULL", Description: "Value is not NULL", Type: TypeOperator},
		}

	case CtxAfterOperator:
		return e.valuesSuggestion(tokens)

	case CtxAfterOrderBy:
		if e.model.Table == "" {
			return []Suggestion{}
		}
		table := e.schema.GetTable(e.model.Table)
		if table == nil {
			return []Suggestion{}
		}
		var suggestions []Suggestion
		for _, col := range table.Columns {
			suggestions = append(suggestions, Suggestion{Text: col.Name, Description: col.Type, Type: TypeColumn})
		}
		return suggestions

	case CtxAfterOrderDir:
		return []Suggestion{
			{Text: "ASC", Description: "Ascending order", Type: TypeKeyword},
			{Text: "DESC", Description: "Descending order", Type: TypeKeyword},
		}

	case CtxAfterGroupBy:
		if e.model.Table == "" {
			return []Suggestion{}
		}
		table := e.schema.GetTable(e.model.Table)
		if table == nil {
			return []Suggestion{}
		}
		var suggestions []Suggestion
		for _, col := range table.Columns {
			suggestions = append(suggestions, Suggestion{Text: col.Name, Description: col.Type, Type: TypeColumn})
		}
		return suggestions

	case CtxAfterLimit:
		return []Suggestion{
			{Text: "10", Description: "10 rows", Type: TypeValue},
			{Text: "25", Description: "25 rows", Type: TypeValue},
			{Text: "50", Description: "50 rows", Type: TypeValue},
			{Text: "100", Description: "100 rows", Type: TypeValue},
			{Text: "500", Description: "500 rows", Type: TypeValue},
		}

	case CtxAfterHaving:
		// Suggest aggregate expressions for HAVING
		return []Suggestion{
			{Text: "COUNT(*) >", Description: "Rows count greater than", Type: TypeFunction},
			{Text: "COUNT(*) >=", Description: "Rows count at least", Type: TypeFunction},
			{Text: "SUM", Description: "Sum condition", Type: TypeFunction},
			{Text: "AVG", Description: "Average condition", Type: TypeFunction},
			{Text: "MIN", Description: "Minimum condition", Type: TypeFunction},
			{Text: "MAX", Description: "Maximum condition", Type: TypeFunction},
		}

	case CtxAfterFunction:
		// The last token is a bare aggregate function name — suggest columns as the argument
		if e.model.Table == "" {
			return []Suggestion{}
		}
		table := e.schema.GetTable(e.model.Table)
		if table == nil {
			return []Suggestion{}
		}
		var suggestions []Suggestion
		suggestions = append(suggestions, Suggestion{Text: "*", Description: "All rows", Type: TypeColumn})
		for _, col := range table.Columns {
			suggestions = append(suggestions, Suggestion{
				Text:        col.Name,
				Description: fmt.Sprintf("%s — function argument", col.Type),
				Type:        TypeColumn,
			})
		}
		return suggestions
	}

	return []Suggestion{}
}

// valuesSuggestion returns value suggestions for the current WHERE column
func (e *Engine) valuesSuggestion(tokens []string) []Suggestion {
	colName, colType := e.findWhereColumn(tokens)

	// Try to fetch distinct values from the DB
	if e.dbPath != "" && colName != "" {
		if vals := e.fetchDistinctValues(colName); len(vals) > 0 {
			var suggestions []Suggestion
			for _, v := range vals {
				suggestions = append(suggestions, Suggestion{
					Text:        v,
					Description: "DB value",
					Type:        TypeValue,
				})
			}
			return suggestions
		}
	}

	// Fall back to type-based suggestions
	colType = strings.ToUpper(colType)
	if strings.Contains(colType, "INT") || strings.Contains(colType, "REAL") ||
		strings.Contains(colType, "NUMERIC") || strings.Contains(colType, "DECIMAL") {
		return []Suggestion{
			{Text: "0", Description: "Zero", Type: TypeValue},
			{Text: "1", Description: "One", Type: TypeValue},
			{Text: "10", Description: "Ten", Type: TypeValue},
			{Text: "100", Description: "Hundred", Type: TypeValue},
		}
	}
	if strings.Contains(colType, "DATE") || strings.Contains(colType, "TIME") {
		return []Suggestion{
			{Text: "2024-01-01", Description: "Date (YYYY-MM-DD)", Type: TypeValue},
			{Text: "2024-01-01 00:00:00", Description: "Datetime", Type: TypeValue},
		}
	}
	if strings.Contains(colType, "BOOL") {
		return []Suggestion{
			{Text: "1", Description: "True", Type: TypeValue},
			{Text: "0", Description: "False", Type: TypeValue},
		}
	}
	return []Suggestion{
		{Text: "''", Description: "Empty string", Type: TypeValue},
		{Text: "NULL", Description: "NULL value", Type: TypeValue},
	}
}

// findWhereColumn locates the column name just before the WHERE operator
func (e *Engine) findWhereColumn(tokens []string) (name, colType string) {
	if e.model.Table == "" {
		return "", ""
	}
	// Walk back from end to find operator; the token before is the column
	for i := len(tokens) - 1; i >= 1; i-- {
		if isOperator(tokens[i]) {
			colName := tokens[i-1]
			table := e.schema.GetTable(e.model.Table)
			if table != nil {
				for _, col := range table.Columns {
					if strings.ToUpper(col.Name) == colName {
						return col.Name, col.Type
					}
				}
			}
			return colName, ""
		}
	}
	return "", ""
}

// fetchDistinctValues queries the DB for up to 10 distinct values of colName
func (e *Engine) fetchDistinctValues(colName string) []string {
	if e.model.Table == "" {
		return nil
	}
	db, err := sql.Open("sqlite3", e.dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()

	q := fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s IS NOT NULL LIMIT 10",
		colName, e.model.Table, colName)
	rows, err := db.Query(q)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var vals []string
	for rows.Next() {
		var v interface{}
		if err := rows.Scan(&v); err != nil {
			break
		}
		if v != nil {
			vals = append(vals, fmt.Sprintf("%v", v))
		}
	}
	return vals
}

// filterAndRank filters suggestions by fuzzy match and sorts by score
func filterAndRank(suggestions []Suggestion, partial string, usageCounts map[string]int) []Suggestion {
	if partial == "" {
		// No filtering, but still rank by usage
		for i := range suggestions {
			suggestions[i].Score = usageCounts[strings.ToLower(suggestions[i].Text)] * 10
		}
		sort.SliceStable(suggestions, func(i, j int) bool {
			return suggestions[i].Score > suggestions[j].Score
		})
		return suggestions
	}

	lower := strings.ToLower(partial)
	var filtered []Suggestion

	for _, s := range suggestions {
		score := fuzzyScore(s.Text, lower)
		if score > 0 {
			score += usageCounts[strings.ToLower(s.Text)] * 10
			s.Score = score
			filtered = append(filtered, s)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})
	return filtered
}

// fuzzyScore returns a relevance score (0 = no match)
func fuzzyScore(text, partial string) int {
	t := strings.ToLower(text)
	p := strings.ToLower(partial)

	if t == p {
		return 100
	}
	if strings.HasPrefix(t, p) {
		return 80
	}
	if strings.Contains(t, p) {
		return 60
	}

	// Character-sequence fuzzy match
	pi, ti := 0, 0
	score := 0
	consecutive := 0
	for pi < len(p) && ti < len(t) {
		if p[pi] == t[ti] {
			score += 10 + consecutive*5
			consecutive++
			pi++
		} else {
			consecutive = 0
		}
		ti++
	}
	if pi < len(p) {
		return 0 // not all characters matched
	}
	return score
}

// isMultiWordOperatorSuffix detects the last word of multi-word operators (IS, NOT, NULL)
func isMultiWordOperatorSuffix(token string, tokens []string) bool {
	// "IS NULL" or "IS NOT NULL" — if last token is NULL and previous is IS or NOT
	if token == "NULL" && len(tokens) >= 2 {
		prev := tokens[len(tokens)-2]
		return prev == "IS" || prev == "NOT"
	}
	return false
}

// isOperator checks if a token is a SQL comparison operator
func isOperator(token string) bool {
	operators := []string{"=", "!=", "<", "<=", ">", ">=", "LIKE", "NOT", "IN", "BETWEEN"}
	for _, op := range operators {
		if token == op {
			return true
		}
	}
	return false
}

// isKeyword checks if a token is a SQL keyword
func isKeyword(token string) bool {
	keywords := []string{
		"SELECT", "FROM", "WHERE", "ORDER", "BY", "LIMIT", "AND", "OR",
		"GROUP", "HAVING", "DISTINCT", "ASC", "DESC",
		"INNER", "LEFT", "RIGHT", "FULL", "CROSS", "JOIN", "ON",
	}
	for _, kw := range keywords {
		if token == kw {
			return true
		}
	}
	return false
}
