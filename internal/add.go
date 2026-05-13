package internal

import (
	"fmt"
	"os"
	"strings"
)

// RunAdd interactively appends a single new profile to an existing
// config.toml and (unless skipInstall is true) runs the install pipeline
// so the user can launch claude-<name> immediately.
//
// Differs from RunOnboard:
//   - requires an existing config (refuses to bootstrap)
//   - asks for exactly one profile, not a loop
//   - appends to the file as raw text, preserving any manual edits the
//     user made to source_dir / bin_dir / other profile sections / comments
func RunAdd(configPath string, skipInstall bool) error {
	configPath = ExpandPath(configPath)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("no usable config at %s — run 'cpm onboard' first (%w)", configPath, err)
	}

	existing := map[string]bool{}
	for name := range cfg.Profiles {
		existing[name] = true
	}

	fmt.Printf("\nAdding a new profile to %s\n", configPath)
	if len(existing) > 0 {
		names := make([]string, 0, len(existing))
		for n := range existing {
			names = append(names, n)
		}
		fmt.Printf("Existing profiles: %s\n\n", strings.Join(names, ", "))
	}

	p, err := promptProfileForm()
	if err != nil {
		return err
	}
	if existing[p.Name] {
		return fmt.Errorf("profile %q already exists in %s — edit it manually or pick a different name", p.Name, configPath)
	}

	block := renderProfileTOMLBlock(p)
	if err := appendToConfigFile(configPath, block); err != nil {
		return fmt.Errorf("cannot append profile to config: %w", err)
	}
	fmt.Printf("\nAppended profile %q to %s\n", p.Name, configPath)

	if skipInstall {
		fmt.Println("Next: run 'cpm install' to create the profile dir and wrapper script.")
		return nil
	}

	fmt.Println("Running 'cpm install' to create profile dir and wrapper scripts...")
	return RunInstall(configPath, false, false)
}

// appendToConfigFile appends block to path. Block is expected to already
// start with "\n[profiles.<name>]" (per renderProfileTOMLBlock), so the
// only thing we have to guarantee is that the existing file ends with a
// newline.
func appendToConfigFile(path, block string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		block = "\n" + block
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(block)
	return err
}
