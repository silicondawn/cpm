//go:build !windows

package internal

import (
	"fmt"
	"sort"
	"strings"
)

func GenerateWrapper(name string, profileDir string, profile *Profile) WrapperFiles {
	var b strings.Builder

	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString(marker + "\n")
	b.WriteString(fmt.Sprintf("# Profile: %s — %s\n", name, profile.Description))
	b.WriteString("set -euo pipefail\n\n")

	b.WriteString("# Unset inherited CLAUDE_*/ANTHROPIC_* env vars to avoid interference\n")
	b.WriteString("while IFS= read -r varname; do\n")
	b.WriteString("  unset \"$varname\"\n")
	b.WriteString("done < <(compgen -v | grep -E \"^(CLAUDE_|ANTHROPIC_)\")\n\n")

	b.WriteString(fmt.Sprintf("export CLAUDE_CONFIG_DIR=\"%s\"\n", profileDir))
	b.WriteString(fmt.Sprintf("export CLAUDE_PROFILE=\"%s\"\n", name))

	if len(profile.Env) > 0 {
		keys := make([]string, 0, len(profile.Env))
		for k := range profile.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("export %s=\"%s\"\n", k, profile.Env[k]))
		}
	}

	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("case \"${1:-}\" in\n  %s) exec claude \"$@\" ;;\nesac\n\n", subcommands))

	if profile.Model != "" {
		b.WriteString("# Default model (overridden if --model is passed on command line)\n")
		b.WriteString("has_model=false\n")
		b.WriteString("for arg in \"$@\"; do\n")
		b.WriteString("  case \"$arg\" in\n")
		b.WriteString("    --model|--model=*) has_model=true; break ;;\n")
		b.WriteString("  esac\n")
		b.WriteString("done\n\n")
	}

	var cmdParts []string
	cmdParts = append(cmdParts, "exec claude")
	for _, d := range profile.AddDirs {
		expanded := ExpandPath(d)
		cmdParts = append(cmdParts, fmt.Sprintf("--add-dir \"%s\"", expanded))
	}
	if profile.Model != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("$([ \"$has_model\" = false ] && echo '--model %s')", profile.Model))
	}
	cmdParts = append(cmdParts, "\"$@\"")
	b.WriteString(strings.Join(cmdParts, " \\\n  ") + "\n")

	return WrapperFiles{
		Main:        "claude-" + name,
		MainContent: b.String(),
		Mode:        0o755,
	}
}
