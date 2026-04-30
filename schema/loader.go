package schema

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Column represents a database column with its metadata
type Column struct {
	Name       string
	Type       string
	NotNull    bool
	PrimaryKey bool
}

// Table represents a database table with its columns
type Table struct {
	Name    string
	Columns []Column
}

// Schema holds all tables discovered from the database
type Schema struct {
	Tables map[string]Table // key is table name
}

// LoadSchema opens a SQLite database and loads all table and column metadata
func LoadSchema(dbPath string) (*Schema, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	schema := &Schema{
		Tables: make(map[string]Table),
	}

	// Get all user tables (exclude sqlite internal tables)
	tables, err := getTables(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	// For each table, get column information
	for _, tableName := range tables {
		columns, err := getColumns(db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}

		schema.Tables[tableName] = Table{
			Name:    tableName,
			Columns: columns,
		}
	}

	return schema, nil
}

// getTables returns all user table names from the database
func getTables(db *sql.DB) ([]string, error) {
	query := `
		SELECT name FROM sqlite_master 
		WHERE type='table' 
		AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// getColumns returns all column metadata for a given table
func getColumns(db *sql.DB, tableName string) ([]Column, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString

		// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		columns = append(columns, Column{
			Name:       name,
			Type:       colType,
			NotNull:    notNull == 1,
			PrimaryKey: pk == 1,
		})
	}

	return columns, rows.Err()
}

// GetTableNames returns a sorted list of all table names
func (s *Schema) GetTableNames() []string {
	names := make([]string, 0, len(s.Tables))
	for name := range s.Tables {
		names = append(names, name)
	}
	return names
}

// GetTable returns a table by name, or nil if not found
func (s *Schema) GetTable(name string) *Table {
	if table, ok := s.Tables[name]; ok {
		return &table
	}
	return nil
}
