package internal

import (
	"fmt"
	"path/filepath"
	"sort"
)

// RunInstall walks every profile in the loaded config and:
//   - creates the profile dir, copies mutable files, links shared dirs
//   - patches attribution settings, seeds .claude.json onboarding state
//   - generates and writes the wrapper script(s)
//   - cleans up stale wrappers for removed profiles
//
// sync = re-copy mutable files from source; force = overwrite diverged ones.
// Used by `cpm install` and called automatically at the end of `cpm onboard`.
func RunInstall(configPath string, sync, force bool) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	profilesBase := ProfilesBaseDir(configPath)

	if sync && !force {
		diverged := CheckDivergence(cfg, profilesBase)
		if len(diverged) > 0 {
			fmt.Print("\nDiverged profile files detected:\n\n")
			for _, d := range diverged {
				fmt.Printf("  %s\n", d.Details)
			}
			fmt.Println("\nMerge changes back to source first, or use: --sync --force")
			return nil
		}
	}

	activeNames := make(map[string]bool)
	names := make([]string, 0, len(cfg.Profiles))
	for k := range cfg.Profiles {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		profile := cfg.Profiles[name]
		profileDir := filepath.Join(profilesBase, name)

		activeNames[name] = true
		fmt.Printf("\nProfile: %s\n", name)

		if err := SetupProfile(name, profileDir, cfg.SourceDir, sync); err != nil {
			return fmt.Errorf("profile %s: %w", name, err)
		}
		if err := PatchAttribution(profileDir, profile.Attribution); err != nil {
			return fmt.Errorf("profile %s attribution: %w", name, err)
		}
		if err := DenyUnsupportedTools(profileDir, profile); err != nil {
			return fmt.Errorf("profile %s deny tools: %w", name, err)
		}
		if err := SyncMCPServers(profileDir); err != nil {
			return fmt.Errorf("profile %s mcp sync: %w", name, err)
		}
		if err := InjectTavilyHint(profileDir, profile); err != nil {
			return fmt.Errorf("profile %s tavily hint: %w", name, err)
		}
		files := GenerateWrapper(name, profileDir, profile)
		if _, err := InstallWrapper(cfg.BinDir, files); err != nil {
			return fmt.Errorf("profile %s wrapper: %w", name, err)
		}
	}

	fmt.Println("\nCleanup:")
	CleanupStaleScripts(cfg.BinDir, activeNames)

	fmt.Println("\nDone. To start a profile, run one of:")
	for _, name := range names {
		fmt.Printf("  claude-%s\n", name)
	}
	return nil
}
