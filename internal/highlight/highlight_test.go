package highlight

import (
	"os"
	"strings"
	"testing"
)

func TestTokenizeRealCaddyfileNoPanic(t *testing.T) {
	content, err := os.ReadFile("../testdata/Caddyfile")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		toks := Tokenize(line)
		joined := ""
		for _, tk := range toks {
			joined += tk.Text
		}
		if joined != line {
			t.Fatalf("tokens don't reconstruct line: got %q want %q", joined, line)
		}
	}
}

func TestKindsSpotCheck(t *testing.T) {
	cases := []struct {
		line string
		want Kind
		text string
	}{
		{`# a comment`, Comment, "# a comment"},
		{`reverse_proxy http://x:1`, Keyword, "reverse_proxy"},
		{`@nzb360 header X-NZB360-Key "{env.SSO_BYPASS_KEY}"`, Matcher, "@nzb360"},
		{`reverse_proxy {args[0]}`, Placeholder, "{args[0]}"},
		{`(ssoauth) {`, SnippetName, "(ssoauth)"},
	}
	for _, c := range cases {
		toks := Tokenize(c.line)
		found := false
		for _, tk := range toks {
			if tk.Kind == c.want && tk.Text == c.text {
				found = true
			}
		}
		if !found {
			t.Errorf("line %q: expected token %q of kind %v, tokens=%#v", c.line, c.text, c.want, toks)
		}
	}
}
