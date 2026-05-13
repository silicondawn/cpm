package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallWrapper(t *testing.T) {
	dir := t.TempDir()
	files := WrapperFiles{
		Main:        "claude-test",
		MainContent: "#!/usr/bin/env bash\necho test\n",
		Mode:        0o755,
	}

	mainPath, err := InstallWrapper(dir, files)
	if err != nil {
		t.Fatalf("InstallWrapper failed: %v", err)
	}
	if mainPath != filepath.Join(dir, "claude-test") {
		t.Errorf("mainPath = %q, want %q", mainPath, filepath.Join(dir, "claude-test"))
	}

	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("cannot read script: %v", err)
	}
	if string(data) != files.MainContent {
		t.Errorf("script content mismatch")
	}

	if runtime.GOOS != "windows" {
		info, _ := os.Stat(mainPath)
		if info.Mode()&0o111 == 0 {
			t.Error("script should be executable")
		}
	}
}

func TestInstallWrapperSidecar(t *testing.T) {
	dir := t.TempDir()
	files := WrapperFiles{
		Main:           "claude-test.cmd",
		MainContent:    "@echo shim\n",
		Sidecar:        "claude-test.ps1",
		SidecarContent: "Write-Host hello\n",
		Mode:           0o644,
	}

	if _, err := InstallWrapper(dir, files); err != nil {
		t.Fatalf("InstallWrapper failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "claude-test.cmd")); err != nil {
		t.Errorf("main file should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "claude-test.ps1")); err != nil {
		t.Errorf("sidecar file should exist: %v", err)
	}
}

func TestInstallWrapperIdempotent(t *testing.T) {
	dir := t.TempDir()
	files := WrapperFiles{
		Main:        "claude-test",
		MainContent: "#!/usr/bin/env bash\necho test\n",
		Mode:        0o755,
	}

	mainPath, err := InstallWrapper(dir, files)
	if err != nil {
		t.Fatalf("first InstallWrapper failed: %v", err)
	}
	info1, _ := os.Stat(mainPath)

	// Second install with same content should not rewrite
	if _, err := InstallWrapper(dir, files); err != nil {
		t.Fatalf("second InstallWrapper failed: %v", err)
	}

	info2, _ := os.Stat(mainPath)
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("wrapper should not be rewritten when content is unchanged")
	}
}

func TestCleanupStaleScripts(t *testing.T) {
	dir := t.TempDir()

	// Write all filenames the active profile owns on this platform.
	// (Unix: ["claude-keep"]; Windows: ["claude-keep.cmd", "claude-keep.ps1"])
	for _, fn := range wrapperFilenames("keep") {
		os.WriteFile(filepath.Join(dir, fn), []byte(marker+"\nkeep\n"), 0o644)
	}
	// Stale script — uses cpm marker but no longer in active set
	for _, fn := range wrapperFilenames("old") {
		os.WriteFile(filepath.Join(dir, fn), []byte(marker+"\nold\n"), 0o644)
	}
	// Non-cpm script — no marker, must not be touched
	other := filepath.Join(dir, "other-script")
	os.WriteFile(other, []byte("just some script\n"), 0o644)

	// A binary-like file whose contents happen to contain the marker
	// (simulating cpm.exe itself, which embeds the marker constant in
	// .rodata). Must not be touched because its name does not start
	// with "claude-".
	binaryLike := filepath.Join(dir, "cpm.exe")
	os.WriteFile(binaryLike, []byte("MZ\x00"+marker+"\x00more"), 0o644)

	activeProfiles := map[string]bool{"keep": true}
	CleanupStaleScripts(dir, activeProfiles)

	for _, fn := range wrapperFilenames("keep") {
		if _, err := os.Stat(filepath.Join(dir, fn)); err != nil {
			t.Errorf("active script %s should be kept: %v", fn, err)
		}
	}
	for _, fn := range wrapperFilenames("old") {
		if _, err := os.Stat(filepath.Join(dir, fn)); !os.IsNotExist(err) {
			t.Errorf("stale script %s should have been removed", fn)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("non-cpm script should be kept")
	}
	if _, err := os.Stat(binaryLike); err != nil {
		t.Error("binary-like file with embedded marker should be kept (name does not start with claude-)")
	}
}
