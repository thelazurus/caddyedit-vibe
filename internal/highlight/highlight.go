// Package highlight provides lightweight, regex-free syntax coloring for
// Caddyfile lines. It is intentionally heuristic rather than a full
// grammar-aware tokenizer: good enough for readability, not a validator.
package highlight

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

type Kind int

const (
	Default Kind = iota
	Comment
	String
	Placeholder
	Matcher
	Keyword
	Punctuation
	SnippetName
)

type Token struct {
	Text string
	Kind Kind
}

var (
	StyleComment     = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)
	StyleString      = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	StylePlaceholder = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	StyleMatcher     = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	StyleKeyword     = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	StylePunctuation = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	StyleSnippet     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	StyleDefault     = lipgloss.NewStyle()
)

func StyleFor(k Kind) lipgloss.Style {
	switch k {
	case Comment:
		return StyleComment
	case String:
		return StyleString
	case Placeholder:
		return StylePlaceholder
	case Matcher:
		return StyleMatcher
	case Keyword:
		return StyleKeyword
	case Punctuation:
		return StylePunctuation
	case SnippetName:
		return StyleSnippet
	default:
		return StyleDefault
	}
}

var keywords = map[string]bool{
	"reverse_proxy": true, "tls": true, "route": true, "handle": true,
	"handle_path": true, "header": true, "header_up": true, "header_down": true,
	"redir": true, "import": true, "uri": true, "forward_auth": true,
	"trusted_proxies": true, "copy_headers": true, "dns": true, "not": true,
	"host": true, "respond": true, "rewrite": true, "encode": true, "log": true,
	"root": true, "file_server": true, "php_fastcgi": true, "basicauth": true,
	"request_header": true, "try_files": true, "replace": true, "path": true,
	"method": true, "expression": true, "vars": true, "map": true,
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}

// Tokenize splits a single line (no newline) into styled segments.
func Tokenize(line string) []Token {
	runes := []rune(line)
	n := len(runes)
	var toks []Token
	push := func(text string, kind Kind) {
		if text == "" {
			return
		}
		if len(toks) > 0 && toks[len(toks)-1].Kind == kind && kind == Default {
			toks[len(toks)-1].Text += text
			return
		}
		toks = append(toks, Token{Text: text, Kind: kind})
	}

	// Snippet definition header: "(name) {" — highlight the whole
	// parenthesized name distinctly.
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "(") {
		if end := strings.Index(trimmed, ")"); end > 0 {
			lead := len(line) - len(trimmed)
			push(line[:lead], Default)
			push(trimmed[:end+1], SnippetName)
			rest := Tokenize(trimmed[end+1:])
			return append(toks, rest...)
		}
	}

	i := 0
	for i < n {
		r := runes[i]
		switch {
		case r == '#':
			push(string(runes[i:]), Comment)
			i = n

		case r == '"':
			j := i + 1
			for j < n {
				if runes[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if runes[j] == '"' {
					j++
					break
				}
				j++
			}
			push(string(runes[i:j]), String)
			i = j

		case r == '{':
			// Try a simple non-nested placeholder: {no-whitespace-or-braces}
			j := i + 1
			ok := false
			for j < n {
				if runes[j] == '}' {
					ok = true
					break
				}
				if runes[j] == '{' || unicode.IsSpace(runes[j]) {
					break
				}
				j++
			}
			if ok && j > i+1 {
				push(string(runes[i:j+1]), Placeholder)
				i = j + 1
			} else {
				push("{", Punctuation)
				i++
			}

		case r == '}':
			push("}", Punctuation)
			i++

		case r == '@':
			j := i + 1
			for j < n && isWordRune(runes[j]) {
				j++
			}
			push(string(runes[i:j]), Matcher)
			i = j

		case isWordRune(r):
			j := i + 1
			for j < n && isWordRune(runes[j]) {
				j++
			}
			word := string(runes[i:j])
			if keywords[strings.ToLower(word)] {
				push(word, Keyword)
			} else {
				push(word, Default)
			}
			i = j

		default:
			push(string(r), Default)
			i++
		}
	}
	return toks
}

// Render returns the ANSI-styled version of a line for display.
func Render(line string) string {
	var b strings.Builder
	for _, t := range Tokenize(line) {
		b.WriteString(StyleFor(t.Kind).Render(t.Text))
	}
	return b.String()
}
