<p align="center">
  <img src="https://github.com/silicondawn/cpm/raw/main/assets/logo.svg" width="140" alt="cpm logo" />
</p>

<h1 align="center">cpm</h1>

<p align="center">
  <strong>Claude Profile Manager</strong> — run multiple Claude Code accounts side-by-side<br/>
  with isolated credentials, shared config, and zero overhead.
</p>

<p align="center">
  <a href="https://github.com/silicondawn/cpm/actions/workflows/test.yml"><img src="https://github.com/silicondawn/cpm/actions/workflows/test.yml/badge.svg" alt="Tests" /></a>
  <a href="https://github.com/silicondawn/cpm/releases/latest"><img src="https://img.shields.io/github/v/release/silicondawn/cpm?label=version" alt="Latest Release" /></a>
  <a href="https://github.com/silicondawn/cpm/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License" /></a>
</p>

<p align="center">
  <img src="https://github.com/silicondawn/cpm/raw/main/assets/demo.svg" width="700" alt="cpm demo" />
</p>

---

## Why?

You have a personal Claude subscription and a company one. Or a Vertex AI setup. Or three clients. Every time you switch, you re-login, lose context, or mix credentials.

**cpm** gives each account its own `claude-<name>` command. Run them in parallel, auto-switch per project, never re-login.

```
claude-personal     # Personal Anthropic subscription
claude-work         # Company team account
claude-vertex       # Company via Google Cloud Vertex AI
```

## How it works

```mermaid
flowchart LR
    A["config.toml"] --> B["cpm install"]
    B --> C["~/.claude-profiles/personal/"]
    B --> D["~/.claude-profiles/work/"]
    B --> E["bin_dir/claude-personal"]
    B --> F["bin_dir/claude-work"]
    C --> G["claude-personal"]
    D --> H["claude-work"]

    style A fill:#7c3aed,color:#fff
    style B fill:#7c3aed,color:#fff
    style G fill:#059669,color:#fff
    style H fill:#059669,color:#fff
```

Each profile gets an isolated `CLAUDE_CONFIG_DIR` with its own credentials, while sharing commands, skills, plugins, and projects via symlinks (Unix) or junctions (Windows):

```
~/.claude-profiles/
├── config.toml
├── personal/
│   ├── settings.json        # Copied (mutable)
│   ├── CLAUDE.md            # Copied (mutable)
│   ├── commands/ -> ~/.claude/commands/   # Linked
│   ├── skills/   -> ~/.claude/skills/     # Linked
│   ├── plugins/  -> ~/.claude/plugins/    # Linked
│   ├── projects/ -> ~/.claude/projects/   # Linked
│   ├── .credentials.json    # Per-profile (created by Claude)
│   └── .claude.json         # Per-profile (seeded by cpm)
└── work/
    └── ...
```

## Install

### Windows (Scoop)

```powershell
scoop bucket add cpm https://github.com/silicondawn/cpm
scoop install cpm
```

Requires **PowerShell 7+** (`pwsh`). Install via `winget install Microsoft.PowerShell` if missing. Profile-shared directories use junctions, so no admin / Developer Mode is required.

### macOS / Linux (GitHub Releases)

Download the latest binary archive from [Releases](https://github.com/silicondawn/cpm/releases/latest):

```bash
# macOS (Intel / Apple Silicon via Rosetta)
curl -L https://github.com/silicondawn/cpm/releases/latest/download/cpm_darwin_amd64.tar.gz \
  | tar -xz -C ~/.local/bin/ cpm

# Linux x86_64
curl -L https://github.com/silicondawn/cpm/releases/latest/download/cpm_linux_amd64.tar.gz \
  | tar -xz -C ~/.local/bin/ cpm

chmod +x ~/.local/bin/cpm
```

### From source

```bash
git clone https://github.com/silicondawn/cpm.git
cd cpm
go install .
```

## Quick start

```bash
# 1. Create config interactively
cpm init

# 2. Install profiles + wrapper scripts
cpm install

# 3. Authenticate each profile (first time only — OAuth in browser)
claude-personal
claude-work
```

**Windows note**: after `cpm install`, run any `claude-<name>` from a new PowerShell session. The wrapper handles env setup automatically. Add the auto-switch hook to `$PROFILE`:

```powershell
Invoke-Expression (& cpm hook | Out-String)
```

## Per-project profiles (like `.nvmrc`)

```mermaid
flowchart LR
    A["cd ~/Work/project"] --> B["shell hook"]
    B --> C{".claude-profile?"}
    C -->|"found: work"| D["auto-switch to work"]
    C -->|"not found"| E["unset profile"]

    style A fill:#7c3aed,color:#fff
    style D fill:#059669,color:#fff
```

Link a profile to any project directory:

```bash
# Set profile for this project
cd ~/Work/company-project
cpm link work

# Auto-switch on cd (add to .zshrc / .bashrc once)
eval "$(cpm hook)"

# Or on Windows ($PROFILE):
Invoke-Expression (& cpm hook | Out-String)

# Now every time you cd into this project, the profile switches.
```

The `.claude-profile` file is automatically added to `.gitignore`.

## Commands

| Command | Description |
|---------|-------------|
| `cpm install` | Create profile directories and wrapper scripts |
| `cpm install --sync` | Re-sync mutable files from `~/.claude` |
| `cpm install --sync --force` | Force overwrite diverged files |
| `cpm list` | List all profiles with status |
| `cpm which` | Show active profile (from env or `.claude-profile`) |
| `cpm status` | Check sync divergence |
| `cpm doctor` | Diagnose issues (broken symlinks, expired creds, ...) |
| `cpm credentials` | Show OAuth token status for all profiles |
| `cpm use <profile>` | Switch shell — Unix: `eval "$(cpm use work)"`; PowerShell: `Invoke-Expression (& cpm use work \| Out-String)` |
| `cpm run <profile> [args]` | One-shot: `cpm run work -p "explain this"` |
| `cpm link <profile>` | Create `.claude-profile` in current dir |
| `cpm unlink` | Remove `.claude-profile` |
| `cpm hook` | Print shell hook for auto-switch |
| `cpm direnv <profile>` | Print `.envrc` snippet |
| `cpm clone <src> <dst>` | Clone profile (without credentials) |
| `cpm init` | Interactive config wizard |
| `cpm version` | Show version + check for updates |
| `cpm upgrade` | Self-update from GitHub Releases |
| `cpm cloud init [--remote <url>]` | Initialize cloud sync repo |
| `cpm cloud push [-m "msg"]` | Push local settings to cloud |
| `cpm cloud pull [--dry-run]` | Pull settings from cloud |
| `cpm cloud status` | Show cloud sync status |
| `cpm cloud remote <url>` | Set/update git remote URL |

## Cloud sync

Sync your Claude Code settings (plugins, skills, commands, `settings.json`) across machines via a private git repository.

```bash
# On your first machine — initialize and push
cpm cloud init --remote git@github.com:you/claude-settings.git
cpm cloud push

# On another machine — clone and pull
cpm cloud init --remote git@github.com:you/claude-settings.git
# Files are automatically distributed on clone

# Later — sync changes
cpm cloud push   # from the machine where you changed settings
cpm cloud pull   # on the other machine
```

### What gets synced

| Synced | Not synced |
|--------|------------|
| `settings.json`, `settings.local.json` | Credentials (`.credentials.json`) |
| `CLAUDE.md` | Sessions, caches |
| `commands/`, `agents/` | `projects/` (cache, ~2 GB) |
| `plugins/installed_plugins.json` | Telemetry |
| `plugins/known_marketplaces.json` | |
| `.skill-lock.json` | |
| cpm `config.toml` | |

### Exclude files from sync

```toml
[cloud]
remote = "git@github.com:you/claude-settings.git"
exclude = ["CLAUDE.md", "commands/"]
```

## Configuration

### `~/.claude-profiles/config.toml`

```toml
source_dir = "~/.claude"
# bin_dir default per platform:
#   Unix:    ~/.local/bin
#   Windows: ~/scoop/shims  (if cpm was installed via scoop)
#            %LOCALAPPDATA%\cpm\bin (otherwise)
# bin_dir = "/custom/path"

[profiles.personal]
description = "Personal Anthropic account"

[profiles.work]
description = "Company team subscription"
model = "sonnet"
add_dirs = ["~/Work/company"]

[profiles.work.attribution]
commit = "Co-Authored-By: Claude <noreply@anthropic.com>"
pr = "Generated with [Claude Code](https://claude.ai/code)"

[profiles.vertex]
description = "Company via Vertex AI"
add_dirs = ["~/Work/company"]

[profiles.vertex.env]
CLAUDE_CODE_USE_VERTEX = "1"
ANTHROPIC_VERTEX_PROJECT_ID = "your-project-id"
CLOUD_ML_REGION = "europe-west1"
```

### Config reference

| Field | Default | Description |
|-------|---------|-------------|
| `source_dir` | `~/.claude` | Source for shared config |
| `bin_dir` | platform-aware (see above) | Where wrapper scripts are installed |
| `profiles.<name>.description` | | Human-readable description |
| `profiles.<name>.model` | | Default model (`sonnet`, `opus`) |
| `profiles.<name>.add_dirs` | | Extra dirs passed via `--add-dir` |
| `profiles.<name>.env` | | Environment variables |
| `profiles.<name>.attribution.commit` | | Git commit attribution text |
| `profiles.<name>.attribution.pr` | | PR description attribution text |
| `cloud.remote` | | Git remote URL for cloud sync |
| `cloud.auto_push` | `false` | Auto-push on `cpm install` |
| `cloud.exclude` | `[]` | Files/dirs to exclude from sync |

### Shared file handling

| Type | Files | Behavior |
|------|-------|----------|
| **Copied** | `settings.json`, `settings.local.json`, `CLAUDE.md` | Copied on first install. `--sync` to refresh. |
| **Linked** | `commands/`, `skills/`, `agents/`, `plugins/`, `projects/` | Shared across all profiles via symlink (Unix) or junction (Windows) |
| **Seeded** | `.claude.json` | Onboarding flags + MCP servers copied from source so the profile skips first-run wizard |
| **Per-profile** | `.credentials.json`, `teams/` | Created by Claude on first OAuth |

## Shell integration

### Prompt (PS1 / Starship)

Show active profile in your terminal prompt:

```bash
# .zshrc — simple
PROMPT='$(cpm prompt)> '

# Starship — custom command
[custom.claude]
command = "cpm prompt"
when = "test -n \"$CLAUDE_PROFILE\""
format = "[$output]($style) "
style = "purple"
```

PowerShell prompt — see `cpm hook` output, which preserves your existing `prompt` function.

### direnv

```bash
# Generate .envrc for a project (Unix)
cpm direnv work >> ~/Work/company-project/.envrc
direnv allow ~/Work/company-project
```

On Windows `cpm direnv` emits a `$env:` snippet you can `Invoke-Expression` from your `.envrc.ps1` or PowerShell-aware loader.

## Upgrading

```bash
# Self-update (downloads matching release binary)
cpm upgrade

# Via Scoop (Windows)
scoop update cpm
```

## Platform support

- macOS, Linux, or Windows 10/11 (PowerShell 7+ for the Windows version — Windows PowerShell 5.1 is not supported)
- Claude Code installed and on PATH
- `bin_dir` on PATH (the default already is on Windows when installed via scoop; on Unix you may need to add `~/.local/bin`)
- Releases ship `amd64` only. Apple Silicon users can run the `darwin_amd64` binary under Rosetta.

## License

[MIT](LICENSE)
