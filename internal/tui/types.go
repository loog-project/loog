package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ViewID identifies which top-level view is active.
type ViewID int

const (
	ExplorerView ViewID = iota
	TimelineView
	CompareView
)

func (v ViewID) String() string {
	switch v {
	case ExplorerView:
		return "Explorer"
	case TimelineView:
		return "Timeline"
	case CompareView:
		return "Compare"
	default:
		return "Unknown"
	}
}

// AllViews returns all available view IDs in tab order.
func AllViews() []ViewID {
	return []ViewID{ExplorerView, TimelineView, CompareView}
}

// ViewMode determines how the detail panel renders content.
type ViewMode int

const (
	DiffMode   ViewMode = iota // Full object YAML with diff highlighting
	ObjectMode                 // Full YAML, no diff annotations
	PatchMode                  // Only changed fields
	JSONMode                   // Raw JSON
	RawMode                    // Raw database representation (debug)
)

func (m ViewMode) String() string {
	switch m {
	case DiffMode:
		return "Diff"
	case ObjectMode:
		return "Object"
	case PatchMode:
		return "Patch"
	case JSONMode:
		return "JSON"
	case RawMode:
		return "Raw"
	default:
		return "?"
	}
}

// PanelID identifies which panel has focus within a view.
type PanelID int

const (
	PanelLeft PanelID = iota
	PanelMiddle
	PanelRight
)

// RenderYAMLObject renders a map as simple YAML text with syntax highlighting.
func RenderYAMLObject(obj map[string]any, theme Theme, indent int) string {
	if obj == nil {
		return theme.MutedStyle().Render("(empty)")
	}
	var sb strings.Builder
	renderYAMLMap(&sb, obj, theme, indent, 0)
	return sb.String()
}

func renderYAMLMap(sb *strings.Builder, m map[string]any, theme Theme, indentSize, depth int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	indent := strings.Repeat(" ", depth*indentSize)
	for _, k := range keys {
		v := m[k]
		keyStr := theme.SyntaxKeyStyle().Render(k) + ":"
		switch val := v.(type) {
		case map[string]any:
			sb.WriteString(indent + keyStr + "\n")
			renderYAMLMap(sb, val, theme, indentSize, depth+1)
		case []any:
			sb.WriteString(indent + keyStr + "\n")
			renderYAMLList(sb, val, theme, indentSize, depth+1)
		case string:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxStringStyle().Render("\""+val+"\"") + "\n")
		case bool:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxBoolStyle().Render(fmt.Sprintf("%v", val)) + "\n")
		case int, int64, float64:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxNumberStyle().Render(fmt.Sprintf("%v", val)) + "\n")
		case nil:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxNullStyle().Render("null") + "\n")
		default:
			sb.WriteString(indent + keyStr + " " + fmt.Sprintf("%v", val) + "\n")
		}
	}
}

func renderYAMLList(sb *strings.Builder, list []any, theme Theme, indentSize, depth int) {
	indent := strings.Repeat(" ", depth*indentSize)
	for _, item := range list {
		switch val := item.(type) {
		case map[string]any:
			keys := make([]string, 0, len(val))
			for k := range val {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				firstKey := keys[0]
				firstVal := val[firstKey]
				firstKeyStr := theme.SyntaxKeyStyle().Render(firstKey) + ":"
				sb.WriteString(indent + "- " + firstKeyStr)
				writeYAMLValue(sb, firstVal, theme, indentSize, depth+1)
				for _, k := range keys[1:] {
					v := val[k]
					subIndent := strings.Repeat(" ", (depth+1)*indentSize)
					keyStr := theme.SyntaxKeyStyle().Render(k) + ":"
					sb.WriteString(subIndent + keyStr)
					writeYAMLValue(sb, v, theme, indentSize, depth+1)
				}
			} else {
				sb.WriteString(indent + "- {}\n")
			}
		case string:
			sb.WriteString(indent + "- " + theme.SyntaxStringStyle().Render("\""+val+"\"") + "\n")
		default:
			sb.WriteString(indent + "- " + fmt.Sprintf("%v", item) + "\n")
		}
	}
}

func writeYAMLValue(sb *strings.Builder, v any, theme Theme, indentSize, depth int) {
	switch val := v.(type) {
	case map[string]any:
		sb.WriteString("\n")
		renderYAMLMap(sb, val, theme, indentSize, depth+1)
	case []any:
		sb.WriteString("\n")
		renderYAMLList(sb, val, theme, indentSize, depth+1)
	case string:
		sb.WriteString(" " + theme.SyntaxStringStyle().Render("\""+val+"\"") + "\n")
	case bool:
		sb.WriteString(" " + theme.SyntaxBoolStyle().Render(fmt.Sprintf("%v", val)) + "\n")
	case int, int64, float64:
		sb.WriteString(" " + theme.SyntaxNumberStyle().Render(fmt.Sprintf("%v", val)) + "\n")
	case nil:
		sb.WriteString(" " + theme.SyntaxNullStyle().Render("null") + "\n")
	default:
		sb.WriteString(" " + fmt.Sprintf("%v", val) + "\n")
	}
}

// RenderJSONObject renders a map as pretty-printed JSON.
func RenderJSONObject(obj map[string]any, theme Theme) string {
	if obj == nil {
		return theme.MutedStyle().Render("(empty)")
	}
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return theme.ErrorStyle().Render("(json error: " + err.Error() + ")")
	}
	return string(b)
}
