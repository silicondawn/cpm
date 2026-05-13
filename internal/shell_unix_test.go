//go:build !windows

package internal

import (
	"strings"
	"testing"
)

func TestGenerateUseOutput(t *testing.T) {
	profile := &Profile{
		Description: "Test",
		Env: map[string]string{
			"CUSTOM_VAR": "value",
		},
	}

	out := GenerateUseOutput("test", "/profiles/test", profile)

	mustContain := []string{
		`CLAUDE_CONFIG_DIR="/profiles/test"`,
		`CLAUDE_PROFILE="test"`,
		`CUSTOM_VAR="value"`,
		"Switched to profile: test",
	}

	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("use output missing %q", s)
		}
	}
}

func TestGenerateShellHook(t *testing.T) {
	hook := GenerateShellHook()

	mustContain := []string{
		"_cpm_auto_switch",
		".claude-profile",
		"cpm use",
		"chpwd",
		"ZSH_VERSION",
	}

	for _, s := range mustContain {
		if !strings.Contains(hook, s) {
			t.Errorf("hook missing %q", s)
		}
	}
}
