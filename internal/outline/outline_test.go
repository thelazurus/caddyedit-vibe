package outline

import (
	"os"
	"strings"
	"testing"
)

func TestParseRealCaddyfile(t *testing.T) {
	content, err := os.ReadFile("../testdata/Caddyfile")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	lines := strings.Split(string(content), "\n")
	blocks := Parse(lines)

	if len(blocks) < 25 {
		t.Fatalf("expected at least 25 top-level blocks, got %d", len(blocks))
	}

	var snippets, sites int
	for _, b := range blocks {
		if b.IsSnippet {
			snippets++
		} else {
			sites++
		}
		if b.EndLine <= b.StartLine {
			t.Errorf("block %q has EndLine %d <= StartLine %d", b.Header, b.EndLine, b.StartLine)
		}
	}
	if snippets != 5 {
		t.Errorf("expected 5 snippet defs (ssoauth, sso-core, sso-core-basic, sso-bypass, sso-bypass-basic), got %d", snippets)
	}

	// Spot check a specific block and its leading comment.
	found := false
	for _, b := range blocks {
		if strings.HasPrefix(b.Header, "sonarr.lazurus.me") {
			found = true
			if b.Comment != "Sonarr" {
				t.Errorf("expected comment %q, got %q", "Sonarr", b.Comment)
			}
		}
	}
	if !found {
		t.Error("did not find sonarr.lazurus.me block")
	}
	_ = sites
}
