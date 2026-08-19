// Package outline extracts a navigable list of top-level blocks (site
// addresses and named snippets) from a Caddyfile buffer.
package outline

import "strings"

type Block struct {
	Header    string // header line text, trimmed, brace stripped
	Comment   string // leading comment lines, joined
	StartLine int    // 0-indexed line of the header
	EndLine   int    // 0-indexed line of the matching closing brace
	IsSnippet bool
}

// Parse scans lines for top-level ({ / }) blocks, ignoring brace characters
// that appear inside quoted strings.
func Parse(lines []string) []Block {
	var blocks []Block
	depth := 0
	var cur *Block

	for i, line := range lines {
		opens, closes := braceDelta(line)

		if depth == 0 && opens > 0 {
			header := strings.TrimSpace(line)
			header = strings.TrimSuffix(header, "{")
			header = strings.TrimSpace(header)
			b := Block{
				Header:    header,
				StartLine: i,
				IsSnippet: strings.HasPrefix(header, "("),
				Comment:   leadingComment(lines, i),
			}
			cur = &b
		}

		depth += opens - closes

		if depth <= 0 && cur != nil {
			cur.EndLine = i
			blocks = append(blocks, *cur)
			cur = nil
			depth = 0 // guard against malformed/unbalanced files
		}
	}
	// Unclosed trailing block (malformed file): still surface it.
	if cur != nil {
		cur.EndLine = len(lines) - 1
		blocks = append(blocks, *cur)
	}
	return blocks
}

// braceDelta counts unquoted '{' and '}' occurrences in a line, skipping
// comments and quoted strings.
func braceDelta(line string) (opens, closes int) {
	inString := false
	escape := false
	for _, r := range line {
		if inString {
			if escape {
				escape = false
				continue
			}
			switch r {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}
		switch r {
		case '#':
			return
		case '"':
			inString = true
		case '{':
			opens++
		case '}':
			closes++
		}
	}
	return
}

// leadingComment walks upward from line index i-1 collecting contiguous
// '#'-prefixed comment lines, stopping at a blank line or non-comment.
func leadingComment(lines []string, i int) string {
	var out []string
	for j := i - 1; j >= 0; j-- {
		t := strings.TrimSpace(lines[j])
		if t == "" {
			break
		}
		if !strings.HasPrefix(t, "#") {
			break
		}
		out = append([]string{strings.TrimSpace(strings.TrimPrefix(t, "#"))}, out...)
	}
	return strings.Join(out, " ")
}
