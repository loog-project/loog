package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/loog-project/loog/internal/resource"
)

// ─── TUI-specific types ───

// ViewID identifies which top-level view is active.
type ViewID int

const (
	ExplorerView ViewID = iota
	TimelineView
	WatchlistView
	CompareView
)

func (v ViewID) String() string {
	switch v {
	case ExplorerView:
		return "Explorer"
	case TimelineView:
		return "Timeline"
	case WatchlistView:
		return "Watchlist"
	case CompareView:
		return "Compare"
	default:
		return "Unknown"
	}
}

// AllViews returns all available view IDs in tab order.
func AllViews() []ViewID {
	return []ViewID{ExplorerView, TimelineView, WatchlistView, CompareView}
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

// ─── Domain type aliases ───
// These re-export the domain types from internal/resource so that existing
// TUI code can continue using tui.Resource, tui.Revision, etc.
// Type aliases (=) make them identical types, not distinct wrappers.

type (
	RevisionID       = resource.RevisionID
	EventType        = resource.EventType
	Resource         = resource.Resource
	Revision         = resource.Revision
	TimelineEntry    = resource.TimelineEntry
	CompareSelection = resource.CompareSelection
	CompareItem      = resource.CompareItem
	ResourceData     = resource.ResourceData
	KindGroup        = resource.KindGroup
	ResourceKind     = resource.ResourceKind
	BurstGroup       = resource.BurstGroup
	ChangeTag        = resource.ChangeTag
	AnalysisResult   = resource.AnalysisResult
	LoopInfo         = resource.LoopInfo
	WindowMode       = resource.WindowMode
)

// Re-export constants. Type aliases don't carry constants, so we define them here.
const (
	EventAdded    = resource.EventAdded
	EventModified = resource.EventModified
	EventDeleted  = resource.EventDeleted
)

const (
	TagSpec     = resource.TagSpec
	TagStatus   = resource.TagStatus
	TagImage    = resource.TagImage
	TagLabels   = resource.TagLabels
	TagConfig   = resource.TagConfig
	TagReplicas = resource.TagReplicas
	TagUnknown  = resource.TagUnknown
)

const (
	WindowAll = resource.WindowAll
	Window15s = resource.Window15s
	Window30s = resource.Window30s
	Window1m  = resource.Window1m
	Window5m  = resource.Window5m
)

// Re-export domain functions so existing TUI code doesn't need to change imports.
var (
	KindIcon             = resource.KindIcon
	RelativeTime         = resource.RelativeTime
	FormatTimestamp      = resource.FormatTimestamp
	DeepEqual            = resource.DeepEqual
	BuildKindGroups      = resource.BuildKindGroups
	GroupTimelineByBurst = resource.GroupTimelineByBurst
	TagRevision          = resource.TagRevision
	NextWindowMode       = resource.NextWindowMode
	WindowHalfDuration   = resource.WindowHalfDuration
)

// ─── YAML / JSON Rendering (presentation layer, depends on Theme) ───

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
