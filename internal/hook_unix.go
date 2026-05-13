//go:build !windows

package internal

func GenerateShellHook() string {
	return `# cpm auto-switch hook — add to your .zshrc or .bashrc:
#   eval "$(cpm hook)"
_cpm_auto_switch() {
  local profile_file=""
  local dir="$PWD"
  while [ "$dir" != "/" ]; do
    if [ -f "$dir/.claude-profile" ]; then
      profile_file="$dir/.claude-profile"
      break
    fi
    dir="$(dirname "$dir")"
  done
  if [ -n "$profile_file" ]; then
    local target
    target="$(cat "$profile_file" | tr -d '[:space:]')"
    if [ -n "$target" ] && [ "$target" != "${CLAUDE_PROFILE:-}" ]; then
      eval "$(cpm use "$target" 2>/dev/null)"
      echo "[cpm] using profile: $target"
    fi
  elif [ -n "${CLAUDE_PROFILE:-}" ]; then
    unset CLAUDE_CONFIG_DIR CLAUDE_PROFILE
    unset $(env | grep -E '^(CLAUDE_|ANTHROPIC_)' | cut -d= -f1) 2>/dev/null
    echo "[cpm] profile unset (no .claude-profile found)"
  fi
}
if [ -n "$ZSH_VERSION" ]; then
  autoload -Uz add-zsh-hook
  add-zsh-hook chpwd _cpm_auto_switch
else
  _cpm_original_cd() { builtin cd "$@" && _cpm_auto_switch; }
  alias cd='_cpm_original_cd'
fi
_cpm_auto_switch
`
}
