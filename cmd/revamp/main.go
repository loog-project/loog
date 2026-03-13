package main

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/ui"
)

type App struct {
	root         ui.Component
	stateManager *ui.StateManager
	eventBus     ui.EventBus

	tracker *service.TrackerService
	rps     store.ResourcePatchStore

	width, height int
}

func NewApp(tracker *service.TrackerService, rps store.ResourcePatchStore) *App {
	eventBus := ui.NewEventBus()
	stateManager := ui.NewStateManager(eventBus)

	return &App{
		stateManager: stateManager,
		eventBus:     eventBus,
		tracker:      tracker,
		rps:          rps,
	}
}

func (a *App) Init() tea.Cmd {
	
}

func main() {
}
