//go:build windows

package internal

import "fmt"

func GenerateDirenvSnippet(name, profileDir string) string {
	return fmt.Sprintf(`# Claude Code profile: %s
# Add this to your .envrc.ps1 or PowerShell-aware envrc handler
$env:CLAUDE_CONFIG_DIR = %s
$env:CLAUDE_PROFILE = %s
`, name, psQuote(profileDir), psQuote(name))
}
