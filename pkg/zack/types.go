package zack

// Focusable can be used to check if a widget or component can be focused and check the current focus state
type Focusable interface {
	SetFocus(active bool)
	HasFocus() bool
}

var _ Focusable = (*Focuser)(nil)

type Focuser struct {
	Focus bool
}

func (f *Focuser) SetFocus(active bool) {
	f.Focus = active
}

func (f *Focuser) HasFocus() bool {
	return f.Focus
}

// =====================================================================================================================

type Boundable interface {
	SetBounds(bounds Bounds)
}

type Bounds struct {
	Width  int
	Height int
}

func NewBounds(width, height int) Bounds {
	return Bounds{
		Width:  width,
		Height: height,
	}
}

// IsValid checks if the bounds are valid. A bound is considered valid if both width and height are greater than zero.
func (b Bounds) IsValid() bool {
	return b.Width > 0 && b.Height > 0
}
