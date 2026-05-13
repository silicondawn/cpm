//go:build windows

package internal

import (
	"fmt"
	"os"
)

func replaceBinary(tmpPath, targetPath string) error {
	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath) // clean stragglers from previous upgrade

	if _, err := os.Stat(targetPath); err == nil {
		if err := os.Rename(targetPath, oldPath); err != nil {
			return fmt.Errorf("rename current binary aside: %w", err)
		}
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Rename(oldPath, targetPath) // best-effort rollback
		return fmt.Errorf("install new binary: %w", err)
	}
	return nil
}

// CleanupOldBinary silently removes targetPath+".old" if present.
// Called once at startup so the next upgrade can rename to .old again.
func CleanupOldBinary(targetPath string) {
	_ = os.Remove(targetPath + ".old")
}
