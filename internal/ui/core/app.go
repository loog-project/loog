package core

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/theme"
)

type App struct {
	width, height int
	theme         theme.Theme
	router        *Router

	shuttingDown bool
}

func NewApp(initialView View, theme theme.Theme) *App {
	router := NewRouter(initialView)
	return &App{
		width:  0,
		height: 0,
		theme:  theme,
		router: router,
	}
}

type tickMsg struct {
	RequestedAt time.Time
}

// tick s every second
func tick() tea.Cmd {
	msg := tickMsg{RequestedAt: time.Now()}
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return msg
	})
}

func (app *App) Init() tea.Cmd {
	return tea.Batch(app.router.Current().Init(), tick())
}

func (app *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// globally shut down if the user presses ctrl+c
		// we don't want to let each view handle this keybind.
		case "ctrl+c", "ctrl+d", "ctrl+q":
			app.shuttingDown = true
			return app, tea.Quit
		}
	case tea.WindowSizeMsg:
		app.width = msg.Width
		app.height = msg.Height
		app.router.PushWindowMeta(app.theme, msg.Width, msg.Height)

	case tickMsg:
		commands = append(commands, tick())
	}

	newView, cmd := app.router.Current().Update(msg)
	app.router.Replace(newView)
	if cmd != nil {
		commands = append(commands, cmd)
	}

	return app, tea.Batch(commands...)
}

func (app *App) View() string {
	if app.shuttingDown {
		return "bye!"
	}
	return app.router.Current().View()
}
