package jira

import (
	"fmt"
	"strings"
)

func adfToMarkdown(value any) string {
	return strings.TrimSpace(renderADFNode(value, 0))
}

func renderADFNode(value any, depth int) string {
	node, ok := value.(map[string]any)
	if !ok {
		if text, ok := value.(string); ok {
			return text
		}
		return ""
	}

	typeName, _ := node["type"].(string)
	switch typeName {
	case "doc":
		return renderChildren(node, depth, "\n\n")
	case "paragraph":
		return strings.TrimSpace(renderChildren(node, depth, ""))
	case "text":
		text, _ := node["text"].(string)
		return applyTextMarks(text, node)
	case "hardBreak":
		return "\n"
	case "bulletList":
		return renderList(node, depth, false)
	case "orderedList":
		return renderList(node, depth, true)
	case "listItem":
		return strings.TrimSpace(renderChildren(node, depth+1, "\n"))
	case "codeBlock":
		language := ""
		if attrs, ok := node["attrs"].(map[string]any); ok {
			language, _ = attrs["language"].(string)
		}
		return fmt.Sprintf("```%s\n%s\n```", language, strings.TrimSpace(renderChildren(node, depth, "\n")))
	case "heading":
		level := 1
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if raw, ok := attrs["level"].(int); ok && raw > 0 {
				level = raw
			}
		}
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		return strings.Repeat("#", level) + " " + strings.TrimSpace(renderChildren(node, depth, ""))
	case "blockquote":
		text := strings.TrimSpace(renderChildren(node, depth, "\n"))
		if text == "" {
			return ""
		}
		lines := strings.Split(text, "\n")
		for index, line := range lines {
			lines[index] = "> " + line
		}
		return strings.Join(lines, "\n")
	case "rule":
		return "---"
	case "emoji":
		attrs, _ := node["attrs"].(map[string]any)
		if shortName, ok := attrs["shortName"].(string); ok {
			return shortName
		}
		return ""
	case "mention":
		attrs, _ := node["attrs"].(map[string]any)
		if text, ok := attrs["text"].(string); ok {
			return text
		}
		return ""
	default:
		return renderChildren(node, depth, "")
	}
}

func renderChildren(node map[string]any, depth int, separator string) string {
	children, _ := node["content"].([]any)
	parts := make([]string, 0, len(children))
	for _, child := range children {
		text := renderADFNode(child, depth)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, separator)
}

func renderList(node map[string]any, depth int, ordered bool) string {
	children, _ := node["content"].([]any)
	lines := make([]string, 0, len(children))
	for index, child := range children {
		text := strings.TrimSpace(renderADFNode(child, depth+1))
		if text == "" {
			continue
		}
		prefix := "- "
		if ordered {
			prefix = fmt.Sprintf("%d. ", index+1)
		}
		indent := strings.Repeat("  ", depth)
		text = strings.ReplaceAll(text, "\n", "\n"+indent+"  ")
		lines = append(lines, indent+prefix+text)
	}
	return strings.Join(lines, "\n")
}

func applyTextMarks(text string, node map[string]any) string {
	marks, _ := node["marks"].([]any)
	for _, rawMark := range marks {
		mark, ok := rawMark.(map[string]any)
		if !ok {
			continue
		}
		switch mark["type"] {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "_" + text + "_"
		case "code":
			text = "`" + text + "`"
		case "link":
			attrs, _ := mark["attrs"].(map[string]any)
			href, _ := attrs["href"].(string)
			if href != "" {
				text = fmt.Sprintf("[%s](%s)", text, href)
			}
		}
	}
	return text
}
