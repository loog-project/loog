package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// Force TrueColor in tests so lipgloss emits real ANSI escape codes.
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// rendered returns the visual width and height of a rendered string,
// using lipgloss measurements which account for ANSI escapes.
func rendered(s string) (w, h int) {
	return lipgloss.Width(s), lipgloss.Height(s)
}

// assertDimensions checks that a rendered string has exactly the expected
// visual width and height. Width is checked per-line to catch overflow.
func assertDimensions(t *testing.T, label string, s string, wantW, wantH int) {
	t.Helper()
	_, gotH := rendered(s)
	if gotH != wantH {
		t.Errorf("%s: height = %d, want %d", label, gotH, wantH)
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lw := lipgloss.Width(line)
		if lw > wantW {
			t.Errorf("%s: line %d width = %d, exceeds max %d: %q",
				label, i, lw, wantW, line[:min(len(line), 80)])
		}
	}
}

// ---------------------------------------------------------------------------
// Truncate
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty", "", 5, ""},
		{"short enough", "abc", 5, "abc"},
		{"exact fit", "abcde", 5, "abcde"},
		{"truncated", "abcdef", 5, "abcd…"},
		{"max 0", "abc", 0, ""},
		{"max 1", "abc", 1, "…"},
		{"unicode", "こんにちは世界", 5, "こんにち…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PadRight
// ---------------------------------------------------------------------------

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		wantW int
	}{
		{"needs padding", "hi", 10, 10},
		{"exact width", "hello", 5, 5},
		{"already wider", "toolong", 3, 7},
		{"empty string", "", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.input, tt.width)
			if lipgloss.Width(got) != tt.wantW {
				t.Errorf("PadRight(%q, %d) visual width = %d, want %d",
					tt.input, tt.width, lipgloss.Width(got), tt.wantW)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PanelBorderEx
// ---------------------------------------------------------------------------

func TestPanelBorderEx_Dimensions(t *testing.T) {
	theme := CatppuccinMocha
	tests := []struct {
		name    string
		content string
		title   string
		w, h    int
		focused bool
		scrollU bool
		scrollD bool
	}{
		{"normal", "hello", "Title", 30, 10, true, false, false},
		{"empty content", "", "Panel", 40, 12, false, false, false},
		{"scrollers", "content", "Scroll", 30, 10, true, true, true},
		{"no title", "data", "", 20, 8, false, false, false},
		{"min size no title", "x", "", 4, 3, false, false, false},
		{"multi-line", "line1\nline2\nline3", "M", 20, 6, true, false, false},
		{"wide content", strings.Repeat("x", 100), "Wide", 30, 5, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := PanelBorderEx(tt.content, tt.title, tt.w, tt.h, tt.focused, theme, tt.scrollU, tt.scrollD)
			assertDimensions(t, tt.name, out, tt.w, tt.h)
		})
	}
}

func TestPanelBorderEx_TooSmall(t *testing.T) {
	theme := CatppuccinMocha
	// When width < 4 or height < 3, it returns raw content
	out := PanelBorderEx("content", "T", 3, 2, false, theme, false, false)
	if out != "content" {
		t.Errorf("expected raw content for too-small panel, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// SplitHorizontal / SplitThreeColumns
// ---------------------------------------------------------------------------

func TestSplitHorizontal_Height(t *testing.T) {
	theme := CatppuccinMocha
	// Build columns without trailing newline
	leftLines := make([]string, 10)
	rightLines := make([]string, 10)
	for i := range 10 {
		leftLines[i] = "L"
		rightLines[i] = "R"
	}
	left := strings.Join(leftLines, "\n")
	right := strings.Join(rightLines, "\n")
	out := SplitHorizontal(left, right, 10, theme)
	_, h := rendered(out)
	if h != 10 {
		t.Errorf("SplitHorizontal height = %d, want 10", h)
	}
}

func TestSplitThreeColumns_Height(t *testing.T) {
	theme := CatppuccinMocha
	colLines := make([]string, 8)
	for i := range 8 {
		colLines[i] = "X"
	}
	col := strings.Join(colLines, "\n")
	out := SplitThreeColumns(col, col, col, 8, theme)
	_, h := rendered(out)
	if h != 8 {
		t.Errorf("SplitThreeColumns height = %d, want 8", h)
	}
}

// ---------------------------------------------------------------------------
// ModalOverlay
// ---------------------------------------------------------------------------

func TestModalOverlay_Dimensions(t *testing.T) {
	theme := CatppuccinMocha
	bg := strings.Repeat(strings.Repeat("X", 60)+"\n", 20)
	bg = strings.TrimSuffix(bg, "\n")
	modal := "dialog\nline2"

	out := ModalOverlay(modal, bg, theme)
	w, h := rendered(out)
	if h != 20 {
		t.Errorf("ModalOverlay height = %d, want 20", h)
	}
	if w > 60 {
		t.Errorf("ModalOverlay width = %d, exceeds bg width 60", w)
	}
}

// ---------------------------------------------------------------------------
// ScrollPosition
// ---------------------------------------------------------------------------

func TestScrollPosition(t *testing.T) {
	tests := []struct {
		cursor, total int
		want          string
	}{
		{0, 0, "0/0"},
		{0, 5, "1/5"},
		{4, 5, "5/5"},
	}
	for _, tt := range tests {
		got := ScrollPosition(tt.cursor, tt.total)
		if got != tt.want {
			t.Errorf("ScrollPosition(%d, %d) = %q, want %q", tt.cursor, tt.total, got, tt.want)
		}
	}
}
