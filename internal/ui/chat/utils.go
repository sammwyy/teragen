package chat

import (
	"os"
	"strings"
	"unicode/utf8"
)

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var sb strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			sb.WriteRune('\n')
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		currentLineWidth := 0
		for j, word := range words {
			wordWidth := utf8.RuneCountInString(word)

			if j > 0 {
				if currentLineWidth+1+wordWidth > width {
					sb.WriteRune('\n')
					currentLineWidth = 0
				} else {
					sb.WriteRune(' ')
					currentLineWidth++
				}
			}

			sb.WriteString(word)
			currentLineWidth += wordWidth
		}
	}

	return sb.String()
}

func (m *Model) renderPath(path string, maxWidth int) string {
	sep := string(os.PathSeparator)
	parts := strings.Split(path, sep)
	if len(parts) == 0 {
		return ""
	}

	type node struct {
		text    string
		isDots  bool
		isStart bool
		isEnd   bool
	}

	nodes := make([]node, len(parts))
	for i, p := range parts {
		nodes[i] = node{text: p, isStart: i == 0, isEnd: i == len(parts)-1}
	}

	calcWidth := func(ns []node) int {
		total := 0
		for i, n := range ns {
			total += utf8.RuneCountInString(n.text)
			if i < len(ns)-1 {
				total += 1
			}
		}
		return total
	}

	for calcWidth(nodes) > maxWidth && len(nodes) > 2 {
		mid := len(nodes) / 2
		removed := false
		for d := 0; d < len(nodes); d++ {
			i := mid + d
			if i > 0 && i < len(nodes)-1 && !nodes[i].isDots {
				nodes[i].text = "..."
				nodes[i].isDots = true
				removed = true
				break
			}
			i = mid - d
			if i > 0 && i < len(nodes)-1 && !nodes[i].isDots {
				nodes[i].text = "..."
				nodes[i].isDots = true
				removed = true
				break
			}
		}

		if !removed {
			for i := 1; i < len(nodes)-1; i++ {
				if nodes[i].isDots {
					nodes = append(nodes[:i], nodes[i+1:]...)
					removed = true
					break
				}
			}
		}

		if !removed {
			break
		}
	}

	var sb strings.Builder
	for i, n := range nodes {
		style := pathMiddleStyle
		if n.isDots {
			style = pathDotsStyle
		} else if n.isStart {
			style = pathStartStyle
		} else if n.isEnd {
			style = pathEndStyle
		}

		sb.WriteString(style.Render(n.text))
		if i < len(nodes)-1 {
			sb.WriteString(pathSepStyle.Render(sep))
		}
	}
	return sb.String()
}
