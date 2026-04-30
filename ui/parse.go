package ui

import (
	"fmt"
	"sqlive/query"
	"sqlive/suggestion"
	"strings"
)

// tokenizeRaw splits input respecting single/double quotes (preserves original case)
func tokenizeRaw(input string) []string {
	var tokens []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			cur.WriteByte(ch)
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			cur.WriteByte(ch)
		case ch == ' ' && !inSingle && !inDouble:
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// tokenizeUpper returns the same tokens as tokenizeRaw but uppercased
func tokenizeUpper(input string) []string {
	raw := tokenizeRaw(input)
	for i, t := range raw {
		raw[i] = strings.ToUpper(t)
	}
	return raw
}

// unquote strips surrounding single or double quotes and un-escapes inner single quotes
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			inner := s[1 : len(s)-1]
			return strings.ReplaceAll(inner, "''", "'")
		}
	}
	return s
}

// updateQueryModel updates the internal query model based on accepted suggestion
func (m *Model) updateQueryModel(s suggestion.Suggestion) {
	switch s.Type {
	case suggestion.TypeKeyword:
		switch s.Text {
		case "WHERE":
			m.buildContext = BuildWhere
			m.whereColumn = ""
			m.whereOperator = ""
		case "ORDER BY", "ORDER":
			m.buildContext = BuildOrder
		case "GROUP BY", "GROUP":
			m.buildContext = BuildGroup
		case "DISTINCT":
			m.queryModel.SetDistinct(true)
		case "ASC":
			m.queryModel.SetLastOrderDirection(query.OrderAsc)
		case "DESC":
			m.queryModel.SetLastOrderDirection(query.OrderDesc)
		case "IS NULL":
			if m.buildContext == BuildWhere && m.whereColumn != "" {
				m.queryModel.AddCondition(m.whereColumn, query.OpIsNull, "")
				m.whereColumn = ""
				m.whereOperator = ""
			}
		case "IS NOT NULL":
			if m.buildContext == BuildWhere && m.whereColumn != "" {
				m.queryModel.AddCondition(m.whereColumn, query.OpIsNotNull, "")
				m.whereColumn = ""
				m.whereOperator = ""
			}
		}

	case suggestion.TypeTable:
		m.queryModel.SetTable(s.Text)
		m.queryModel.ClearColumns()

	case suggestion.TypeColumn:
		if m.buildContext == BuildFunctionArg && m.pendingFunction != "" {
			// Complete the function call: e.g. SUM(amount)
			m.queryModel.AddColumn(m.pendingFunction + "(" + s.Text + ")")
			m.buildContext = BuildSelect
			m.pendingFunction = ""
		} else if m.buildContext == BuildWhere {
			m.whereColumn = s.Text
		} else if m.buildContext == BuildOrder {
			m.queryModel.AddOrderBy(s.Text, query.OrderAsc)
		} else if m.buildContext == BuildGroup {
			m.queryModel.AddGroupBy(s.Text)
		} else {
			if s.Text == "*" {
				m.queryModel.ClearColumns()
			} else {
				m.queryModel.AddColumn(s.Text)
			}
		}

	case suggestion.TypeOperator:
		if m.buildContext == BuildWhere && m.whereColumn != "" {
			m.whereOperator = query.Operator(s.Text)
		}

	case suggestion.TypeValue:
		if m.buildContext == BuildWhere && m.whereColumn != "" && m.whereOperator != "" {
			if s.Text != "''" && s.Text != "" {
				m.queryModel.AddCondition(m.whereColumn, m.whereOperator, s.Text)
			}
		}

	case suggestion.TypeFunction:
		if strings.Contains(s.Text, "(") {
			// Complete expression like COUNT(*) — add directly as a column
			m.queryModel.AddColumn(s.Text)
		} else {
			// Bare name like SUM, AVG — wait for the user to pick its argument column
			m.buildContext = BuildFunctionArg
			m.pendingFunction = s.Text
		}
	}
}

// parseInput parses the text input to completely rebuild the query model
func (m *Model) parseInput() {
	input := m.textInput.Value()
	tokens := tokenizeUpper(input)
	originalTokens := tokenizeRaw(input)

	if len(tokens) == 0 {
		m.queryModel.Reset()
		m.buildContext = BuildSelect
		return
	}

	// Reset and rebuild from scratch
	m.queryModel.Reset()
	m.buildContext = BuildSelect

	// Locate SELECT keyword
	selectIdx := -1
	for i, token := range tokens {
		if token == "SELECT" {
			selectIdx = i
			break
		}
	}
	if selectIdx == -1 {
		return
	}

	// Table-first flow: SELECT [DISTINCT] <table> <columns…> [JOIN…] [WHERE…] [GROUP BY…] [HAVING…] [ORDER BY…] [LIMIT…] [OFFSET…]
	tableIdx := selectIdx + 1
	if tableIdx < len(tokens) && tokens[tableIdx] == "DISTINCT" {
		m.queryModel.SetDistinct(true)
		tableIdx = selectIdx + 2
	}

	if tableIdx < len(tokens) && !isKeywordToken(tokens[tableIdx]) {
		m.queryModel.SetTable(originalTokens[tableIdx])

		// Parse SELECT column list (everything after table until a clause keyword)
		for i := tableIdx + 1; i < len(tokens); i++ {
			t := tokens[i]
			if t == "WHERE" || t == "ORDER" || t == "LIMIT" || t == "GROUP" || t == "HAVING" ||
				t == "INNER" || t == "LEFT" || t == "RIGHT" || t == "FULL" || t == "CROSS" || t == "JOIN" || t == "OFFSET" {
				break
			}
			if originalTokens[i] == "*" {
				m.queryModel.ClearColumns()
			} else if !isKeywordToken(t) {
				m.queryModel.AddColumn(originalTokens[i])
			}
		}
	}

	// Parse WHERE clause
	whereIdx := indexOfToken(tokens, "WHERE")
	if whereIdx >= 0 {
		m.buildContext = BuildWhere
		i := whereIdx + 1
		for i < len(tokens) {
			// Reached another clause keyword
			if isClauseKeyword(tokens[i]) {
				break
			}
			// Skip AND / OR connectors (but record them for condition logic)
			if tokens[i] == "AND" || tokens[i] == "OR" {
				i++
				continue
			}

			// Try to parse a condition starting at i
			cond, consumed := parseConditionAt(tokens, originalTokens, i)
			if consumed == 0 {
				// not parseable yet — record partial for WHERE context tracking
				if !isKeywordToken(tokens[i]) {
					m.whereColumn = originalTokens[i]
				}
				if i+1 < len(tokens) && isOperatorToken(tokens[i+1]) {
					m.whereOperator = query.Operator(originalTokens[i+1])
				}
				break
			}
			// Prepend logic op from the token before this condition
			if i > whereIdx+1 {
				// Find logic op immediately before index i
				for k := i - 1; k > whereIdx; k-- {
					if tokens[k] == "OR" {
						cond.LogicOp = "OR"
						break
					}
					if tokens[k] == "AND" {
						break
					}
				}
			}
			m.queryModel.AddConditionFull(cond)
			i += consumed
		}
	}

	// Parse GROUP BY
	groupIdx := indexOfToken(tokens, "GROUP")
	if groupIdx >= 0 && groupIdx+1 < len(tokens) && tokens[groupIdx+1] == "BY" {
		for i := groupIdx + 2; i < len(tokens); i++ {
			t := tokens[i]
			if isClauseKeyword(t) {
				break
			}
			// Support comma-separated: "col1,col2"
			for _, c := range strings.Split(originalTokens[i], ",") {
				c = strings.TrimSpace(c)
				if c != "" && !isKeywordToken(strings.ToUpper(c)) {
					m.queryModel.AddGroupBy(c)
				}
			}
		}
		m.buildContext = BuildGroup
	}

	// Parse HAVING
	havingIdx := indexOfToken(tokens, "HAVING")
	if havingIdx >= 0 {
		end := len(originalTokens)
		for _, kw := range []string{"ORDER", "LIMIT", "OFFSET"} {
			idx := indexOfToken(tokens, kw)
			if idx > havingIdx && idx < end {
				end = idx
			}
		}
		if havingIdx+1 < len(originalTokens) {
			m.queryModel.SetHaving(strings.Join(originalTokens[havingIdx+1:end], " "))
		}
	}

	// Parse ORDER BY
	orderIdx := indexOfToken(tokens, "ORDER")
	if orderIdx >= 0 && orderIdx+1 < len(tokens) && tokens[orderIdx+1] == "BY" {
		if orderIdx+2 < len(tokens) {
			colName := originalTokens[orderIdx+2]
			direction := query.OrderAsc
			if orderIdx+3 < len(tokens) && tokens[orderIdx+3] == "DESC" {
				direction = query.OrderDesc
			}
			m.queryModel.AddOrderBy(colName, direction)
		}
		m.buildContext = BuildOrder
	}

	// Parse LIMIT
	limitIdx := indexOfToken(tokens, "LIMIT")
	if limitIdx >= 0 && limitIdx+1 < len(tokens) {
		var v int
		fmt.Sscanf(tokens[limitIdx+1], "%d", &v)
		if v > 0 {
			m.queryModel.SetLimit(v)
		}
	}

	// Parse OFFSET
	offsetIdx := indexOfToken(tokens, "OFFSET")
	if offsetIdx >= 0 && offsetIdx+1 < len(tokens) {
		var v int
		fmt.Sscanf(tokens[offsetIdx+1], "%d", &v)
		if v >= 0 {
			m.queryModel.SetOffset(v)
		}
	}
}

// parseConditionAt attempts to parse a WHERE condition starting at index i.
// Returns the Condition and the number of tokens consumed (0 if unable to parse).
func parseConditionAt(upper, original []string, i int) (query.Condition, int) {
	if i >= len(upper) {
		return query.Condition{}, 0
	}
	col := original[i]
	upperCol := upper[i]
	if isKeywordToken(upperCol) {
		return query.Condition{}, 0
	}

	if i+1 >= len(upper) {
		return query.Condition{}, 0
	}

	// IS NULL / IS NOT NULL
	if upper[i+1] == "IS" {
		if i+2 < len(upper) && upper[i+2] == "NULL" {
			return query.Condition{Column: col, Operator: query.OpIsNull, LogicOp: "AND"}, 3
		}
		if i+3 < len(upper) && upper[i+2] == "NOT" && upper[i+3] == "NULL" {
			return query.Condition{Column: col, Operator: query.OpIsNotNull, LogicOp: "AND"}, 4
		}
		return query.Condition{}, 0
	}

	// BETWEEN col AND val2
	if upper[i+1] == "BETWEEN" {
		if i+4 < len(upper) && upper[i+3] == "AND" {
			v1 := unquote(original[i+2])
			v2 := unquote(original[i+4])
			return query.Condition{Column: col, Operator: query.OpBetween, Value: v1, Value2: v2, LogicOp: "AND"}, 5
		}
		return query.Condition{}, 0
	}

	// NOT LIKE / NOT IN
	if upper[i+1] == "NOT" && i+2 < len(upper) {
		switch upper[i+2] {
		case "LIKE":
			if i+3 < len(upper) {
				return query.Condition{Column: col, Operator: query.OpNotLike, Value: unquote(original[i+3]), LogicOp: "AND"}, 4
			}
		case "IN":
			// IN (v1, v2, ...) - collect until closing paren
			vals, consumed := parseInList(original, i+3)
			return query.Condition{Column: col, Operator: query.OpNotIn, Values: vals, LogicOp: "AND"}, consumed + 3
		}
		return query.Condition{}, 0
	}

	// IN (v1, v2, ...)
	if upper[i+1] == "IN" {
		vals, consumed := parseInList(original, i+2)
		return query.Condition{Column: col, Operator: query.OpIn, Values: vals, LogicOp: "AND"}, consumed + 2
	}

	// Standard single-value operator
	if isOperatorToken(upper[i+1]) {
		if i+2 >= len(upper) {
			return query.Condition{}, 0
		}
		if isClauseKeyword(upper[i+2]) {
			return query.Condition{}, 0
		}
		return query.Condition{
			Column:   col,
			Operator: query.Operator(original[i+1]),
			Value:    unquote(original[i+2]),
			LogicOp:  "AND",
		}, 3
	}

	return query.Condition{}, 0
}

// parseInList collects values from IN (...), returning the values and number of tokens consumed
func parseInList(tokens []string, start int) ([]string, int) {
	if start >= len(tokens) {
		return nil, 0
	}
	consumed := 0
	var vals []string

	// Handle case where opening paren is attached to first token or is separate token "("
	for i := start; i < len(tokens); i++ {
		t := tokens[i]
		t = strings.Trim(t, "()")
		if t != "" {
			for _, v := range strings.Split(t, ",") {
				v = strings.TrimSpace(v)
				v = strings.Trim(v, "()")
				v = unquote(v)
				if v != "" {
					vals = append(vals, v)
				}
			}
		}
		consumed++
		if strings.Contains(tokens[i], ")") {
			break
		}
	}
	return vals, consumed
}

// indexOfToken returns the first index of target in tokens, or -1
func indexOfToken(tokens []string, target string) int {
	for i, t := range tokens {
		if t == target {
			return i
		}
	}
	return -1
}

// isClauseKeyword returns true for tokens that start major SQL clauses
func isClauseKeyword(token string) bool {
	switch token {
	case "WHERE", "ORDER", "LIMIT", "GROUP", "HAVING", "OFFSET":
		return true
	}
	return false
}

// isKeywordToken checks if a token is a SQL keyword
func isKeywordToken(token string) bool {
	keywords := []string{
		"SELECT", "FROM", "WHERE", "ORDER", "BY", "LIMIT", "AND", "OR",
		"ASC", "DESC", "GROUP", "HAVING", "DISTINCT", "OFFSET",
	}
	for _, kw := range keywords {
		if token == kw {
			return true
		}
	}
	return false
}

// isOperatorToken checks if a token is a SQL comparison operator
func isOperatorToken(token string) bool {
	operators := []string{"=", "!=", "<", "<=", ">", ">=", "LIKE"}
	for _, op := range operators {
		if token == op {
			return true
		}
	}
	return false
}
