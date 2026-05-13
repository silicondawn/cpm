//go:build !windows

package internal

import (
	"fmt"
	"strings"
)

// GenerateUseOutput outputs shell commands to be eval'd by the user's shell.
// Usage: eval "$(cpm use <profile>)"
func GenerateUseOutput(name string, profileDir string, profile *Profile) string {
	var b strings.Builder

	b.WriteString("unset $(env | grep -E '^(CLAUDE_|ANTHROPIC_)' | cut -d= -f1) 2>/dev/null;\n")
	b.WriteString(fmt.Sprintf("export CLAUDE_CONFIG_DIR=\"%s\";\n", profileDir))
	b.WriteString(fmt.Sprintf("export CLAUDE_PROFILE=\"%s\";\n", name))

	for k, v := range profile.Env {
		b.WriteString(fmt.Sprintf("export %s=\"%s\";\n", k, v))
	}

	b.WriteString(fmt.Sprintf("echo \"Switched to profile: %s\";\n", name))
	return b.String()
}
