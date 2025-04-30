package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	logStyleInfo = lipgloss.NewStyle().
			Foreground(ColorBrightBlue).
			Padding(0, 1)
	logStyleWarning = lipgloss.NewStyle().
			Background(ColorOrange).
			Foreground(ColorBlack).
			Padding(0, 1)
	logStyleError = lipgloss.NewStyle().
			Background(ColorRed).
			Foreground(ColorWhite).
			Bold(true).
			Padding(0, 1)
)

type LogView struct {
	Base

	viewport viewport.Model
	logger   *UILogger

	autoscroll bool
}

func NewLogView(logger *UILogger) *LogView {
	l := &LogView{
		logger:     logger,
		viewport:   viewport.New(10, 10),
		autoscroll: true,
	}
	l.renderLogView()
	return l
}

func (lv *LogView) SetSize(width int, height int) {
	lv.viewport.Width = width - 2
	lv.viewport.Height = height - 3
}

func (lv *LogView) Breadcrumb() string {
	return "Log"
}

func (lv *LogView) renderLogView() {
	var lines []string
	for _, msg := range lv.logger.messages {
		lines = append(lines, fmt.Sprintf("%s [%-7s] [%-10s]: %s",
			msg.Time.Format("15:04:05"),
			levelString(msg.Level),
			msg.Source,
			msg.Text))
	}
	lv.viewport.SetContent(strings.Join(lines, "\n"))
}

func (lv *LogView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "q", "esc":
			return lv, PushChangeView(Pop, nil)
		case "s":
			lv.autoscroll = !lv.autoscroll
			if lv.autoscroll {
				lv.viewport.GotoBottom()
			}
		default:
			return lv, ScrollViewport(v, &lv.viewport)
		}
	case LogMsg:
		lv.renderLogView()
		if lv.autoscroll {
			lv.viewport.GotoBottom()
		}
	}
	return lv, nil
}

func (lv *LogView) View() string {
	return fmt.Sprintf("Log (%d) [autoscroll %s]\n\n%s",
		len(lv.logger.messages),
		ternary(lv.autoscroll, "on", "off"),
		lv.Theme.BorderIdleContainerStyle.Render(lv.viewport.View()))
}

func (lv *LogView) KeyMap() string {
	return NewShortcuts(
		"q/esc", "go back",
		"s", "toggle autoscroll",
	).Render(lv.Theme)
}

func levelString(level LogLevel) string {
	switch level {
	case LogLevelInfo:
		return logStyleInfo.Render("INFO")
	case LogLevelWarning:
		return logStyleWarning.Render("WARN")
	case LogLevelError:
		return logStyleError.Render("ERROR")
	default:
		return "unknown"
	}
}
