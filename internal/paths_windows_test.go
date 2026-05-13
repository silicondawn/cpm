//go:build windows

package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultBinDirUsesLocalAppData(t *testing.T) {
	orig := os.Getenv("LOCALAPPDATA")
	t.Cleanup(func() { os.Setenv("LOCALAPPDATA", orig) })

	os.Setenv("LOCALAPPDATA", `C:\Users\testuser\AppData\Local`)
	got := defaultBinDir()
	want := filepath.Join(`C:\Users\testuser\AppData\Local`, "cpm", "bin")
	if got != want {
		t.Fatalf("defaultBinDir() = %q, want %q", got, want)
	}
}

func TestDefaultBinDirFallbackWhenLocalAppDataUnset(t *testing.T) {
	orig := os.Getenv("LOCALAPPDATA")
	t.Cleanup(func() { os.Setenv("LOCALAPPDATA", orig) })

	os.Unsetenv("LOCALAPPDATA")
	got := defaultBinDir()
	if !strings.Contains(got, ".local") {
		t.Fatalf("expected fallback to contain .local, got %q", got)
	}
}

func TestWrapperFilenamesWindows(t *testing.T) {
	got := wrapperFilenames("work")
	want := []string{"claude-work.cmd", "claude-work.ps1"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
