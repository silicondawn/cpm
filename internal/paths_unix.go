//go:build !windows

package internal

import (
	"os"
	"path/filepath"
)

func defaultBinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

func wrapperFilenames(profileName string) []string {
	return []string{"claude-" + profileName}
}
