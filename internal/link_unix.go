//go:build !windows

package internal

import "os"

func linkShared(src, dst string) error {
	return os.Symlink(src, dst)
}

func resolveLinkTarget(path string) (string, error) {
	return os.Readlink(path)
}
