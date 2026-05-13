//go:build windows

package internal

import (
	"fmt"
	"sort"
	"strings"
)

func GenerateWrapper(name string, profileDir string, profile *Profile) WrapperFiles {
	cmdContent := generateWindowsCmdShim(name)
	ps1Content := generateWindowsPS1(name, profileDir, profile)

	return WrapperFiles{
		Main:           "claude-" + name + ".cmd",
		MainContent:    cmdContent,
		Sidecar:        "claude-" + name + ".ps1",
		SidecarContent: ps1Content,
		Mode:           0o644, // Windows ignores mode; just keep something sensible
	}
}

func generateWindowsCmdShim(name string) string {
	var b strings.Builder
	b.WriteString(":: " + marker + "\n")
	b.WriteString(fmt.Sprintf(":: Profile: %s\n", name))
	b.WriteString(fmt.Sprintf("@pwsh -NoProfile -ExecutionPolicy Bypass -File \"%%~dp0claude-%s.ps1\" %%*\n", name))
	return b.String()
}

// psQuote single-quotes a string for embedding in PowerShell, doubling embedded single quotes.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func generateWindowsPS1(name, profileDir string, profile *Profile) string {
	var b strings.Builder

	b.WriteString("# " + marker + "\n")
	b.WriteString(fmt.Sprintf("# Profile: %s — %s\n\n", name, profile.Description))

	b.WriteString("# Unset inherited CLAUDE_*/ANTHROPIC_* env vars\n")
	b.WriteString("Get-ChildItem env: |\n")
	b.WriteString("  Where-Object { $_.Name -match '^(CLAUDE_|ANTHROPIC_)' } |\n")
	b.WriteString("  ForEach-Object { Remove-Item \"env:$($_.Name)\" }\n\n")

	b.WriteString(fmt.Sprintf("$env:CLAUDE_CONFIG_DIR = %s\n", psQuote(profileDir)))
	b.WriteString(fmt.Sprintf("$env:CLAUDE_PROFILE    = %s\n", psQuote(name)))

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

	b.WriteString("\n")

	b.WriteString("# Subcommand bypass — pass through to claude directly\n")
	b.WriteString("$bypassCmds = @(")
	var quoted []string
	for _, s := range strings.Split(subcommands, "|") {
		quoted = append(quoted, "'"+s+"'")
	}
	b.WriteString(strings.Join(quoted, ","))
	b.WriteString(")\n")
	b.WriteString("if ($args.Count -gt 0 -and $bypassCmds -contains $args[0]) {\n")
	b.WriteString("  & claude @args\n")
	b.WriteString("  exit $LASTEXITCODE\n")
	b.WriteString("}\n\n")

	b.WriteString("$cmdArgs = @()\n")
	for _, d := range profile.AddDirs {
		expanded := ExpandPath(d)
		b.WriteString(fmt.Sprintf("$cmdArgs += @('--add-dir', %s)\n", psQuote(expanded)))
	}

	if profile.Model != "" {
		b.WriteString("\n# Default model (overridden if --model is passed on command line)\n")
		b.WriteString("$hasModel = $false\n")
		b.WriteString("foreach ($a in $args) {\n")
		b.WriteString("  if ($a -eq '--model' -or $a.StartsWith('--model=')) { $hasModel = $true; break }\n")
		b.WriteString("}\n")
		b.WriteString(fmt.Sprintf("if (-not $hasModel) { $cmdArgs += @('--model', %s) }\n", psQuote(profile.Model)))
	}

	b.WriteString("\n$cmdArgs += $args\n\n")
	b.WriteString("& claude @cmdArgs\n")
	b.WriteString("exit $LASTEXITCODE\n")

	return b.String()
}
