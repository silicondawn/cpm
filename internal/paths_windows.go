//go:build windows

package internal

import (
	"os"
	"path/filepath"
)

func defaultBinDir() string {
	if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
		return filepath.Join(appData, "cpm", "bin")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

func wrapperFilenames(profileName string) []string {
	return []string{
		"claude-" + profileName + ".cmd",
		"claude-" + profileName + ".ps1",
	}
}
