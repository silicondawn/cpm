package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
)

type onboardProfile struct {
	Name        string
	Description string
	AuthMode    string // "oauth" or "api"
	BaseURL     string
	APIKey      string
	Model       string
}

// RunOnboard runs the interactive TUI onboarding flow, writes config.toml,
// and (unless skipInstall is true) immediately runs the install pipeline so
// the user can launch claude-<name> right away.
// Replaces the older RunInit stdin prompts.
func RunOnboard(configPath string, skipInstall bool) error {
	configPath = ExpandPath(configPath)

	if _, err := os.Stat(configPath); err == nil {
		var overwrite bool
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Config already exists at %s", configPath)).
			Description("Overwrite it?").
			Affirmative("Overwrite").
			Negative("Cancel").
			Value(&overwrite).
			Run(); err != nil {
			return err
		}
		if !overwrite {
			return fmt.Errorf("aborted by user")
		}
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	binDir := defaultBinDir()
	sourceDir := "~/.claude"

	fmt.Printf("\nWelcome to cpm — let's set up your first profile.\n")
	fmt.Printf("Config will be written to: %s\n", configPath)
	fmt.Printf("Source dir:                %s\n", sourceDir)
	fmt.Printf("Wrappers will go to:       %s\n\n", binDir)

	var profiles []onboardProfile
	for {
		p, err := promptProfileForm()
		if err != nil {
			return err
		}
		profiles = append(profiles, p)

		var addMore bool
		if err := huh.NewConfirm().
			Title("Add another profile?").
			Affirmative("Yes, add another").
			Negative("No, done").
			Value(&addMore).
			Run(); err != nil {
			return err
		}
		if !addMore {
			break
		}
	}

	if len(profiles) == 0 {
		return fmt.Errorf("no profiles defined, aborting")
	}

	tomlOut := renderConfigTOML(sourceDir, binDir, profiles)
	if err := os.WriteFile(configPath, []byte(tomlOut), 0o644); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}

	fmt.Printf("\nConfig written to %s\n", configPath)

	if skipInstall {
		fmt.Println("Next: run 'cpm install' to create profile dirs and wrapper scripts.")
		printPostInitHints(binDir)
		return nil
	}

	fmt.Println("Running 'cpm install' to create profile dirs and wrapper scripts...")
	if err := RunInstall(configPath, false, false); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	printPostInitHints(binDir)
	return nil
}

func promptProfileForm() (onboardProfile, error) {
	p := onboardProfile{AuthMode: "oauth"}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Profile basics").
				Description("Each profile becomes a 'claude-<name>' command."),
			huh.NewInput().
				Title("Profile name").
				Placeholder("e.g. personal, work, deepseek").
				Value(&p.Name).
				Validate(validateProfileName),
			huh.NewInput().
				Title("Description (optional)").
				Placeholder("free-form short description").
				Value(&p.Description),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Authentication mode").
				Description("How does this profile authenticate to Claude?").
				Options(
					huh.NewOption("OAuth (Anthropic subscription, browser sign-in on first launch)", "oauth"),
					huh.NewOption("Anthropic-compatible API (base URL + API key — DeepSeek, Z.ai, GLM, sub2api, custom gateway)", "api"),
				).
				Value(&p.AuthMode),
		),
		// API-mode-only group
		huh.NewGroup(
			huh.NewInput().
				Title("ANTHROPIC_BASE_URL").
				Description("Examples:\n  https://api.deepseek.com/anthropic\n  https://api.z.ai/coding/paas/v4/anthropic\n  https://ai.proem.dev").
				Placeholder("https://...").
				Value(&p.BaseURL).
				Validate(validateBaseURL),
			huh.NewInput().
				Title("ANTHROPIC_API_KEY").
				Description("Stored in config.toml in plain text. Press enter when done.").
				EchoMode(huh.EchoModePassword).
				Value(&p.APIKey).
				Validate(validateNonEmpty("API key")),
			huh.NewInput().
				Title("Default model").
				Description("Provider-specific name. Examples:\n  deepseek-v4-pro, glm-4.6, claude-sonnet-4-5").
				Value(&p.Model).
				Validate(validateNonEmpty("model")),
		).WithHideFunc(func() bool { return p.AuthMode != "api" }),
		// OAuth-mode-only group (optional model default)
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default Claude model (optional)").
				Description("Skip if you want to use Claude Code's default.").
				Options(
					huh.NewOption("(none)", ""),
					huh.NewOption("sonnet", "sonnet"),
					huh.NewOption("opus", "opus"),
					huh.NewOption("haiku", "haiku"),
				).
				Value(&p.Model),
		).WithHideFunc(func() bool { return p.AuthMode != "oauth" }),
	)

	if err := form.Run(); err != nil {
		return p, err
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	p.BaseURL = strings.TrimSpace(p.BaseURL)
	p.Model = strings.TrimSpace(p.Model)
	return p, nil
}

func renderConfigTOML(sourceDir, binDir string, profiles []onboardProfile) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("source_dir = %q\n", sourceDir))
	b.WriteString(fmt.Sprintf("bin_dir = %q\n", binDir))

	for _, p := range profiles {
		b.WriteString(fmt.Sprintf("\n[profiles.%s]\n", p.Name))
		if p.Description != "" {
			b.WriteString(fmt.Sprintf("description = %q\n", p.Description))
		}
		if p.Model != "" {
			b.WriteString(fmt.Sprintf("model = %q\n", p.Model))
		}
		if p.AuthMode == "api" {
			b.WriteString(fmt.Sprintf("\n[profiles.%s.env]\n", p.Name))
			b.WriteString(fmt.Sprintf("ANTHROPIC_BASE_URL = %q\n", p.BaseURL))
			b.WriteString(fmt.Sprintf("ANTHROPIC_API_KEY = %q\n", p.APIKey))
		}
	}
	return b.String()
}

func validateProfileName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("profile name is required")
	}
	for _, r := range s {
		if !(r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return fmt.Errorf("only letters, digits, '-' and '_' are allowed")
		}
	}
	return nil
}

func validateBaseURL(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("base URL is required")
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return fmt.Errorf("must start with http:// or https://")
	}
	return nil
}

func validateNonEmpty(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

// printPostInitHints prints platform-aware next steps after a successful
// onboarding (PATH setup if needed, $PROFILE auto-switch hook, Developer
// Mode reminder on Windows). Skips the PATH-setup step when bin_dir is
// already on $PATH (e.g. scoop shims).
func printPostInitHints(binDir string) {
	if runtime.GOOS != "windows" {
		return
	}
	fmt.Println("\n--- Windows setup hints ---")

	step := 1
	if !isOnPath(binDir) {
		fmt.Printf("%d. Make sure %s is on your PATH:\n", step, binDir)
		fmt.Printf("   [Environment]::SetEnvironmentVariable('Path', $env:Path + ';' + %q, 'User')\n", binDir)
		step++
	} else {
		fmt.Printf("Bin dir %s is already on your PATH.\n", binDir)
	}
	fmt.Printf("%d. Add to your $PROFILE for auto-switch:\n", step)
	fmt.Println(`   Invoke-Expression (& cpm hook | Out-String)`)
	step++
	fmt.Printf("%d. (Rare) For cross-volume symlinks, enable Developer Mode in Windows Settings.\n", step)
	fmt.Println("\nRun 'cpm doctor' anytime to verify.")
}

// isOnPath reports whether dir is one of the entries in $PATH.
func isOnPath(dir string) bool {
	target := strings.ToLower(filepath.Clean(dir))
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.ToLower(filepath.Clean(p)) == target {
			return true
		}
	}
	return false
}
