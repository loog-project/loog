package views

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
)

type StatusBarView struct {
	width, height int
	theme         theme.Theme
}

func NewStatusBarView() *StatusBarView {
	return &StatusBarView{}
}

var _ core.View = (*StatusBarView)(nil)

func (sb *StatusBarView) SetSize(width, height int) {
	sb.width = width
	sb.height = height
}

func (sb *StatusBarView) SetTheme(theme theme.Theme) {
	sb.theme = theme
}

func (sb *StatusBarView) Init() tea.Cmd {
	return nil
}

func (sb *StatusBarView) Update(msg tea.Msg) (core.View, tea.Cmd) {
	return sb, nil
}

func (sb *StatusBarView) View() string {
	return sb.theme.BreadcrumbBarStyle.Render("Hier könnte Ihre Werbung stehen!")
}
