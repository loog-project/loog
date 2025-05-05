package layouts

import (
	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
)

// Layout should implement core.Sizeable and core.Themable besides core.View
type Layout interface {
	core.View
	core.Sizeable
	core.Themeable
}

// dispatchTheme dispatches the theme to all views
func dispatchTheme(newTheme theme.Theme, views ...core.View) {
	for _, view := range views {
		if t, ok := view.(core.Themeable); ok {
			t.SetTheme(newTheme)
		}
	}
}
