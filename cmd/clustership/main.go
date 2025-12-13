package main

import (
	"clustership/pkg/tui"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// create the TUI app
	app := tui.NewAppModel()

	// run with alt screen so we don't mess up the terminal
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running ClusterShip: %v\n", err)
		os.Exit(1)
	}
}
