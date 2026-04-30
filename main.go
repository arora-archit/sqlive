package main

import (
	"fmt"
	"os"
	"sqlive/schema"
	"sqlive/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Check for database path argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: sqlive <path-to-sqlite-database>")
		fmt.Println("\nExample:")
		fmt.Println("  sqlive ./mydata.db")
		os.Exit(1)
	}

	dbPath := os.Args[1]

	// Load schema
	fmt.Printf("Loading schema from %s...\n", dbPath)
	s, err := schema.LoadSchema(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading schema: %v\n", err)
		os.Exit(1)
	}

	// Display loaded tables
	tableNames := s.GetTableNames()
	if len(tableNames) == 0 {
		fmt.Println("Warning: No tables found in database")
	} else {
		fmt.Printf("Loaded %d table(s): %v\n", len(tableNames), tableNames)
	}

	// Create and run UI
	m := ui.NewModel(dbPath, s)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
