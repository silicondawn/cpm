//go:build windows

package internal

import (
	"os"
	"path/filepath"
	"strings"
)

// defaultBinDir picks the best default bin_dir for Windows:
//
//  1. If cpm.exe itself was installed by scoop (path under ~/scoop/shims
//     or ~/scoop/apps), use ~/scoop/shims — it is already on PATH and
//     the wrappers naturally live next to the cpm shim.
//  2. Otherwise, %LOCALAPPDATA%\cpm\bin (user has to add it to PATH;
//     `cpm init` prints how).
//  3. Last-ditch fallback: ~/.local/bin.
func defaultBinDir() string {
	if dir := scoopShimsDir(); dir != "" {
		return dir
	}
	if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
		return filepath.Join(appData, "cpm", "bin")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

// scoopShimsDir returns ~/scoop/shims if cpm.exe is running from a scoop
// install (either the shim itself or the versioned app dir), otherwise "".
func scoopShimsDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// Normalize for case-insensitive substring matching on Windows.
	norm := strings.ToLower(filepath.ToSlash(exe))

	// Match either ~/scoop/shims/cpm.exe or ~/scoop/apps/cpm/...
	for _, marker := range []string{"/scoop/shims/", "/scoop/apps/"} {
		if idx := strings.Index(norm, marker); idx >= 0 {
			// Use the cased original path up to the marker, then append /scoop/shims.
			return filepath.Join(exe[:idx], "scoop", "shims")
		}
	}
	return ""
}

func wrapperFilenames(profileName string) []string {
	return []string{
		"claude-" + profileName + ".cmd",
		"claude-" + profileName + ".ps1",
	}
}
