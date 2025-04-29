package diffpreview

// Render renders a YAML-like diff view between a and b
func Render(a, b map[string]any, theme Theme) string {
	node := Diff(a, b)
	return RenderYAML(node, theme, DefaultRenderOptions)
}

// RenderWithOptions renders a YAML-like diff view with custom options
func RenderWithOptions(a, b map[string]any, theme Theme, opts RenderOptions) string {
	node := Diff(a, b)
	return RenderYAML(node, theme, opts)
}
