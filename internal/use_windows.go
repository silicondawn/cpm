//go:build windows

package internal

import (
	"fmt"
	"sort"
	"strings"
)

func GenerateUseOutput(name string, profileDir string, profile *Profile) string {
	var b strings.Builder

	b.WriteString("Get-ChildItem env: | Where-Object { $_.Name -match '^(CLAUDE_|ANTHROPIC_)' } | ForEach-Object { Remove-Item \"env:$($_.Name)\" }\n")
	b.WriteString(fmt.Sprintf("$env:CLAUDE_CONFIG_DIR = %s\n", psQuote(profileDir)))
	b.WriteString(fmt.Sprintf("$env:CLAUDE_PROFILE = %s\n", psQuote(name)))

	if len(profile.Env) > 0 {
		keys := make([]string, 0, len(profile.Env))
		for k := range profile.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("$env:%s = %s\n", k, psQuote(profile.Env[k])))
		}
	}

	b.WriteString(fmt.Sprintf("Write-Host \"Switched to profile: %s\"\n", name))
	return b.String()
}
