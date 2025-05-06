package views

import (
	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/internal/ui/theme"
)

type ListView struct {
	width, height int
	theme         theme.Theme

	tracker *service.TrackerService
	rps     store.ResourcePatchStore
}
