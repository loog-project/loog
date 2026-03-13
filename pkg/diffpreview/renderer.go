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

var DefaultRenderOptions = RenderOptions{
	IndentSize:                2,
	EnableBackgroundHighlight: true,
}

func RenderYAML(node *AnnotatedNode, theme Theme, opts RenderOptions) string {
	var sb strings.Builder
	renderNode(&sb, node, theme, opts, 0)
	return sb.String()
}

func renderNode(sb *strings.Builder, node *AnnotatedNode, theme Theme, opts RenderOptions, indent int) {
	space := strings.Repeat(" ", indent*opts.IndentSize)

	if node.Children != nil {
		keys := sortKeys(node.Children)
		for _, key := range keys {
			child := node.Children[key]

			keyStr := theme.SyntaxHighlight("key", key) + ":"
			if opts.EnableBackgroundHighlight {
				keyStr = theme.BackgroundHighlight(child.Change, keyStr)
			}

			sb.WriteString(space + keyStr)

			if child.List != nil {
				// Render as a YAML list
				sb.WriteString("\n")
				renderAnnotatedList(sb, child.List, theme, opts, indent+1, child.Change)
			} else if child.Children == nil {
				sb.WriteString(" ")
				renderValue(sb, child, theme, opts, indent)
			} else {
				sb.WriteString("\n")
				renderNode(sb, child, theme, opts, indent+1)
			}
		}
	} else if node.List != nil {
		renderAnnotatedList(sb, node.List, theme, opts, indent, node.Change)
	} else {
		renderValue(sb, node, theme, opts, indent)
	}
}

// renderAnnotatedList renders a list of annotated nodes as YAML list items.
// For map items, the first key is placed on the same line as "- " (standard YAML).
func renderAnnotatedList(sb *strings.Builder, items []*AnnotatedNode, theme Theme, opts RenderOptions, indent int, parentChange ChangeType) {
	space := strings.Repeat(" ", indent*opts.IndentSize)

	for _, item := range items {
		// Determine the effective change type for this list item
		itemChange := item.Change
		if itemChange == Unchanged && parentChange != Unchanged {
			itemChange = parentChange
		}

		prefix := "- "
		if opts.EnableBackgroundHighlight && itemChange != Unchanged {
			prefix = theme.BackgroundHighlight(itemChange, prefix)
		}

		if item.Children != nil {
			// Map item: render first key on same line as "- ", rest indented
			keys := sortKeys(item.Children)
			if len(keys) > 0 {
				firstKey := keys[0]
				firstChild := item.Children[firstKey]

				firstKeyStr := theme.SyntaxHighlight("key", firstKey) + ":"
				if opts.EnableBackgroundHighlight {
					firstKeyStr = theme.BackgroundHighlight(firstChild.Change, firstKeyStr)
				}

				sb.WriteString(space + prefix + firstKeyStr)

				if firstChild.List != nil {
					sb.WriteString("\n")
					renderAnnotatedList(sb, firstChild.List, theme, opts, indent+2, firstChild.Change)
				} else if firstChild.Children == nil {
					sb.WriteString(" ")
					renderValue(sb, firstChild, theme, opts, indent+1)
				} else {
					sb.WriteString("\n")
					renderNode(sb, firstChild, theme, opts, indent+2)
				}

				// Render remaining keys at indent+1
				for _, key := range keys[1:] {
					child := item.Children[key]

					keyStr := theme.SyntaxHighlight("key", key) + ":"
					if opts.EnableBackgroundHighlight {
						keyStr = theme.BackgroundHighlight(child.Change, keyStr)
					}

					indentStr := strings.Repeat(" ", (indent+1)*opts.IndentSize)
					sb.WriteString(indentStr + keyStr)

					if child.List != nil {
						sb.WriteString("\n")
						renderAnnotatedList(sb, child.List, theme, opts, indent+2, child.Change)
					} else if child.Children == nil {
						sb.WriteString(" ")
						renderValue(sb, child, theme, opts, indent+1)
					} else {
						sb.WriteString("\n")
						renderNode(sb, child, theme, opts, indent+2)
					}
				}
			} else {
				sb.WriteString(space + prefix + "{}\n")
			}
		} else if item.List != nil {
			// Nested list
			sb.WriteString(space + prefix + "\n")
			renderAnnotatedList(sb, item.List, theme, opts, indent+1, itemChange)
		} else {
			// Scalar item
			sb.WriteString(space + prefix)
			renderValue(sb, item, theme, opts, indent)
		}
	}
}

func renderValue(sb *strings.Builder, node *AnnotatedNode, theme Theme, opts RenderOptions, indent int) {
	switch v := node.Value.(type) {
	case map[string]any:
		// Render nested map properly
		sb.WriteString("\n")
		renderInlineMap(sb, v, theme, opts, indent+1, node.Change)
	case []any:
		// Render list properly
		sb.WriteString("\n")
		renderInlineList(sb, v, theme, opts, indent+1, node.Change)
	case string:
		content := theme.SyntaxHighlight("string", fmt.Sprintf("\"%s\"", v))
		content = maybeHighlightBackground(content, node.Change, theme, opts)
		sb.WriteString(content + "\n")
	case bool:
		content := theme.SyntaxHighlight("bool", fmt.Sprintf("%v", v))
		content = maybeHighlightBackground(content, node.Change, theme, opts)
		sb.WriteString(content + "\n")
	case int, float64:
		content := theme.SyntaxHighlight("number", fmt.Sprintf("%v", v))
		content = maybeHighlightBackground(content, node.Change, theme, opts)
		sb.WriteString(content + "\n")
	case nil:
		content := theme.SyntaxHighlight("null", "null")
		content = maybeHighlightBackground(content, node.Change, theme, opts)
		sb.WriteString(content + "\n")
	default:
		// Fallback: fmt %v
		content := fmt.Sprintf("%v", v)
		content = maybeHighlightBackground(content, node.Change, theme, opts)
		sb.WriteString(content + "\n")
	}
}

func renderInlineMap(sb *strings.Builder, m map[string]any, theme Theme, opts RenderOptions, indent int, parentChange ChangeType) {
	space := strings.Repeat(" ", indent*opts.IndentSize)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := m[k]

		keyStr := theme.SyntaxHighlight("key", k) + ":"
		if opts.EnableBackgroundHighlight && parentChange != Unchanged {
			keyStr = theme.BackgroundHighlight(parentChange, keyStr)
		}
		sb.WriteString(space + keyStr + " ")

		switch v := val.(type) {
		case string:
			content := theme.SyntaxHighlight("string", fmt.Sprintf("\"%s\"", v))
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(content + "\n")
		case bool:
			content := theme.SyntaxHighlight("bool", fmt.Sprintf("%v", v))
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(content + "\n")
		case int, float64:
			content := theme.SyntaxHighlight("number", fmt.Sprintf("%v", v))
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(content + "\n")
		case nil:
			content := theme.SyntaxHighlight("null", "null")
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(content + "\n")
		case map[string]any:
			sb.WriteString("\n")
			renderInlineMap(sb, v, theme, opts, indent+1, parentChange)
		case []any:
			sb.WriteString("\n")
			renderInlineList(sb, v, theme, opts, indent+1, parentChange)
		default:
			sb.WriteString(fmt.Sprintf("%v\n", v))
		}
	}
}

func renderInlineList(sb *strings.Builder, list []any, theme Theme, opts RenderOptions, indent int, parentChange ChangeType) {
	space := strings.Repeat(" ", indent*opts.IndentSize)
	for _, item := range list {
		prefix := "- "
		if opts.EnableBackgroundHighlight && parentChange != Unchanged {
			prefix = theme.BackgroundHighlight(parentChange, prefix)
		}

		switch v := item.(type) {
		case string:
			content := theme.SyntaxHighlight("string", fmt.Sprintf("\"%s\"", v))
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(space + prefix + content + "\n")
		case bool:
			content := theme.SyntaxHighlight("bool", fmt.Sprintf("%v", v))
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(space + prefix + content + "\n")
		case int, float64:
			content := theme.SyntaxHighlight("number", fmt.Sprintf("%v", v))
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(space + prefix + content + "\n")
		case nil:
			content := theme.SyntaxHighlight("null", "null")
			if opts.EnableBackgroundHighlight && parentChange != Unchanged {
				content = theme.BackgroundHighlight(parentChange, content)
			}
			sb.WriteString(space + prefix + content + "\n")
		case map[string]any:
			// First key on same line as "- "
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				firstKey := keys[0]
				firstKeyStr := theme.SyntaxHighlight("key", firstKey) + ":"
				if opts.EnableBackgroundHighlight && parentChange != Unchanged {
					firstKeyStr = theme.BackgroundHighlight(parentChange, firstKeyStr)
				}
				sb.WriteString(space + prefix + firstKeyStr + " ")

				firstVal := v[firstKey]
				switch fv := firstVal.(type) {
				case map[string]any:
					sb.WriteString("\n")
					renderInlineMap(sb, fv, theme, opts, indent+2, parentChange)
				case []any:
					sb.WriteString("\n")
					renderInlineList(sb, fv, theme, opts, indent+2, parentChange)
				default:
					writeScalar(sb, fv, theme, opts, parentChange)
				}

				// Remaining keys at indent+1
				for _, key := range keys[1:] {
					subKeyStr := theme.SyntaxHighlight("key", key) + ":"
					if opts.EnableBackgroundHighlight && parentChange != Unchanged {
						subKeyStr = theme.BackgroundHighlight(parentChange, subKeyStr)
					}
					subSpace := strings.Repeat(" ", (indent+1)*opts.IndentSize)
					sb.WriteString(subSpace + subKeyStr + " ")

					subVal := v[key]
					switch sv := subVal.(type) {
					case map[string]any:
						sb.WriteString("\n")
						renderInlineMap(sb, sv, theme, opts, indent+2, parentChange)
					case []any:
						sb.WriteString("\n")
						renderInlineList(sb, sv, theme, opts, indent+2, parentChange)
					default:
						writeScalar(sb, sv, theme, opts, parentChange)
					}
				}
			} else {
				sb.WriteString(space + prefix + "{}\n")
			}
		case []any:
			sb.WriteString(space + prefix + "\n")
			renderInlineList(sb, v, theme, opts, indent+1, parentChange)
		default:
			sb.WriteString(space + prefix + fmt.Sprintf("%v\n", v))
		}
	}
}

// writeScalar writes a syntax-highlighted scalar value followed by newline.
func writeScalar(sb *strings.Builder, val any, theme Theme, opts RenderOptions, change ChangeType) {
	switch v := val.(type) {
	case string:
		content := theme.SyntaxHighlight("string", fmt.Sprintf("\"%s\"", v))
		if opts.EnableBackgroundHighlight && change != Unchanged {
			content = theme.BackgroundHighlight(change, content)
		}
		sb.WriteString(content + "\n")
	case bool:
		content := theme.SyntaxHighlight("bool", fmt.Sprintf("%v", v))
		if opts.EnableBackgroundHighlight && change != Unchanged {
			content = theme.BackgroundHighlight(change, content)
		}
		sb.WriteString(content + "\n")
	case int, float64:
		content := theme.SyntaxHighlight("number", fmt.Sprintf("%v", v))
		if opts.EnableBackgroundHighlight && change != Unchanged {
			content = theme.BackgroundHighlight(change, content)
		}
		sb.WriteString(content + "\n")
	case nil:
		content := theme.SyntaxHighlight("null", "null")
		if opts.EnableBackgroundHighlight && change != Unchanged {
			content = theme.BackgroundHighlight(change, content)
		}
		sb.WriteString(content + "\n")
	default:
		sb.WriteString(fmt.Sprintf("%v\n", v))
	}
}

func maybeHighlightBackground(content string, change ChangeType, theme Theme, opts RenderOptions) string {
	if opts.EnableBackgroundHighlight {
		return theme.BackgroundHighlight(change, content)
	}
	return content
}

func sortKeys(m map[string]*AnnotatedNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
