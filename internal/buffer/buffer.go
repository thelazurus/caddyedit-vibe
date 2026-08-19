// Package buffer implements a minimal line-oriented text buffer for the
// caddyedit TUI: rune-indexed cursor movement and edits, nothing fancier.
package buffer

import "strings"

const IndentUnit = "    "

// Buffer holds file content as a slice of lines plus a cursor position.
type Buffer struct {
	Lines              []string
	Row, Col           int // Col is a rune index into Lines[Row]
	Dirty              bool
	HadTrailingNewline bool
}

// New builds a Buffer from raw file content.
func New(content string) *Buffer {
	trailing := strings.HasSuffix(content, "\n")
	content = strings.TrimSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	return &Buffer{Lines: lines, HadTrailingNewline: trailing || content == ""}
}

// String renders the buffer back to file content.
func (b *Buffer) String() string {
	s := strings.Join(b.Lines, "\n")
	if b.HadTrailingNewline {
		s += "\n"
	}
	return s
}

func (b *Buffer) LineCount() int { return len(b.Lines) }

func (b *Buffer) Line(i int) string {
	if i < 0 || i >= len(b.Lines) {
		return ""
	}
	return b.Lines[i]
}

func (b *Buffer) curLine() []rune { return []rune(b.Lines[b.Row]) }

func (b *Buffer) clampCol() {
	l := len([]rune(b.Lines[b.Row]))
	if b.Col > l {
		b.Col = l
	}
	if b.Col < 0 {
		b.Col = 0
	}
}

func (b *Buffer) clampRow() {
	if b.Row < 0 {
		b.Row = 0
	}
	if b.Row >= len(b.Lines) {
		b.Row = len(b.Lines) - 1
	}
}

func (b *Buffer) InsertRune(r rune) {
	line := b.curLine()
	out := make([]rune, 0, len(line)+1)
	out = append(out, line[:b.Col]...)
	out = append(out, r)
	out = append(out, line[b.Col:]...)
	b.Lines[b.Row] = string(out)
	b.Col++
	b.Dirty = true
}

func (b *Buffer) InsertTab() {
	for _, r := range IndentUnit {
		b.InsertRune(r)
	}
}

// leadingWhitespace returns the indentation prefix of a line.
func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

func (b *Buffer) InsertNewline() {
	line := b.curLine()
	before := string(line[:b.Col])
	after := string(line[b.Col:])

	indent := leadingWhitespace(before)
	trimmedBefore := strings.TrimRight(before, " \t")
	if strings.HasSuffix(trimmedBefore, "{") {
		indent += IndentUnit
	}

	b.Lines[b.Row] = before
	newLine := indent + after
	tail := append([]string{newLine}, b.Lines[b.Row+1:]...)
	b.Lines = append(b.Lines[:b.Row+1], tail...)
	b.Row++
	b.Col = len([]rune(indent))
	b.Dirty = true
}

func (b *Buffer) Backspace() {
	if b.Col > 0 {
		line := b.curLine()
		out := make([]rune, 0, len(line)-1)
		out = append(out, line[:b.Col-1]...)
		out = append(out, line[b.Col:]...)
		b.Lines[b.Row] = string(out)
		b.Col--
		b.Dirty = true
		return
	}
	if b.Row > 0 {
		prevLen := len([]rune(b.Lines[b.Row-1]))
		b.Lines[b.Row-1] += b.Lines[b.Row]
		b.Lines = append(b.Lines[:b.Row], b.Lines[b.Row+1:]...)
		b.Row--
		b.Col = prevLen
		b.Dirty = true
	}
}

func (b *Buffer) DeleteForward() {
	line := b.curLine()
	if b.Col < len(line) {
		out := make([]rune, 0, len(line)-1)
		out = append(out, line[:b.Col]...)
		out = append(out, line[b.Col+1:]...)
		b.Lines[b.Row] = string(out)
		b.Dirty = true
		return
	}
	if b.Row < len(b.Lines)-1 {
		b.Lines[b.Row] += b.Lines[b.Row+1]
		b.Lines = append(b.Lines[:b.Row+1], b.Lines[b.Row+2:]...)
		b.Dirty = true
	}
}

func (b *Buffer) MoveLeft() {
	if b.Col > 0 {
		b.Col--
	} else if b.Row > 0 {
		b.Row--
		b.Col = len([]rune(b.Lines[b.Row]))
	}
}

func (b *Buffer) MoveRight() {
	line := b.curLine()
	if b.Col < len(line) {
		b.Col++
	} else if b.Row < len(b.Lines)-1 {
		b.Row++
		b.Col = 0
	}
}

func (b *Buffer) MoveUp() {
	if b.Row > 0 {
		b.Row--
		b.clampCol()
	}
}

func (b *Buffer) MoveDown() {
	if b.Row < len(b.Lines)-1 {
		b.Row++
		b.clampCol()
	}
}

func (b *Buffer) MoveHome() { b.Col = 0 }
func (b *Buffer) MoveEnd()  { b.Col = len([]rune(b.Lines[b.Row])) }

func (b *Buffer) MoveDocStart() { b.Row, b.Col = 0, 0 }
func (b *Buffer) MoveDocEnd() {
	b.Row = len(b.Lines) - 1
	b.Col = len([]rune(b.Lines[b.Row]))
}

func (b *Buffer) PageUp(n int) {
	b.Row -= n
	b.clampRow()
	b.clampCol()
}

func (b *Buffer) PageDown(n int) {
	b.Row += n
	b.clampRow()
	b.clampCol()
}

// GotoLine moves the cursor to the start of line i (0-indexed), clamped.
func (b *Buffer) GotoLine(i int) {
	b.Row = i
	b.clampRow()
	b.Col = 0
}
