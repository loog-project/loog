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

			if child.Children == nil {
				sb.WriteString(" ")
				renderValue(sb, child, theme, opts)
			} else {
				sb.WriteString("\n")
				renderNode(sb, child, theme, opts, indent+1)
			}
		}
	} else {
		renderValue(sb, node, theme, opts)
	}
}

func renderValue(sb *strings.Builder, node *AnnotatedNode, theme Theme, opts RenderOptions) {
	switch v := node.Value.(type) {
	case map[string]any:
		// Render nested map properly
		sb.WriteString("\n")
		renderInlineMap(sb, v, theme, opts, 1)
	case []any:
		// Render list properly
		sb.WriteString("\n")
		renderInlineList(sb, v, theme, opts, 1)
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

func renderInlineMap(sb *strings.Builder, m map[string]any, theme Theme, opts RenderOptions, indent int) {
	space := strings.Repeat(" ", indent*opts.IndentSize)

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := m[k]

		keyStr := theme.SyntaxHighlight("key", k) + ":"
		sb.WriteString(space + keyStr + " ")

		switch v := val.(type) {
		case string:
			sb.WriteString(theme.SyntaxHighlight("string", fmt.Sprintf("\"%s\"", v)) + "\n")
		case bool:
			sb.WriteString(theme.SyntaxHighlight("bool", fmt.Sprintf("%v", v)) + "\n")
		case int, float64:
			sb.WriteString(theme.SyntaxHighlight("number", fmt.Sprintf("%v", v)) + "\n")
		case nil:
			sb.WriteString(theme.SyntaxHighlight("null", "null") + "\n")
		case map[string]any:
			sb.WriteString("\n")
			renderInlineMap(sb, v, theme, opts, indent+1)
		case []any:
			sb.WriteString("\n")
			renderInlineList(sb, v, theme, opts, indent+1)
		default:
			sb.WriteString(fmt.Sprintf("%v\n", v))
		}
	}
}

func renderInlineList(sb *strings.Builder, list []any, theme Theme, opts RenderOptions, indent int) {
	space := strings.Repeat(" ", indent*opts.IndentSize)
	for _, item := range list {
		sb.WriteString(space + "- ")

		switch v := item.(type) {
		case string:
			sb.WriteString(theme.SyntaxHighlight("string", fmt.Sprintf("\"%s\"", v)) + "\n")
		case bool:
			sb.WriteString(theme.SyntaxHighlight("bool", fmt.Sprintf("%v", v)) + "\n")
		case int, float64:
			sb.WriteString(theme.SyntaxHighlight("number", fmt.Sprintf("%v", v)) + "\n")
		case nil:
			sb.WriteString(theme.SyntaxHighlight("null", "null") + "\n")
		case map[string]any:
			sb.WriteString("\n")
			renderInlineMap(sb, v, theme, opts, indent+1)
		case []any:
			sb.WriteString("\n")
			renderInlineList(sb, v, theme, opts, indent+1)
		default:
			sb.WriteString(fmt.Sprintf("%v\n", v))
		}
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
