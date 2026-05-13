//go:build windows

package internal

import (
	"strings"
	"testing"
)

func TestGenerateUseOutputWindowsEnvFormat(t *testing.T) {
	p := &Profile{
		Env: map[string]string{"FOO": "bar"},
	}
	s := GenerateUseOutput("work", `C:\path`, p)

	for _, need := range []string{
		`$env:CLAUDE_CONFIG_DIR = 'C:\path'`,
		`$env:CLAUDE_PROFILE = 'work'`,
		`$env:FOO = 'bar'`,
		"Get-ChildItem env:",
		`Switched to profile: work`,
	} {
		if !strings.Contains(s, need) {
			t.Errorf("use output missing %q; got:\n%s", need, s)
		}
	}
}

func TestGenerateUseOutputWindowsEscapesQuote(t *testing.T) {
	p := &Profile{}
	s := GenerateUseOutput("p", `C:\it's\path`, p)
	if !strings.Contains(s, `'C:\it''s\path'`) {
		t.Errorf("expected doubled single quote in path; got:\n%s", s)
	}
}
