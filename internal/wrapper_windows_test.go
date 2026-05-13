//go:build windows

package internal

import (
	"strings"
	"testing"
)

func TestGenerateWrapperWindowsBasic(t *testing.T) {
	p := &Profile{
		Description: "Personal account",
	}
	files := GenerateWrapper("personal", `C:\Users\u\.claude-profiles\personal`, p)

	if files.Main != "claude-personal.cmd" {
		t.Errorf("Main = %q, want claude-personal.cmd", files.Main)
	}
	if files.Sidecar != "claude-personal.ps1" {
		t.Errorf("Sidecar = %q, want claude-personal.ps1", files.Sidecar)
	}

	if !strings.Contains(files.MainContent, "pwsh") {
		t.Errorf("Main content should invoke pwsh; got:\n%s", files.MainContent)
	}
	if !strings.Contains(files.MainContent, "claude-personal.ps1") {
		t.Errorf("Main content should reference sidecar by name; got:\n%s", files.MainContent)
	}
	if !strings.Contains(files.MainContent, "%~dp0") {
		t.Errorf("Main content should use %%~dp0 for relative resolution; got:\n%s", files.MainContent)
	}

	ps1 := files.SidecarContent
	if !strings.Contains(ps1, marker) {
		t.Errorf("Sidecar missing marker; got:\n%s", ps1)
	}
	if !strings.Contains(ps1, "CLAUDE_CONFIG_DIR") {
		t.Errorf("Sidecar missing CLAUDE_CONFIG_DIR; got:\n%s", ps1)
	}
	if !strings.Contains(ps1, "CLAUDE_PROFILE") {
		t.Errorf("Sidecar missing CLAUDE_PROFILE; got:\n%s", ps1)
	}
	if !strings.Contains(ps1, "Get-ChildItem env:") {
		t.Errorf("Sidecar missing env-clearing block; got:\n%s", ps1)
	}
	if !strings.Contains(ps1, "& claude @cmdArgs") {
		t.Errorf("Sidecar missing claude invocation; got:\n%s", ps1)
	}
}

func TestGenerateWrapperWindowsWithEnvAndModel(t *testing.T) {
	p := &Profile{
		Description: "Vertex",
		Model:       "sonnet",
		Env: map[string]string{
			"CLAUDE_CODE_USE_VERTEX":      "1",
			"ANTHROPIC_VERTEX_PROJECT_ID": "my-proj",
		},
	}
	files := GenerateWrapper("vertex", `C:\path\vertex`, p)
	ps1 := files.SidecarContent

	if !strings.Contains(ps1, `$env:CLAUDE_CODE_USE_VERTEX = '1'`) {
		t.Errorf("expected $env:CLAUDE_CODE_USE_VERTEX = '1' in:\n%s", ps1)
	}
	if !strings.Contains(ps1, `$env:ANTHROPIC_VERTEX_PROJECT_ID = 'my-proj'`) {
		t.Errorf("expected $env:ANTHROPIC_VERTEX_PROJECT_ID = 'my-proj' in:\n%s", ps1)
	}
	if !strings.Contains(ps1, "--model") {
		t.Errorf("expected --model handling for non-empty Model; got:\n%s", ps1)
	}
	if !strings.Contains(ps1, "'sonnet'") {
		t.Errorf("expected model name 'sonnet' single-quoted; got:\n%s", ps1)
	}
}

func TestGenerateWrapperWindowsEscapesSingleQuotes(t *testing.T) {
	p := &Profile{Description: "x"}
	files := GenerateWrapper("p", `C:\path\with's\quote`, p)
	ps1 := files.SidecarContent
	if !strings.Contains(ps1, `'C:\path\with''s\quote'`) {
		t.Errorf("expected doubled single quote in path; got:\n%s", ps1)
	}
}

func TestGenerateWrapperWindowsSubcommandBypass(t *testing.T) {
	files := GenerateWrapper("p", `C:\p`, &Profile{})
	ps1 := files.SidecarContent
	for _, sub := range []string{"mcp", "auth", "doctor", "install", "plugin"} {
		if !strings.Contains(ps1, "'"+sub+"'") {
			t.Errorf("subcommand %q not present in bypass list; got:\n%s", sub, ps1)
		}
	}
}
