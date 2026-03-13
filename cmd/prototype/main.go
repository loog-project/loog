package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/tui"
)

func main() {
	// Generate dummy Kubernetes resource data
	store := tui.GenerateDummyData()

	fmt.Fprintf(os.Stderr, "loog prototype: %d resources, %d revisions\n",
		store.TotalResourceCount(), store.TotalRevisionCount())

	// Create the app
	app := tui.NewApp(store)

	// Run the program
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
