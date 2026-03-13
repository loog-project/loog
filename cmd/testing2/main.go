package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

/* ------------------------------------------------------------------
TIMELINE  v4 – adds **diagonal path arrows** between consecutive
revisions (like git log’s branching lines).
Controls: ↑/↓ scroll  ←/→ focus column  +/- zoom rows  q quit
------------------------------------------------------------------*/

type Revision struct {
	Timestamp time.Time
	Manifest  string
}

type TimelineModel struct {
	resources []string
	history   map[string][]Revision

	selRes, selRev int // focused cell
	offset         int // top row offset (scroll)

	width, height int
	rowHeight     int
}

/* ---------------- mock data ---------------- */
func NewTimelineModel() TimelineModel {
	base := time.Now().Add(-8 * time.Hour)
	makeRevs := func(n int, gap time.Duration) []Revision {
		out := make([]Revision, n)
		t := base
		for i := 0; i < n; i++ {
			out[i] = Revision{Timestamp: t, Manifest: fmt.Sprintf("rev %02d", i)}
			t = t.Add(-gap - time.Duration(i%3)*20*time.Minute)
		}
		return out
	}

	hist := map[string][]Revision{
		"deployment/api":      makeRevs(11, 50*time.Minute),
		"deployment/frontend": makeRevs(15, 35*time.Minute),
		"statefulset/redis":   makeRevs(9, 70*time.Minute),
	}
	res := []string{"deployment/api", "deployment/frontend", "statefulset/redis"}
	return TimelineModel{resources: res, history: hist, rowHeight: 15}
}

/* ---------------- styles ---------------- */
var palette = []lipgloss.Color{
	lipgloss.Color("81"), lipgloss.Color("208"), lipgloss.Color("141"),
}

func colStyle(i int) lipgloss.Style { return lipgloss.NewStyle().Foreground(palette[i%len(palette)]) }

var (
	timeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	legendStyle = lipgloss.NewStyle().Faint(true)
	cellSelBg   = lipgloss.Color("238")
	borderBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

/* ---------------- tea.Model ---------------- */
func (m TimelineModel) Init() tea.Cmd { return nil }

func (m TimelineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.scroll(-1)
		case "down", "j":
			m.scroll(1)
		case "left", "h":
			if m.selRes > 0 {
				m.selRes--
			}
		case "right", "l":
			if m.selRes < len(m.resources)-1 {
				m.selRes++
			}
		case "+":
			if m.rowHeight > 5 {
				m.rowHeight--
			}
		case "-":
			m.rowHeight++
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	}
	return m, nil
}

func (m *TimelineModel) scroll(delta int) {
	maxRows := len(m.buildTimelineRows())
	m.offset += delta
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxRows-1 {
		m.offset = maxRows - 1
	}
}

/* ---------------- View ----------------- */
func (m TimelineModel) View() string {
	if m.width == 0 {
		return "initialising…"
	}

	rows := m.buildTimelineRows()

	start := m.offset
	end := start + m.rowHeight
	if end > len(rows) {
		end = len(rows)
	}
	viewRows := rows[start:end]

	legend := legendStyle.Render("↑↓ scroll  ←→ focus column  +/- zoom rows  q quit")
	content := append([]string{m.renderHeader()}, viewRows...)
	content = append(content, legend)

	return borderBox.Render(lipgloss.JoinVertical(lipgloss.Left, content...))
}

/* ----- Header */
func (m TimelineModel) renderHeader() string {
	colWidth := 20
	labels := make([]string, len(m.resources))
	for i, r := range m.resources {
		st := colStyle(i)
		if i == m.selRes {
			st = st.Background(cellSelBg).Bold(true)
		}
		labels[i] = st.Width(colWidth).Align(lipgloss.Center).Render(r)
	}
	return lipgloss.PlaceHorizontal(m.width-18, lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Left, labels...))
}

/* ----- Build rows with connection paths */
func (m TimelineModel) buildTimelineRows() []string {
	// union of timestamps
	tsMap := map[int64]bool{}
	for _, revs := range m.history {
		for _, r := range revs {
			tsMap[r.Timestamp.Unix()] = true
		}
	}
	var times []int64
	for t := range tsMap {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] > times[j] })

	colWidth := 20
	rows := make([]string, len(times))

	// map from unix->rowIdx
	rowForUnix := map[int64]int{}
	for idx, u := range times {
		rowForUnix[u] = idx
	}

	// build base markers + verticals
	colIndices := make([][]int, len(m.resources)) // rows that have rev per column

	for rowIdx, unix := range times {
		t := time.Unix(unix, 0)
		timeLbl := timeStyle.Width(16).Render(t.Format("02 Jan 15:04"))

		cols := make([]string, len(m.resources))
		for colIdx, res := range m.resources {
			st := colStyle(colIdx)
			marker := " "
			isRev := false
			_ = isRev
			for j, rev := range m.history[res] {
				if rev.Timestamp.Unix() == unix {
					isRev = true
					marker = "•"
					if colIdx == m.selRes && j == m.selRev {
						marker = "●"
						st = st.Background(cellSelBg).Bold(true)
					}
					colIndices[colIdx] = append(colIndices[colIdx], rowIdx)
					break
				}
			}
			cols[colIdx] = st.Width(colWidth).Align(lipgloss.Center).Render(marker)
		}
		rows[rowIdx] = timeLbl + " " + lipgloss.JoinHorizontal(lipgloss.Left, cols...)
	}

	// add vertical lines
	for colIdx := range m.resources {
		for k := 0; k < len(colIndices[colIdx])-1; k++ {
			a, b := colIndices[colIdx][k], colIndices[colIdx][k+1]
			if a > b {
				a, b = b, a
			}
			for r := a + 1; r < b; r++ {
				rows[r] = replaceAt(rows[r], '│', 17+1+colIdx*colWidth+colWidth/2)
			}
		}
	}

	// ----------------------------------------------------------------
	// DIAGONAL PATHS BETWEEN CONSECUTIVE REVISIONS ACROSS COLUMNS
	// ----------------------------------------------------------------
	type evt struct{ row, col int }
	var seq []evt
	for rowIdx, unix := range times {
		for colIdx, res := range m.resources {
			for _, rev := range m.history[res] {
				if rev.Timestamp.Unix() == unix {
					seq = append(seq, evt{rowIdx, colIdx})
					break
				}
			}
		}
	}

	for i := 0; i < len(seq)-1; i++ {
		src, dst := seq[i], seq[i+1]
		if src.row == dst.row {
			continue // simultaneous handled by horizontals
		}
		dir := 1
		if dst.col < src.col {
			dir = -1
		}
		// first row: corner
		cornerChar := '┐'
		if dir == -1 {
			cornerChar = '┌'
		}
		rows[src.row] = replaceAt(rows[src.row], cornerChar, 17+1+src.col*colWidth+colWidth/2)
		// walk rows between
		steps := dst.row - src.row
		if steps < 0 {
			steps = -steps
		}
		col := src.col
		for step := 1; step < steps; step++ {
			r := src.row + step*sign(dst.row-src.row)
			// move col gradually toward dst.col
			if step*len(m.resources) < steps { // rough slope
				col += dir
			}
			diag := '╲'
			if dir == -1 {
				diag = '╱'
			}
			rows[r] = replaceAt(rows[r], diag, 17+1+col*colWidth+colWidth/2)
		}
	}

	// horizontal lines between simultaneous events (same row)
	for rowIdx, unix := range times {
		colPos := []int{}
		for colIdx, res := range m.resources {
			for _, rev := range m.history[res] {
				if rev.Timestamp.Unix() == unix {
					colPos = append(colPos, colIdx)
					break
				}
			}
		}
		if len(colPos) < 2 {
			continue
		}
		first, last := colPos[0], colPos[len(colPos)-1]
		for i := first + 1; i < last; i++ {
			rows[rowIdx] = replaceAt(rows[rowIdx], '─', 17+1+i*colWidth+colWidth/2)
		}
	}

	return rows
}

func sign(x int) int {
	if x < 0 {
		return -1
	}
	return 1
}

func replaceAt(s string, r rune, idx int) string {
	rs := []rune(s)
	if idx >= 0 && idx < len(rs) {
		rs[idx] = r
	}
	return string(rs)
}

func (m *TimelineModel) ensureCursorVisible(row int) {
	if row < m.offset {
		m.offset = row
	} else if row >= m.offset+m.rowHeight {
		m.offset = row - m.rowHeight + 1
	}
}

/* ---------------- main ---------------- */
func main() {
	p := tea.NewProgram(NewTimelineModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
