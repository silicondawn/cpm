//go:build windows

package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLinkSharedJunctionRoundtrip verifies linkShared creates a junction
// and resolveLinkTarget reads back the original path (without the \??\ prefix).
func TestLinkSharedJunctionRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "link-to-src")

	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	if err := linkShared(src, dst); err != nil {
		t.Fatalf("linkShared: %v", err)
	}

	got, err := resolveLinkTarget(dst)
	if err != nil {
		t.Fatalf("resolveLinkTarget: %v", err)
	}
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(src)) {
		t.Fatalf("resolveLinkTarget = %q, want %q", got, src)
	}
}

// TestLinkSharedCreatesAccessibleDirectory verifies that after linkShared,
// the destination is usable as a directory (file inside src visible via dst).
func TestLinkSharedCreatesAccessibleDirectory(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "link")

	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	canary := filepath.Join(src, "marker.txt")
	if err := os.WriteFile(canary, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write canary: %v", err)
	}

	if err := linkShared(src, dst); err != nil {
		t.Fatalf("linkShared: %v", err)
	}

	via := filepath.Join(dst, "marker.txt")
	data, err := os.ReadFile(via)
	if err != nil {
		t.Fatalf("read via link: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("read via link = %q, want %q", string(data), "hello")
	}
}

// TestSameVolumeHelper sanity-checks the volume comparison.
func TestSameVolumeHelper(t *testing.T) {
	if !sameVolume(`C:\foo`, `C:\bar\baz`) {
		t.Error("expected C:\\foo and C:\\bar\\baz to share volume")
	}
	if sameVolume(`C:\foo`, `D:\bar`) {
		t.Error("expected C:\\foo and D:\\bar to differ")
	}
	if !sameVolume(`c:\foo`, `C:\bar`) {
		t.Error("expected case-insensitive equality")
	}
}
