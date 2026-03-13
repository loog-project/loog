package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/resource"
)

// RunAnalysisCmd returns a tea.Cmd that performs background analysis
// on a ResourceData and returns the result as an AnalysisCompleteMsg.
// The analysis runs in a goroutine with a small delay to demonstrate
// the async pattern that will be used with a real store.
func RunAnalysisCmd(rd *ResourceData) tea.Cmd {
	if rd == nil {
		return nil
	}
	uid := rd.Resource.UID
	revisions := make([]Revision, len(rd.Revisions))
	copy(revisions, rd.Revisions)

	return func() tea.Msg {
		time.Sleep(200 * time.Millisecond)

		result := resource.Analyze(&ResourceData{
			Resource:  Resource{UID: uid},
			Revisions: revisions,
		}, 8)

		return AnalysisCompleteMsg{Result: result}
	}
}
