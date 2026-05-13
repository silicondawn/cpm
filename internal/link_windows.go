//go:build windows

package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// linkShared creates dst as a directory link pointing to src.
// Strategy: junction for same-volume directories (no privilege needed);
// symlink fallback for cross-volume (requires Developer Mode).
func linkShared(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if sameVolume(src, dst) {
		if err := mklinkJunction(dst, src); err == nil {
			return nil
		}
		// junction failed (extremely rare); fall through to symlink
	}

	if err := os.Symlink(src, dst); err == nil {
		return nil
	} else if isPrivilegeNotHeldError(err) {
		return fmt.Errorf(
			"cannot create symlink to %s — Windows requires Developer Mode for cross-volume symlinks.\n"+
				"Enable: Settings → Privacy & Security → For developers → Developer Mode = On\n"+
				"Then re-run 'cpm install'.", src)
	} else {
		return fmt.Errorf("cannot create symlink to %s: %w", src, err)
	}
}

// resolveLinkTarget reads the link target and normalizes Windows junction
// prefixes (\??\ or \\?\) out, returning a path comparable to the original src.
func resolveLinkTarget(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	target = strings.TrimPrefix(target, `\??\`)
	target = strings.TrimPrefix(target, `\\?\`)
	return filepath.Clean(target), nil
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

func isPrivilegeNotHeldError(err error) bool {
	return strings.Contains(err.Error(), "A required privilege is not held by the client")
}
