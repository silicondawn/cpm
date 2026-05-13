package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Check struct {
	Name   string
	Status string // "ok", "warn", "error"
	Detail string
}

func RunDoctor(cfg *Config, profilesBase string) []Check {
	var checks []Check

	// Check claude binary
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		checks = append(checks, Check{"claude binary", "error", "claude not found on PATH"})
	} else {
		checks = append(checks, Check{"claude binary", "ok", claudePath})
	}

	// Check source dir
	if _, err := os.Stat(cfg.SourceDir); os.IsNotExist(err) {
		checks = append(checks, Check{"source directory", "error", fmt.Sprintf("%s does not exist", cfg.SourceDir)})
	} else {
		checks = append(checks, Check{"source directory", "ok", cfg.SourceDir})
	}

	// Check bin dir
	if _, err := os.Stat(cfg.BinDir); os.IsNotExist(err) {
		checks = append(checks, Check{"bin directory", "warn", fmt.Sprintf("%s does not exist (will be created on install)", cfg.BinDir)})
	} else {
		checks = append(checks, Check{"bin directory", "ok", cfg.BinDir})
	}

	// Check bin dir is on PATH
	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	binOnPath := false
	for _, d := range pathDirs {
		if d == cfg.BinDir {
			binOnPath = true
			break
		}
	}
	if binOnPath {
		checks = append(checks, Check{"bin dir on PATH", "ok", cfg.BinDir})
	} else {
		checks = append(checks, Check{"bin dir on PATH", "warn", fmt.Sprintf("%s is not on PATH", cfg.BinDir)})
	}

	// Check each profile
	for name := range cfg.Profiles {
		profileDir := filepath.Join(profilesBase, name)

		if _, err := os.Stat(profileDir); os.IsNotExist(err) {
			checks = append(checks, Check{fmt.Sprintf("profile/%s", name), "warn", "not installed (run cpm install)"})
			continue
		}

		// Check shared-directory links
		for _, dir := range symlinkDirs {
			link := filepath.Join(profileDir, dir)
			target, err := resolveLinkTarget(link)
			if err != nil {
				continue // Not a link or doesn't exist
			}
			if _, err := os.Stat(target); os.IsNotExist(err) {
				checks = append(checks, Check{fmt.Sprintf("profile/%s/%s", name, dir), "error", fmt.Sprintf("broken link -> %s", target)})
			}
		}

		// Check credentials
		credPath := filepath.Join(profileDir, ".credentials.json")
		credInfo, err := os.Stat(credPath)
		if os.IsNotExist(err) {
			checks = append(checks, Check{fmt.Sprintf("profile/%s/credentials", name), "warn", "not authenticated (run claude-" + name + ")"})
		} else if err == nil {
			age := time.Since(credInfo.ModTime())
			if age > 7*24*time.Hour {
				checks = append(checks, Check{fmt.Sprintf("profile/%s/credentials", name), "warn", fmt.Sprintf("credentials last updated %s ago", formatDuration(age))})
			} else {
				checks = append(checks, Check{fmt.Sprintf("profile/%s/credentials", name), "ok", fmt.Sprintf("last updated %s ago", formatDuration(age))})
			}
		}

		// Check wrapper script(s) — Unix has 1 file; Windows has .cmd + .ps1
		expected := wrapperFilenames(name)
		var missing []string
		var present []string
		for _, fn := range expected {
			p := filepath.Join(cfg.BinDir, fn)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				missing = append(missing, fn)
			} else {
				present = append(present, p)
			}
		}
		if len(missing) > 0 {
			checks = append(checks, Check{
				fmt.Sprintf("profile/%s/wrapper", name),
				"warn",
				fmt.Sprintf("missing: %s (run cpm install)", strings.Join(missing, ", ")),
			})
		} else {
			checks = append(checks, Check{
				fmt.Sprintf("profile/%s/wrapper", name),
				"ok",
				strings.Join(present, ", "),
			})
		}
	}

	return checks
}

func PrintChecks(checks []Check) {
	for _, c := range checks {
		var icon string
		switch c.Status {
		case "ok":
			icon = "  OK"
		case "warn":
			icon = "WARN"
		case "error":
			icon = " ERR"
		}
		fmt.Printf("  [%s] %-35s %s\n", icon, c.Name, c.Detail)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func GetCredentialInfo(profileDir string) (account string, expired bool, err error) {
	credPath := filepath.Join(profileDir, ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return "", false, fmt.Errorf("no credentials found")
	}

	var creds map[string]any
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", false, fmt.Errorf("cannot parse credentials")
	}

	// Try to extract account info
	if email, ok := creds["email"].(string); ok {
		account = email
	} else if sub, ok := creds["subject"].(string); ok {
		account = sub
	} else if id, ok := creds["account_uuid"].(string); ok {
		account = id
	} else {
		account = "(unknown account)"
	}

	// Check expiry
	if expiresAt, ok := creds["expires_at"].(float64); ok {
		expTime := time.Unix(int64(expiresAt), 0)
		expired = time.Now().After(expTime)
	} else if expiresIn, ok := creds["expires_in"].(float64); ok {
		// expires_in is relative — check file mod time
		info, statErr := os.Stat(credPath)
		if statErr == nil {
			expTime := info.ModTime().Add(time.Duration(expiresIn) * time.Second)
			expired = time.Now().After(expTime)
		}
	}

	return account, expired, nil
}
