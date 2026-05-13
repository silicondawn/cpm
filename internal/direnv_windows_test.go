//go:build windows

package internal

import (
	"strings"
	"testing"
)

func TestGenerateDirenvSnippetWindows(t *testing.T) {
	out := GenerateDirenvSnippet("work", `C:\Users\u\.claude-profiles\work`)

	mustContain := []string{
		`$env:CLAUDE_CONFIG_DIR = 'C:\Users\u\.claude-profiles\work'`,
		`$env:CLAUDE_PROFILE = 'work'`,
	}

	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("direnv snippet missing %q; got:\n%s", s, out)
		}
	}
}
