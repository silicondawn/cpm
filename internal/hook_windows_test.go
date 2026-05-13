//go:build windows

package internal

import (
	"strings"
	"testing"
)

func TestGenerateShellHookContainsPromptFunction(t *testing.T) {
	h := GenerateShellHook()

	for _, need := range []string{
		"function global:prompt",
		"$script:__cpm_last_dir",
		"$script:__cpm_orig_prompt",
		".claude-profile",
		"cpm use $target",
		"Invoke-Expression",
		"Get-ChildItem env:",
	} {
		if !strings.Contains(h, need) {
			t.Errorf("hook missing %q; got:\n%s", need, h)
		}
	}
}

func TestGenerateShellHookCallsOriginalPrompt(t *testing.T) {
	h := GenerateShellHook()
	if !strings.Contains(h, "& $script:__cpm_orig_prompt") {
		t.Errorf("hook must invoke original prompt at end; got:\n%s", h)
	}
}
