package ingest

import (
	"fmt"
	"strings"
)

// ExtractDigraph extracts a DOT digraph block from LLM output text.
// It handles: raw digraph, markdown-fenced digraphs, leading/trailing commentary.
// It uses brace counting (respecting quoted strings) to find the matching closing brace.
func ExtractDigraph(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty input")
	}

	// Find "digraph" keyword.
	idx := strings.Index(text, "digraph")
	if idx == -1 {
		return "", fmt.Errorf("no digraph found in output")
	}

	// Start from the digraph keyword.
	sub := text[idx:]

	// Find the opening brace.
	openIdx := strings.Index(sub, "{")
	if openIdx == -1 {
		return "", fmt.Errorf("digraph has no opening brace")
	}

	// Count braces to find the matching close, respecting quoted strings.
	depth := 0
	inQuote := false
	escape := false
	closeIdx := -1

	for i := openIdx; i < len(sub); i++ {
		ch := sub[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inQuote {
			escape = true
			continue
		}
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				closeIdx = i
				break
			}
		}
	}

	if closeIdx == -1 {
		return "", fmt.Errorf("unmatched braces in digraph")
	}

	return sub[:closeIdx+1], nil
}
