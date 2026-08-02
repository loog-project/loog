package diffpreview

import (
	"fmt"
	"sort"
	"strings"
)

type RenderOptions struct {
	IndentSize                int
	EnableBackgroundHighlight bool
}

// RenderYAML turns an AnnotatedNode tree into syntax-highlighted YAML with
// colored backgrounds on changed lines.
func RenderYAML(node *AnnotatedNode, theme Theme, opts RenderOptions) string {
	r := renderer{theme: theme, opts: opts}
	r.renderNode(node, 0)
	return r.sb.String()
}

type renderer struct {
	sb    strings.Builder
	theme Theme
	opts  RenderOptions
}

func (r *renderer) indent(level int) string {
	return strings.Repeat(" ", level*r.opts.IndentSize)
}

// renderNode dispatches to the right renderer based on node shape.
func (r *renderer) renderNode(node *AnnotatedNode, level int) {
	switch {
	case node.Children != nil:
		r.renderMap(node.Children, level)
	case node.List != nil:
		r.renderList(node.List, level, node.Change)
	default:
		r.renderLeaf(node.Value, node.Change)
	}
}

func (r *renderer) renderMap(children map[string]*AnnotatedNode, level int) {
	prefix := r.indent(level)
	for _, key := range sortedKeys(children) {
		child := children[key]
		r.sb.WriteString(prefix + r.styledKey(key, child.Change))
		r.renderChildValue(child, level)
	}
}

// renderList writes a YAML sequence. Map items get their first key on the
// same line as "- " to match standard YAML block style.
func (r *renderer) renderList(items []*AnnotatedNode, level int, parentChange ChangeType) {
	prefix := r.indent(level)

	for _, item := range items {
		change := effectiveChange(item.Change, parentChange)
		dash := r.styledDash(change)

		switch {
		case item.Children != nil:
			r.renderListMapItem(item.Children, prefix, dash, level, change)
		case item.List != nil:
			if len(item.List) == 0 {
				r.sb.WriteString(prefix + dash + "[]\n")
				break
			}
			r.sb.WriteString(prefix + dash + "\n")
			r.renderList(item.List, level+1, change)
		default:
			r.sb.WriteString(prefix + dash)
			r.renderLeaf(item.Value, change)
		}
	}
}

// renderListMapItem writes a map inside a list. The first key shares the
// "- " line; the rest are indented one level deeper.
func (r *renderer) renderListMapItem(children map[string]*AnnotatedNode, prefix, dash string, level int, _ ChangeType) {
	keys := sortedKeys(children)
	if len(keys) == 0 {
		r.sb.WriteString(prefix + dash + "{}\n")
		return
	}

	first := children[keys[0]]
	r.sb.WriteString(prefix + dash + r.styledKey(keys[0], first.Change))
	r.renderChildValue(first, level+1)

	rest := r.indent(level + 1)
	for _, key := range keys[1:] {
		child := children[key]
		r.sb.WriteString(rest + r.styledKey(key, child.Change))
		r.renderChildValue(child, level+1)
	}
}

// renderChildValue writes everything after "key:" depending on whether the
// child is a map, list, or scalar.
func (r *renderer) renderChildValue(child *AnnotatedNode, level int) {
	switch {
	case child.List != nil:
		if len(child.List) == 0 {
			r.sb.WriteString(" []\n")
			return
		}
		r.sb.WriteString("\n")
		r.renderList(child.List, level+1, child.Change)
	case child.Children != nil:
		if len(child.Children) == 0 {
			r.sb.WriteString(" {}\n")
			return
		}
		r.sb.WriteString("\n")
		r.renderMap(child.Children, level+1)
	default:
		r.sb.WriteString(" ")
		r.renderLeaf(child.Value, child.Change)
	}
}

// renderLeaf writes a syntax-highlighted scalar, optionally with a diff
// background colour, followed by a newline.
func (r *renderer) renderLeaf(val any, change ChangeType) {
	content := r.syntaxHighlight(val)
	if r.opts.EnableBackgroundHighlight {
		if bg := r.theme.backgroundStyle(change); bg != nil {
			content = bg.Render(content)
		}
	}
	r.sb.WriteString(content + "\n")
}

func (r *renderer) syntaxHighlight(val any) string {
	switch v := val.(type) {
	case string:
		return r.theme.StringStyle.Render(fmt.Sprintf("%q", v))
	case bool:
		return r.theme.BoolStyle.Render(fmt.Sprintf("%v", v))
	case int:
		return r.theme.NumberStyle.Render(fmt.Sprintf("%d", v))
	case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return r.theme.NumberStyle.Render(fmt.Sprintf("%d", v))
	case float32, float64:
		return r.theme.NumberStyle.Render(fmt.Sprintf("%v", v))
	case nil:
		return r.theme.NullStyle.Render("null")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (r *renderer) styledKey(key string, change ChangeType) string {
	s := r.theme.KeyStyle.Render(key) + ":"
	if r.opts.EnableBackgroundHighlight {
		if bg := r.theme.backgroundStyle(change); bg != nil {
			s = bg.Render(s)
		}
	}
	return s
}

func (r *renderer) styledDash(change ChangeType) string {
	dash := "- "
	if r.opts.EnableBackgroundHighlight && change != Unchanged {
		if bg := r.theme.backgroundStyle(change); bg != nil {
			dash = bg.Render(dash)
		}
	}
	return dash
}

// effectiveChange picks the item's own change type unless it is Unchanged,
// in which case it inherits from the parent.
func effectiveChange(item, parent ChangeType) ChangeType {
	if item != Unchanged {
		return item
	}
	return parent
}

func sortedKeys(m map[string]*AnnotatedNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
