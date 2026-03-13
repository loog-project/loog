package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/resource"
)

// RunAnalysisCmd returns a tea.Cmd that performs background analysis
// on a ResourceData and returns the result as an AnalysisCompleteMsg.
func RunAnalysisCmd(rd *ResourceData) tea.Cmd {
	if rd == nil {
		return nil
	}
	uid := rd.Resource.UID
	revisions := make([]Revision, len(rd.Revisions))
	copy(revisions, rd.Revisions)

	return func() tea.Msg {
		result := resource.Analyze(&ResourceData{
			Resource:  Resource{UID: uid},
			Revisions: revisions,
		}, 8)

		return AnalysisCompleteMsg{Result: result}
	}
}
