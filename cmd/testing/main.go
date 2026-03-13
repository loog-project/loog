package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Revision represents a stored version of a K8s object.
// In a real app Manifest would hold the full YAML/JSON.
type Revision struct {
	Timestamp time.Time
	Manifest  string
}

// TimelineModel = all UI state.
// WindowStart/WindowEnd implement panning independent of the selected rev.
type TimelineModel struct {
	resources            []string
	history              map[string][]Revision
	cursorRes, cursorRev int

	windowStart, windowEnd time.Time // current viewport horizontally
	zoomFactor             float64   // 1 = full history, larger -> zoomed in

	width, height int
}

/* ------------------------- Mock data -------------------------- */
func NewTimelineModel() TimelineModel {
	base := time.Now().Add(-120 * time.Hour) // 5‑day span

	makeRevs := func(offset time.Duration, n int, gap time.Duration) []Revision {
		out := make([]Revision, n)
		t := base.Add(offset)
		for i := 0; i < n; i++ {
			out[i] = Revision{Timestamp: t, Manifest: fmt.Sprintf("kind: Config\nrev: %02d", i)}
			t = t.Add(gap + time.Duration(i*i)*gap/4 + time.Duration(i%3)*20*time.Minute)
		}
		return out
	}

	hist := map[string][]Revision{
		"deployment/api":          makeRevs(0, 18, 30*time.Minute),
		"deployment/frontend":     makeRevs(90*time.Minute, 26, 25*time.Minute),
		"deployment/payment":      makeRevs(4*time.Hour, 20, 45*time.Minute),
		"statefulset/redis":       makeRevs(8*time.Hour, 14, 75*time.Minute),
		"cronjob/cleanup":         makeRevs(12*time.Hour, 11, 90*time.Minute),
		"daemonset/log‑collector": makeRevs(16*time.Hour, 22, 40*time.Minute),
		"job/db‑migrate":          makeRevs(40*time.Hour, 8, 2*time.Hour),
	}

	// sort & gather bounds
	var minT, maxT time.Time
	for _, revs := range hist {
		sort.Slice(revs, func(i, j int) bool { return revs[i].Timestamp.Before(revs[j].Timestamp) })
		if minT.IsZero() || revs[0].Timestamp.Before(minT) {
			minT = revs[0].Timestamp
		}
		if maxT.IsZero() || revs[len(revs)-1].Timestamp.After(maxT) {
			maxT = revs[len(revs)-1].Timestamp
		}
	}

	resources := make([]string, 0, len(hist))
	for k := range hist {
		resources = append(resources, k)
	}
	sort.Strings(resources)

	return TimelineModel{
		resources:   resources,
		history:     hist,
		cursorRes:   0,
		cursorRev:   len(hist[resources[0]]) - 1,
		windowStart: minT,
		windowEnd:   maxT,
		zoomFactor:  1,
	}
}

/* ---------------- Lipgloss theme ---------------- */
var rowPalette = []lipgloss.Color{
	lipgloss.Color("141"), lipgloss.Color("208"), lipgloss.Color("51"), lipgloss.Color("10"),
	lipgloss.Color("219"), lipgloss.Color("190"), lipgloss.Color("81"), lipgloss.Color("203"),
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("60")).Padding(0, 1)
	timelineBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	manifestBox   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
	legendStyle   = lipgloss.NewStyle().Faint(true)
	manifestHdrSt = lipgloss.NewStyle().Bold(true).Underline(true)
	manifestSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	rowBgSel      = lipgloss.Color("236")
	gridStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func rowColor(i int) lipgloss.Color { return rowPalette[i%len(rowPalette)] }

/* ---------------- tea.Model impl ---------------- */
func (m TimelineModel) Init() tea.Cmd { return nil }

func (m TimelineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		// resource navigation
		case "up", "k":
			if m.cursorRes > 0 {
				m.cursorRes--
				m.cursorRev = min(m.cursorRev, len(m.history[m.resources[m.cursorRes]])-1)
			}
		case "down", "j":
			if m.cursorRes < len(m.resources)-1 {
				m.cursorRes++
				m.cursorRev = min(m.cursorRev, len(m.history[m.resources[m.cursorRes]])-1)
			}

		// revision navigation
		case "left", "h":
			if m.cursorRev > 0 {
				m.cursorRev--
			}
		case "right", "l":
			if m.cursorRev < len(m.history[m.resources[m.cursorRes]])-1 {
				m.cursorRev++
			}

		// zoom in/out
		case "+", "=":
			m.zoomFactor *= 2
			if m.zoomFactor > 64 {
				m.zoomFactor = 64
			}
			m.recalcWindow()
		case "-":
			if m.zoomFactor > 1 {
				m.zoomFactor /= 2
				m.recalcWindow()
			}
		case "0":
			m.zoomFactor = 1
			m.recalcWindow()

		// window pan (fwd/back 25% of window span)
		case ",": // back
			span := m.windowEnd.Sub(m.windowStart)
			shift := span / 4
			m.windowStart = m.windowStart.Add(-shift)
			m.windowEnd = m.windowEnd.Add(-shift)
		case ".": // forward
			span := m.windowEnd.Sub(m.windowStart)
			shift := span / 4
			m.windowStart = m.windowStart.Add(shift)
			m.windowEnd = m.windowEnd.Add(shift)
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	}
	return m, nil
}

// recalc window based on new zoom around currently selected timestamp.
func (m *TimelineModel) recalcWindow() {
	center := m.selectedTimestamp()
	full := m.getFullRange() // total span as time.Duration
	half := time.Duration(float64(full) / m.zoomFactor / 2)
	m.windowStart = center.Add(-half)
	m.windowEnd = center.Add(half)
}
func (m TimelineModel) View() string {
	if m.width == 0 {
		return "initialising..."
	}

	header := titleStyle.Width(m.width).Align(lipgloss.Center).Render("📈  Kubernetes Interactive Timeline  📜")

	tlHeight := int(float64(m.height-4) * 0.55)
	if tlHeight < len(m.resources)+4 {
		tlHeight = len(m.resources) + 4
	}

	timeline := m.renderTimeline(tlHeight)
	manifest := m.renderManifest(m.height - tlHeight - 4)

	return lipgloss.JoinVertical(lipgloss.Top, header, timelineBox.Render(timeline), manifestBox.Render(manifest))
}

/* ---------------- Timeline drawing ---------------- */
func (m TimelineModel) renderTimeline(availHeight int) string {
	barWidth := m.width - 45 // leave space for labels
	if barWidth < 40 {
		barWidth = 40
	}

	visible := m.windowEnd.Sub(m.windowStart)

	// grid lines every 20% of bar width
	gridCount := 4
	gridPos := make([]int, gridCount)
	for i := 1; i <= gridCount; i++ {
		gridPos[i-1] = int(float64(i) / float64(gridCount+1) * float64(barWidth-1))
	}

	rows := make([]string, len(m.resources)+1) // +1 for time axis

	// build time‑axis row
	axisRunes := make([]rune, barWidth)
	for i := range axisRunes {
		axisRunes[i] = ' '
	}
	for _, p := range gridPos {
		axisRunes[p] = '┬'
	}
	// time labels
	for _, p := range gridPos {
		ts := m.windowStart.Add(time.Duration(float64(visible) * float64(p) / float64(barWidth)))
		label := ts.Format("02 Jan 15:04")
		start := p - len(label)/2
		if start < 0 {
			continue
		}
		if start+len(label) >= barWidth {
			continue
		}
		for i, r := range label {
			axisRunes[start+i] = r
		}
	}
	rows[0] = gridStyle.Render(string(axisRunes))

	for i, res := range m.resources {
		line := make([]rune, barWidth)
		for j := range line {
			line[j] = '─'
		}
		// overlay grid lines
		for _, p := range gridPos {
			line[p] = '┆'
		}

		// plot rev markers
		for j, rev := range m.history[res] {
			if rev.Timestamp.Before(m.windowStart) || rev.Timestamp.After(m.windowEnd) {
				continue
			}
			pos := int(float64(rev.Timestamp.Sub(m.windowStart)) / float64(visible) * float64(barWidth-1))
			if pos < 0 || pos >= barWidth {
				continue
			}
			if i == m.cursorRes && j == m.cursorRev {
				line[pos] = '⬤'
			} else {
				line[pos] = '•'
			}
		}

		style := lipgloss.NewStyle().Foreground(rowColor(i))
		bar := style.Render(string(line))

		// label
		label := lipgloss.NewStyle().Foreground(style.GetForeground()).Bold(i == m.cursorRes).Render(truncate(res, 30))
		label = lipgloss.NewStyle().Width(30).Render(label)
		if i == m.cursorRes {
			bar = lipgloss.NewStyle().Background(rowBgSel).Render(bar)
			label = lipgloss.NewStyle().Background(rowBgSel).Render(label)
		}
		rows[i+1] = fmt.Sprintf("%s %s", label, bar)
	}

	legend := legendStyle.Render("↑/↓ res  ←/→ rev  +/- zoom  ,/. pan  0 reset  q quit")
	return lipgloss.JoinVertical(lipgloss.Top, append(rows, legend)...)
}

/* ---------------- Manifest ---------------- */
func (m TimelineModel) renderManifest(avail int) string {
	rev := m.history[m.resources[m.cursorRes]][m.cursorRev]
	hdr := manifestHdrSt.Render(fmt.Sprintf("%s — %s (%s ago)", m.resources[m.cursorRes], rev.Timestamp.Format(time.RFC822), human(time.Since(rev.Timestamp))))
	return lipgloss.JoinVertical(lipgloss.Top, hdr, manifestSt.Render(rev.Manifest))
}

/* ---------------- helpers ---------------- */
func (m TimelineModel) selectedTimestamp() time.Time {
	return m.history[m.resources[m.cursorRes]][m.cursorRev].Timestamp
}

func (m TimelineModel) getFullRange() time.Duration {
	var minT, maxT time.Time
	for _, revs := range m.history {
		if len(revs) == 0 {
			continue
		}
		if minT.IsZero() || revs[0].Timestamp.Before(minT) {
			minT = revs[0].Timestamp
		}
		if maxT.IsZero() || revs[len(revs)-1].Timestamp.After(maxT) {
			maxT = revs[len(revs)-1].Timestamp
		}
	}
	return maxT.Sub(minT)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func human(d time.Duration) string {
	switch {
	case d < time.Minute:
		return d.Truncate(time.Second).String()
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

/* ---------------- main ---------------- */
func main() {
	p := tea.NewProgram(NewTimelineModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
