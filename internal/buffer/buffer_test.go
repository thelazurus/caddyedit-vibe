package buffer

import (
	"os"
	"testing"
)

func TestRoundTripRealCaddyfile(t *testing.T) {
	content, err := os.ReadFile("../testdata/Caddyfile")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	b := New(string(content))
	if got := b.String(); got != string(content) {
		t.Fatalf("round trip mismatch: len(got)=%d len(want)=%d", len(got), len(content))
	}
}

func TestEditingOps(t *testing.T) {
	b := New("hello world\nsecond line\n")

	b.Row, b.Col = 0, 5
	b.InsertRune(',')
	if b.Lines[0] != "hello, world" {
		t.Fatalf("InsertRune: got %q", b.Lines[0])
	}

	b.MoveEnd()
	b.InsertNewline()
	if len(b.Lines) != 3 || b.Lines[1] != "" {
		t.Fatalf("InsertNewline: got %#v", b.Lines)
	}

	b.Backspace()
	if len(b.Lines) != 2 || b.Lines[0] != "hello, world" {
		t.Fatalf("Backspace join: got %#v", b.Lines)
	}

	b.Row, b.Col = 0, 0
	b.DeleteForward()
	if b.Lines[0] != "ello, world" {
		t.Fatalf("DeleteForward: got %q", b.Lines[0])
	}

	// Auto-indent after a line ending in '{'.
	b2 := New("foo {\n}\n")
	b2.Row, b2.Col = 0, 5
	b2.InsertNewline()
	if b2.Lines[1] != IndentUnit {
		t.Fatalf("auto-indent: got %q want %q", b2.Lines[1], IndentUnit)
	}
}
