package query

import (
	"fmt"
	"strings"
)

// Operator represents a comparison operator in WHERE clause
type Operator string

const (
	OpEqual        Operator = "="
	OpNotEqual     Operator = "!="
	OpLessThan     Operator = "<"
	OpLessEqual    Operator = "<="
	OpGreaterThan  Operator = ">"
	OpGreaterEqual Operator = ">="
	OpLike         Operator = "LIKE"
	OpNotLike      Operator = "NOT LIKE"
	OpIn           Operator = "IN"
	OpNotIn        Operator = "NOT IN"
	OpBetween      Operator = "BETWEEN"
	OpIsNull       Operator = "IS NULL"
	OpIsNotNull    Operator = "IS NOT NULL"
)

// Condition represents a single WHERE clause condition
type Condition struct {
	Column   string
	Operator Operator
	Value    string   // single-value operators
	Value2   string   // BETWEEN upper bound
	Values   []string // IN / NOT IN list
	LogicOp  string   // "AND" or "OR" connector to the next condition
}

// OrderDirection represents ASC or DESC
type OrderDirection string

const (
	OrderAsc  OrderDirection = "ASC"
	OrderDesc OrderDirection = "DESC"
)

// OrderBy represents an ORDER BY clause
type OrderBy struct {
	Column    string
	Direction OrderDirection
}

// Model represents the structured query state
type Model struct {
	Table      string
	Columns    []string
	Conditions []Condition
	GroupBy    []string
	Having     string
	OrderBy    []OrderBy
	Limit      int
	Offset     int
	Distinct   bool
}

// NewModel creates a new empty query model
func NewModel() *Model {
	return &Model{
		Columns:    []string{},
		Conditions: []Condition{},
		GroupBy:    []string{},
		OrderBy:    []OrderBy{},
		Distinct:   false,
	}
}

// Reset clears all query state
func (m *Model) Reset() {
	m.Table = ""
	m.Columns = []string{}
	m.Conditions = []Condition{}
	m.GroupBy = []string{}
	m.Having = ""
	m.OrderBy = []OrderBy{}
	m.Limit = 0
	m.Offset = 0
	m.Distinct = false
}

// SetTable sets the table name
func (m *Model) SetTable(table string) {
	m.Table = table
}

// AddColumn adds a column to the SELECT list
func (m *Model) AddColumn(column string) {
	for _, col := range m.Columns {
		if col == column {
			return
		}
	}
	m.Columns = append(m.Columns, column)
}

// ClearColumns removes all columns
func (m *Model) ClearColumns() {
	m.Columns = []string{}
}

// AddCondition adds a WHERE condition with AND logic
func (m *Model) AddCondition(column string, operator Operator, value string) {
	m.Conditions = append(m.Conditions, Condition{
		Column:   column,
		Operator: operator,
		Value:    value,
		LogicOp:  "AND",
	})
}

// AddConditionFull adds a fully specified condition
func (m *Model) AddConditionFull(cond Condition) {
	if cond.LogicOp == "" {
		cond.LogicOp = "AND"
	}
	m.Conditions = append(m.Conditions, cond)
}

// AddOrderBy adds an ORDER BY clause
func (m *Model) AddOrderBy(column string, direction OrderDirection) {
	m.OrderBy = append(m.OrderBy, OrderBy{
		Column:    column,
		Direction: direction,
	})
}

// AddGroupBy adds a GROUP BY column
func (m *Model) AddGroupBy(column string) {
	for _, c := range m.GroupBy {
		if c == column {
			return
		}
	}
	m.GroupBy = append(m.GroupBy, column)
}

// SetDistinct sets whether SELECT should include DISTINCT
func (m *Model) SetDistinct(d bool) {
	m.Distinct = d
}

// SetLastOrderDirection updates the direction of the last ORDER BY entry
func (m *Model) SetLastOrderDirection(direction OrderDirection) {
	if len(m.OrderBy) == 0 {
		return
	}
	m.OrderBy[len(m.OrderBy)-1].Direction = direction
}

// SetLimit sets the LIMIT value
func (m *Model) SetLimit(limit int) {
	m.Limit = limit
}

// SetOffset sets the OFFSET value for pagination
func (m *Model) SetOffset(offset int) {
	m.Offset = offset
}

// SetHaving sets the HAVING clause expression
func (m *Model) SetHaving(h string) {
	m.Having = h
}

// IsValid returns true if the model represents a valid query
func (m *Model) IsValid() bool {
	return m.Table != ""
}

// ToSQL generates a SQL SELECT statement from the model
func (m *Model) ToSQL() string {
	if !m.IsValid() {
		return ""
	}

	var parts []string

	selectPrefix := "SELECT"
	if m.Distinct {
		selectPrefix = "SELECT DISTINCT"
	}

	if len(m.Columns) == 0 {
		parts = append(parts, selectPrefix+" *")
	} else {
		parts = append(parts, selectPrefix+" "+strings.Join(m.Columns, ", "))
	}

	parts = append(parts, "FROM "+m.Table)

	if len(m.Conditions) > 0 {
		var clauses []string
		for i, cond := range m.Conditions {
			expr := conditionExpr(cond)
			if i == 0 {
				clauses = append(clauses, expr)
			} else {
				logicOp := m.Conditions[i-1].LogicOp
				if logicOp == "" {
					logicOp = "AND"
				}
				clauses = append(clauses, logicOp+" "+expr)
			}
		}
		parts = append(parts, "WHERE "+strings.Join(clauses, " "))
	}

	if len(m.GroupBy) > 0 {
		parts = append(parts, "GROUP BY "+strings.Join(m.GroupBy, ", "))
	}

	if m.Having != "" {
		parts = append(parts, "HAVING "+m.Having)
	}

	if len(m.OrderBy) > 0 {
		var orders []string
		for _, order := range m.OrderBy {
			orders = append(orders, fmt.Sprintf("%s %s", order.Column, order.Direction))
		}
		parts = append(parts, "ORDER BY "+strings.Join(orders, ", "))
	}

	if m.Limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", m.Limit))
	}

	if m.Offset > 0 {
		parts = append(parts, fmt.Sprintf("OFFSET %d", m.Offset))
	}

	return strings.Join(parts, " ")
}

// conditionExpr renders a single condition as a SQL fragment
func conditionExpr(cond Condition) string {
	switch cond.Operator {
	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", cond.Column)
	case OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", cond.Column)
	case OpBetween:
		return fmt.Sprintf("%s BETWEEN %s AND %s", cond.Column, quoteValue(cond.Value), quoteValue(cond.Value2))
	case OpIn, OpNotIn:
		vals := make([]string, 0, len(cond.Values))
		for _, v := range cond.Values {
			vals = append(vals, quoteValue(v))
		}
		if len(vals) == 0 && cond.Value != "" {
			vals = append(vals, quoteValue(cond.Value))
		}
		return fmt.Sprintf("%s %s (%s)", cond.Column, cond.Operator, strings.Join(vals, ", "))
	default:
		return fmt.Sprintf("%s %s %s", cond.Column, cond.Operator, quoteValue(cond.Value))
	}
}

// quoteValue wraps a value in single quotes if it isn't a bare number/NULL/TRUE/FALSE
func quoteValue(value string) string {
	if value == "" {
		return "''"
	}
	first := value[0]
	if (first >= '0' && first <= '9') || (first == '-' && len(value) > 1) {
		return value
	}
	upper := strings.ToUpper(value)
	if upper == "NULL" || upper == "TRUE" || upper == "FALSE" {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func needsQuoting(value string) bool {
	if value == "" {
		return true
	}
	first := value[0]
	if (first >= '0' && first <= '9') || (first == '-' && len(value) > 1) {
		return false
	}
	upper := strings.ToUpper(value)
	if upper == "NULL" || upper == "TRUE" || upper == "FALSE" {
		return false
	}
	return true
}
