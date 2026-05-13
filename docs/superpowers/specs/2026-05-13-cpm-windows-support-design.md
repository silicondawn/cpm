# cpm Windows Support — Design

**Date:** 2026-05-13
**Status:** Draft, awaiting user review
**Branch:** `windows-support`
**Upstream:** `https://github.com/JakubKontra/claude-profile-manager` @ commit at fork time

---

## 1. Goals & Non-Goals

### Goals

1. `cpm` runs on Windows (PowerShell 7+) with feature parity to the existing macOS/Linux support — every command (`install`, `list`, `use`, `run`, `clone`, `doctor`, `hook`, `link`, `unlink`, `which`, `direnv`, `prompt`, `credentials`, `version`, `upgrade`, `init`, `cloud *`) works.
2. Single codebase, three targets — Windows / macOS / Linux — via Go build tags. The main code path does not branch on `runtime.GOOS`.
3. Additive only — no existing darwin/linux behavior is changed. This fork's diff against upstream should be a clean superset suitable for upstream PR.
4. Single-file distribution as `cpm.exe` via GitHub Releases. Keep existing Homebrew tap untouched. Add a Scoop bucket for Windows discoverability.
5. Profile-shared directories (`commands/`, `skills/`, `agents/`, `plugins/`, `projects/`) use **junction first, symlink fallback** so a normal user with no elevation can complete `cpm install`.

### Non-Goals

1. No Windows PowerShell 5.1 support — target only PowerShell 7+ (`pwsh`).
2. No cmd.exe interactive integration — generated `claude-<name>.cmd` shims are callable from cmd, but `cpm use` / `cpm hook` only emit PowerShell.
3. No WSL support — WSL users should use the Linux binary directly.
4. No reinvention of direnv — `cpm direnv` emits a `$env:` snippet; the user pipes it into their own `.envrc.ps1` mechanism.
5. No new Go dependencies on the Unix side. `BurntSushi/toml` + `spf13/cobra` remain the only direct deps for the cross-platform code. Windows-specific code may import `golang.org/x/sys/windows` only if a future iteration replaces the `mklink` shell-out.
6. No installer (MSI / Inno). GitHub Releases binary + Scoop bucket only.
7. No Windows-on-ARM64 in the first release. amd64 only.

---

## 2. Architecture — Build Tag Split

The package layout stays flat (single `internal/` package, no sub-packages, no interface abstractions). Platform-divergent functions get **two physical files** with build tags:

```
cpm/
├── main.go                              # exec call routed through execClaude() platform helper
├── go.mod                               # module path unchanged: github.com/jakubkontra/cpm
├── go.sum
├── .goreleaser.yaml                     # +windows target, +scoop publisher
├── .github/
│   └── workflows/
│       └── test.yml                     # matrix: ubuntu-latest, macos-latest, windows-latest
├── README.md                            # +section: Install on Windows
├── config.example.toml                  # unchanged
└── internal/
    ├── config.go                        # unchanged — TOML, ExpandPath (cross-platform via os.UserHomeDir)
    ├── profile.go                       # main flow unchanged; os.Symlink → linkShared()
    ├── clone.go                         # os.Symlink → linkShared()
    ├── doctor.go                        # main flow unchanged; uses resolveLinkTarget() + wrapper name helpers
    ├── sync.go                          # unchanged (JSON, cross-platform)
    ├── cloud.go                         # unchanged (git subprocess is cross-platform)
    ├── version.go                       # asset name suffix .exe on windows; replaceBinary() platform-split
    ├── init.go                          # interactive prompts; bin_dir default via defaultBinDir()
    ├── shell.go                         # Detect/Link/Unlink/CurrentProfile remain cross-platform
    │
    │   # Platform-divergent pairs:
    ├── exec_unix.go        // !windows   execClaude() via syscall.Exec
    ├── exec_windows.go     // windows    execClaude() via exec.Cmd.Run + propagate exit code
    ├── wrapper_unix.go     // !windows   GenerateWrapper(): bash script, no extension
    ├── wrapper_windows.go  // windows    GenerateWrapper(): writes .cmd + .ps1 pair, returns both
    ├── hook_unix.go        // !windows   GenerateShellHook(): zsh/bash auto-switch
    ├── hook_windows.go     // windows    GenerateShellHook(): PowerShell prompt-function wrapper
    ├── use_unix.go         // !windows   GenerateUseOutput(): export VAR=...
    ├── use_windows.go      // windows    GenerateUseOutput(): $env:VAR = '...'
    ├── direnv_unix.go      // !windows   GenerateDirenvSnippet(): export VAR=...
    ├── direnv_windows.go   // windows    GenerateDirenvSnippet(): $env:VAR = '...'
    ├── link_unix.go        // !windows   linkShared(): os.Symlink
    ├── link_windows.go     // windows    linkShared(): mklink /J → os.Symlink fallback
    ├── paths_unix.go       // !windows   defaultBinDir(): ~/.local/bin
    ├── paths_windows.go    // windows    defaultBinDir(): %LOCALAPPDATA%\cpm\bin
    ├── upgrade_unix.go     // !windows   replaceBinary(): os.Rename
    ├── upgrade_windows.go  // windows    replaceBinary(): rename current to .old, install new
    │
    │   # Tests follow the same pairing:
    ├── *_test.go                        # cross-platform tests (unchanged)
    ├── wrapper_unix_test.go             // !windows   asserts bash output
    ├── wrapper_windows_test.go          // windows    asserts .cmd + .ps1 output
    ├── hook_unix_test.go                // !windows
    ├── hook_windows_test.go             // windows
    ├── link_unix_test.go                // !windows
    └── link_windows_test.go             // windows    junction roundtrip, readlink resolves correctly
```

### Build tag style

Use Go 1.17+ syntax:
- `//go:build windows`
- `//go:build !windows`

The `_windows.go` suffix triggers the Go toolchain's filename-based build constraint, but we add the explicit `//go:build` directive too for clarity. The `_unix.go` suffix is **not** a Go built-in — its build tag is the source of truth.

### Platform abstraction boundary — function-level, not interface-level

Cross-platform callers reference these names with no awareness of platform:

| Function | Returns | Notes |
|---|---|---|
| `execClaude(claudePath string, args []string, env []string) error` | terminates process | Unix: never returns (`syscall.Exec`). Windows: spawns, propagates exit code, calls `os.Exit`. |
| `GenerateWrapper(name, profileDir string, profile *Profile) WrapperFiles` | struct with main path + optional sidecar | Unix: `{Main: "claude-<name>", Content: "..."}`. Windows: `{Main: "claude-<name>.cmd", Content: "..."}` + sidecar `.ps1`. |
| `InstallWrapper(binDir string, files WrapperFiles) error` | writes file(s) | Unix: 1 file, 0o755. Windows: 2 files. |
| `WrapperFilenames(profileName string) []string` | list of all files for this profile | Used by `CleanupStaleScripts` and `doctor`. |
| `GenerateShellHook() string` | hook code | Unix: bash/zsh. Windows: PowerShell. |
| `GenerateUseOutput(...)` | eval-able script | Unix: `export ...`. Windows: `$env:... = '...'`. |
| `GenerateDirenvSnippet(...)` | `.envrc` snippet | Same split. |
| `linkShared(src, dst string) error` | creates link | Unix: symlink. Windows: junction → symlink fallback. |
| `resolveLinkTarget(path string) (string, error)` | normalized target | Strips `\??\` prefix on Windows. |
| `defaultBinDir() string` | path | Unix: `~/.local/bin`. Windows: `%LOCALAPPDATA%\cpm\bin`. |
| `replaceBinary(tmpPath, targetPath string) error` | atomic-ish replace | Unix: rename. Windows: rename-old + rename-new. |

No interfaces. No struct methods. Compiler picks the implementation by file build tag.

---

## 3. Platform-Specific Implementation

### 3.1 Wrapper script generation (`wrapper_windows.go`)

Generates a **`.cmd` + `.ps1` pair** per profile. Both contain the cpm-generated marker for stale-script cleanup.

**`claude-<name>.cmd`** (thin shim, no logic):

```cmd
:: Generated by cpm (Claude Profile Manager)
:: Profile: <name>
@pwsh -NoProfile -ExecutionPolicy Bypass -File "%~dp0claude-<name>.ps1" %*
```

**`claude-<name>.ps1`** (real wrapper):

```powershell
# Generated by cpm (Claude Profile Manager)
# Profile: <name> — <description>

# Unset inherited CLAUDE_*/ANTHROPIC_* env vars
Get-ChildItem env: |
  Where-Object { $_.Name -match '^(CLAUDE_|ANTHROPIC_)' } |
  ForEach-Object { Remove-Item "env:$($_.Name)" }

$env:CLAUDE_CONFIG_DIR = '<profileDir>'
$env:CLAUDE_PROFILE    = '<name>'
# Profile env entries written here, one per line:
$env:<KEY> = '<VALUE>'

# Subcommand bypass — pass through to claude directly
$bypassCmds = @('mcp','auth','doctor','install','setup-token','update','upgrade','agents','auto-mode','plugin','plugins')
if ($args.Count -gt 0 -and $bypassCmds -contains $args[0]) {
  & claude @args
  exit $LASTEXITCODE
}

$cmdArgs = @()
# Per profile.AddDirs:
$cmdArgs += @('--add-dir', '<expanded-dir>')

# Per profile.Model (only if not already in $args):
$hasModel = $false
foreach ($a in $args) {
  if ($a -eq '--model' -or $a.StartsWith('--model=')) { $hasModel = $true; break }
}
if (-not $hasModel) {
  $cmdArgs += @('--model', '<model>')
}

$cmdArgs += $args

& claude @cmdArgs
exit $LASTEXITCODE
```

**Single-quoting note**: profile values (paths, env, attribution) embed in PowerShell single-quoted strings. Single quotes inside a value get doubled (`it's` → `it''s`). The generator implements this escape.

**`InstallWrapper` on Windows**: writes both files. `CleanupStaleScripts` reads both `.cmd` and `.ps1` files in `bin_dir`, checks for the marker, removes stale ones (both files of a removed profile).

### 3.2 Shell hook (`hook_windows.go`)

`cpm hook` emits this PowerShell, intended to be added once to `$PROFILE`:

```powershell
# cpm auto-switch hook — add to $PROFILE:
#   Invoke-Expression (& cpm hook | Out-String)

$script:__cpm_last_dir = $null
$script:__cpm_orig_prompt = $function:prompt

function global:prompt {
  if ($PWD.Path -ne $script:__cpm_last_dir) {
    $script:__cpm_last_dir = $PWD.Path

    $dir = $PWD.Path
    $profileFile = $null
    while ($dir) {
      $candidate = Join-Path $dir '.claude-profile'
      if (Test-Path -LiteralPath $candidate) { $profileFile = $candidate; break }
      $parent = Split-Path $dir -Parent
      if (-not $parent -or $parent -eq $dir) { break }
      $dir = $parent
    }

    if ($profileFile) {
      $target = (Get-Content -LiteralPath $profileFile -Raw).Trim()
      if ($target -and $target -ne $env:CLAUDE_PROFILE) {
        Invoke-Expression (& cpm use $target 2>$null | Out-String)
        Write-Host "[cpm] using profile: $target" -ForegroundColor DarkCyan
      }
    } elseif ($env:CLAUDE_PROFILE) {
      Get-ChildItem env: | Where-Object { $_.Name -match '^(CLAUDE_|ANTHROPIC_)' } |
        ForEach-Object { Remove-Item "env:$($_.Name)" }
      Write-Host "[cpm] profile unset" -ForegroundColor DarkGray
    }
  }

  & $script:__cpm_orig_prompt
}
```

Design choices:
- Saves the user's original `prompt` function in `$script:__cpm_orig_prompt` and calls it at the end — compatible with starship, oh-my-posh, or any user-defined prompt.
- Diffs `$PWD.Path` against last-seen path; logic runs only when the user actually changed directory.
- Does not override `Set-Location` or `cd` — avoids conflicts with zoxide, z, or custom aliases.

### 3.3 `cpm use` (`use_windows.go`)

`cpm use <name>` outputs:

```powershell
Get-ChildItem env: | Where-Object { $_.Name -match '^(CLAUDE_|ANTHROPIC_)' } | ForEach-Object { Remove-Item "env:$($_.Name)" }
$env:CLAUDE_CONFIG_DIR = '<profileDir>'
$env:CLAUDE_PROFILE = '<name>'
# Per profile.Env entries
Write-Host "Switched to profile: <name>"
```

Recommended user invocation:

```powershell
Invoke-Expression (& cpm use work | Out-String)
```

README suggests defining a function alias:

```powershell
function cpmuse { Invoke-Expression (& cpm use $args[0] | Out-String) }
```

### 3.4 `cpm direnv` (`direnv_windows.go`)

Outputs a PowerShell snippet suitable for `.envrc.ps1` (direnv-windows or user-managed):

```powershell
# Claude Code profile: <name>
$env:CLAUDE_CONFIG_DIR = '<profileDir>'
$env:CLAUDE_PROFILE = '<name>'
```

### 3.5 Sharing mechanism (`link_windows.go`)

```go
// linkShared creates a directory link from dst → src.
// Strategy: junction for same-volume directories (no privilege needed);
//           symlink fallback for cross-volume (requires Developer Mode).
func linkShared(src, dst string) error {
    src = filepath.Clean(src)
    dst = filepath.Clean(dst)

    if sameVolume(src, dst) {
        if err := mklinkJunction(dst, src); err == nil {
            return nil
        }
        // junction failed (extremely rare; e.g. NTFS-only feature missing) — fall through to symlink
    }

    if err := os.Symlink(src, dst); err == nil {
        return nil
    } else if isPrivilegeNotHeldError(err) {
        return fmt.Errorf(
            "cannot create symlink to %s — Windows requires Developer Mode for cross-volume symlinks.\n"+
            "Enable: Settings → Privacy & Security → For developers → Developer Mode = On\n"+
            "Then re-run 'cpm install'.", src)
    } else {
        return err
    }
}

func sameVolume(a, b string) bool {
    return strings.EqualFold(filepath.VolumeName(a), filepath.VolumeName(b))
}

func mklinkJunction(dst, src string) error {
    cmd := exec.Command("cmd", "/c", "mklink", "/J", dst, src)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("mklink /J: %s: %w", strings.TrimSpace(string(out)), err)
    }
    return nil
}

func resolveLinkTarget(path string) (string, error) {
    target, err := os.Readlink(path)
    if err != nil {
        return "", err
    }
    target = strings.TrimPrefix(target, `\??\`)  // junction targets prefix this on Windows
    return filepath.Clean(target), nil
}

func isPrivilegeNotHeldError(err error) bool {
    return strings.Contains(err.Error(), "A required privilege is not held by the client")
}
```

**`profile.go` change**: the symlink branch replaces `os.Symlink(src, dst)` with `linkShared(src, dst)`, and `os.Readlink(dst)` with `resolveLinkTarget(dst)`. The "is the existing link pointing at the right place" check now correctly handles junctions.

**`doctor.go` change**: same swap for the `Readlink` call in the broken-link check.

### 3.6 Default paths (`paths_windows.go`)

```go
func defaultBinDir() string {
    if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
        return filepath.Join(appData, "cpm", "bin")
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".local", "bin")  // extreme fallback
}

func wrapperMainFilename(profileName string) string  { return "claude-" + profileName + ".cmd" }
func wrapperSidecarFilename(profileName string) string { return "claude-" + profileName + ".ps1" }
func wrapperFilenames(profileName string) []string {
    return []string{wrapperMainFilename(profileName), wrapperSidecarFilename(profileName)}
}
```

`SourceDir` default remains `~/.claude` (Claude Code uses this path on Windows too).
`ProfilesBaseDir` default remains `~/.claude-profiles`.
Both rely on `os.UserHomeDir()` which is already cross-platform.

### 3.7 `execClaude` (`exec_windows.go`)

```go
func execClaude(claudePath string, args []string, env []string) error {
    cmd := exec.Command(claudePath, args[1:]...)
    cmd.Env = env
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            os.Exit(exitErr.ExitCode())
        }
        return err
    }
    os.Exit(0)
    return nil
}
```

Tradeoff: `cpm.exe` stays resident as a parent process while `claude` runs (~10MB extra). Ctrl-C is propagated by Go's default signal handling on Windows (`CTRL_C_EVENT` to the process group).

### 3.8 Self-upgrade (`upgrade_windows.go`)

Windows cannot delete or overwrite a running executable, but **can rename it**:

```go
func replaceBinary(tmpPath, targetPath string) error {
    oldPath := targetPath + ".old"
    _ = os.Remove(oldPath)  // clean stragglers from previous upgrade

    if _, err := os.Stat(targetPath); err == nil {
        if err := os.Rename(targetPath, oldPath); err != nil {
            return fmt.Errorf("rename current binary aside: %w", err)
        }
    }

    if err := os.Rename(tmpPath, targetPath); err != nil {
        _ = os.Rename(oldPath, targetPath)  // best-effort rollback
        return fmt.Errorf("install new binary: %w", err)
    }
    return nil
}
```

On every subsequent `cpm` invocation, a tiny init-time hook silently `os.Remove`s `<targetPath>.old` if present.

### 3.9 Asset name in self-upgrade (`version.go`)

```go
assetName := fmt.Sprintf("cpm_%s_%s", runtime.GOOS, runtime.GOARCH)
if runtime.GOOS == "windows" {
    assetName += ".exe"
}
```

The version-check / latest-tag logic is already cross-platform.

---

## 4. CI / Release / Distribution

### `.goreleaser.yaml`

```yaml
version: 2

builds:
  - binary: cpm
    env: [CGO_ENABLED=0]
    goos: [darwin, linux, windows]
    goarch: [amd64, arm64]
    ignore:
      - { goos: windows, goarch: arm64 }
    ldflags:
      - -s -w
      - -X github.com/jakubkontra/cpm/internal.Version={{.Version}}
      - -X github.com/jakubkontra/cpm/internal.Commit={{.ShortCommit}}

archives:
  - format: binary
    name_template: "cpm_{{ .Os }}_{{ .Arch }}{{ if eq .Os \"windows\" }}.exe{{ end }}"

scoops:
  - repository:
      owner: <fork-owner>
      name: scoop-cpm
      token: "{{ .Env.SCOOP_TAP_TOKEN }}"
    homepage: "https://github.com/<fork-owner>/cpm"
    description: "Claude Profile Manager — manage multiple Claude Code accounts"
    license: MIT

brews:                                        # unchanged from upstream
  - repository:
      owner: jakubkontra
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    ...
```

The fork-owner placeholder is filled during the publishing decision (see Open Questions).

### GitHub Actions matrix

`.github/workflows/test.yml`:

```yaml
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
runs-on: ${{ matrix.os }}
steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with: { go-version: '1.26' }
  - run: go test ./...
```

GitHub-hosted `windows-latest` runners have Developer Mode enabled by default, so symlink fallback tests run too.

### Distribution channels

| Channel | Status | Notes |
|---|---|---|
| GitHub Releases | Required | `cpm_windows_amd64.exe` alongside existing assets |
| Homebrew tap | Unchanged | darwin/linux only |
| Scoop bucket | **New** — `<fork-owner>/scoop-cpm` | Generated by goreleaser |
| winget | Deferred | Submit a manifest to winget-pkgs once stable |

User-side Windows install:

```powershell
scoop bucket add cpm https://github.com/<fork-owner>/scoop-cpm
scoop install cpm
```

### `cpm init` interactive prompt (Windows path)

After collecting source_dir / bin_dir / profiles, print:

```
You're on Windows. A few quick checks:

1. PATH: %LOCALAPPDATA%\cpm\bin should be on your PATH.
   [Environment]::SetEnvironmentVariable('Path', $env:Path + ';' + "$env:LOCALAPPDATA\cpm\bin", 'User')

2. Auto-switch hook: add to $PROFILE
   Invoke-Expression (& cpm hook | Out-String)

3. (Rare) For cross-volume symlinks, enable Developer Mode:
   Settings → Privacy & Security → For developers → Developer Mode = On

Run 'cpm doctor' to verify.
```

---

## 5. Testing, Compatibility, Documentation

### Test layout

| Tier | Files | Where it runs | Coverage |
|---|---|---|---|
| Cross-platform logic | `config_test.go`, `profile_test.go`, `sync_test.go`, `cloud_test.go`, `doctor_test.go` (non-platform parts) | all 3 | TOML parsing, copy/sync, attribution patch, JSON shuffling |
| Unix behavior | `*_unix_test.go` with `//go:build !windows` | ubuntu + macos | bash output verification (contains `set -euo pipefail`, `compgen` regex, `add-zsh-hook`) |
| Windows behavior | `*_windows_test.go` with `//go:build windows` | windows-latest | .cmd + .ps1 content verification, junction roundtrip |
| Junction end-to-end | `link_windows_test.go::TestJunctionRoundtrip` | windows | mkdir → linkShared → resolveLinkTarget → assert path equality after `\??\` strip |

### Configuration compatibility

The `config.toml` schema has **zero changes**. The same file is portable across all three platforms:

- `source_dir = "~/.claude"` — works everywhere (`os.UserHomeDir()` expands)
- `bin_dir = "~/.local/bin"` — works on Windows too (user can pick the Unix-style path), but the **default** value (when the field is empty or absent) differs by platform
- Profile attributes (`description`, `model`, `add_dirs`, `env`, `attribution`) are all platform-neutral

This means: a user can `cpm cloud push` from macOS and `cpm cloud pull` on Windows. Only the wrapper file names differ.

### README changes — minimal

Add one section, `## Install on Windows`, with the scoop + GH Releases instructions and the `$PROFILE` hook one-liner. Add to the Requirements list:
- Windows 10 1703+ or Windows 11
- PowerShell 7+ (`pwsh`) — not Windows PowerShell 5.1
- Normal user is enough; no Administrator required
- For cross-volume symlink fallback only: Developer Mode

### Error handling matrix

| Scenario | Behavior |
|---|---|
| `claude.exe` not on PATH | `doctor` reports error; suggests `npm install -g @anthropic-ai/claude-code` |
| `pwsh` not on PATH | `cpm install` aborts with: install pwsh via `winget install Microsoft.PowerShell` |
| junction creation fails (rare) | falls back to symlink; if symlink fails too with privilege-not-held, error with Developer Mode instructions |
| `dst` exists as a real directory (not a link) | unchanged: `skipped <dir>/ (real directory exists)` |
| User copies `.credentials.json` from mac to Windows | works; file content is platform-neutral |
| Ctrl-C during `cpm run` | Go default signal handling propagates `CTRL_C_EVENT` to child |
| Stale `cpm.exe.old` after upgrade | silently removed on next `cpm` invocation |

### Explicit out-of-scope

- No PowerShell module (`.psd1` / `.psm1`) packaging — wrappers are plain `.ps1` files
- No MSI / Inno installer — Scoop + GH Releases binary only
- No automatic Developer Mode enable — surface only as an actionable error message
- No installation to `Program Files` — `%LOCALAPPDATA%\cpm\bin` is unprivileged and sufficient

---

## 6. Open Questions (resolve before implementation)

1. **GitHub fork owner** — the design uses `<fork-owner>` placeholder in scoop manifest and README. Resolve before any push. Candidates: personal GitHub account vs. `proem` org.
2. **Module path** — the design keeps `module github.com/jakubkontra/cpm` so cherry-picks from upstream don't need import rewrites, and a future PR back is friction-free. If the fork eventually publishes as a distinct module (e.g. `github.com/<fork-owner>/cpm`), the rename is a one-shot mechanical change. Recommendation: keep upstream module path.
3. **Branch strategy** — `main` tracks upstream; `windows-support` is the development branch where all changes land. After the feature is stable, merge `windows-support` into `main` and tag a release.

---

## 7. Implementation order (for the upcoming plan)

The implementation plan (next phase, after this spec is approved) will sequence the work roughly as:

1. **Skeleton** — create all `_unix.go` / `_windows.go` stubs that compile but return errors; move existing logic into `_unix.go` files unchanged; verify three-platform builds pass.
2. **Path helpers** (`paths_windows.go`) + **build tags wiring** in `config.go` / `init.go`.
3. **Link layer** (`link_windows.go` + junction tests) — highest-risk piece, test thoroughly first.
4. **Wrapper generation** (`wrapper_windows.go`) — `.cmd` + `.ps1` pair, escape testing.
5. **Shell hook & use** (`hook_windows.go`, `use_windows.go`, `direnv_windows.go`).
6. **Exec & upgrade** (`exec_windows.go`, `upgrade_windows.go`).
7. **CI matrix** and **goreleaser** changes.
8. **README** Windows section.
9. **Manual end-to-end smoke test** on a real Windows install (`cpm init` → `install` → `claude-personal --version` → hook auto-switch).
