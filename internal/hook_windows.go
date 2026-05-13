//go:build windows

package internal

func GenerateShellHook() string {
	return `# cpm auto-switch hook — add to your PowerShell profile ($PROFILE):
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
`
}
