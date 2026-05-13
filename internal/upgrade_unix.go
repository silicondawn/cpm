//go:build !windows

package internal

import "os"

func replaceBinary(tmpPath, targetPath string) error {
	return os.Rename(tmpPath, targetPath)
}

// CleanupOldBinary is a no-op on Unix.
func CleanupOldBinary(targetPath string) {}
