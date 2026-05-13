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
	Name          string
	Description   string
	AuthMode      string // "oauth" or "api"
	Provider      string // "custom" | "deepseek-1m" | … — provider preset for api mode
	BaseURL       string
	APIKey        string
	AuthKeyVar    string // "api_key" | "auth_token" | "both" — which env var(s) the gateway expects (api mode only)
	Model         string
	TavilyEnabled bool
	TavilyAPIKey  string
}

// providerEnv is a single env var emitted by a provider preset. We use a
// slice so the rendered TOML output is in a predictable order rather
// than Go map iteration order.
type providerEnv struct{ Key, Value string }

// providerPreset bundles the curated defaults for a known Anthropic-
// compatible gateway. Onboard auto-fills these so the user only types
// their API key.
type providerPreset struct {
	Label      string // shown in the Provider select
	BaseURL    string
	AuthKeyVar string // "auth_token" / "api_key" / "both"
	Model      string // becomes profile.model in config.toml
	ExtraEnv   []providerEnv
}

// providerPresets is a registry of known gateways. Adding a new preset
// here automatically wires it into the onboard form and renderer.
var providerPresets = map[string]providerPreset{
	"deepseek-1m": {
		Label:      "DeepSeek (1M-context preset — official recommended)",
		BaseURL:    "https://api.deepseek.com/anthropic",
		AuthKeyVar: "auth_token",
		Model:      "deepseek-v4-pro[1m]",
		ExtraEnv: []providerEnv{
			{"ANTHROPIC_MODEL", "deepseek-v4-pro[1m]"},
			{"ANTHROPIC_DEFAULT_OPUS_MODEL", "deepseek-v4-pro[1m]"},
			{"ANTHROPIC_DEFAULT_SONNET_MODEL", "deepseek-v4-pro[1m]"},
			{"ANTHROPIC_DEFAULT_HAIKU_MODEL", "deepseek-v4-flash"},
			{"CLAUDE_CODE_SUBAGENT_MODEL", "deepseek-v4-flash"},
			{"CLAUDE_CODE_EFFORT_LEVEL", "max"},
		},
	},
}

// providerPresetOrder controls the order presets appear in the onboard
// Select. "custom" stays in promptProfileForm so it can be inserted
// first as the fallback option.
var providerPresetOrder = []string{"deepseek-1m"}

// applyProviderPreset overwrites BaseURL/AuthKeyVar/Model with the
// preset's values after the form runs. Extra env vars are not stored on
// the profile struct — the renderer reads them directly from the
// registry by Provider key.
func applyProviderPreset(p *onboardProfile) {
	if p.AuthMode != "api" || p.Provider == "" || p.Provider == "custom" {
		return
	}
	preset, ok := providerPresets[p.Provider]
	if !ok {
		return
	}
	p.BaseURL = preset.BaseURL
	p.AuthKeyVar = preset.AuthKeyVar
	p.Model = preset.Model
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
	// DeepSeek 1M-context preset is the default — first option in the
	// select and pre-selected. Pressing enter through the form on a
	// fresh profile auto-fills DS official-recommended config.
	p := onboardProfile{AuthMode: "oauth", Provider: "deepseek-1m", AuthKeyVar: "auth_token"}

	// Build the Provider select dynamically: presets in
	// providerPresetOrder first (the first one is the default),
	// "Custom" as the trailing fallback.
	providerOptions := []huh.Option[string]{}
	for _, key := range providerPresetOrder {
		preset, ok := providerPresets[key]
		if !ok {
			continue
		}
		providerOptions = append(providerOptions, huh.NewOption(preset.Label, key))
	}
	providerOptions = append(providerOptions, huh.NewOption("Custom — enter base URL, auth var, and model manually", "custom"))

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
					huh.NewOption("Anthropic-compatible API (base URL + API key — DeepSeek, Z.ai, GLM, sub2api, Manifest, custom gateway)", "api"),
				).
				Value(&p.AuthMode),
		),
		// API-mode group: Provider select. The custom-only details
		// (BASE_URL / AuthKeyVar / Model) live in their own group so
		// huh's group-level hide can skip them for presets. The API
		// key input has its own group too so it stays in the flow no
		// matter which provider was picked.
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Provider").
				Description("Pick a preset to skip base-URL / auth-var / model entry. The\npreset also wires recommended ANTHROPIC_DEFAULT_* + CLAUDE_CODE_*\nenv vars. Pick 'Custom' for anything else.").
				Options(providerOptions...).
				Value(&p.Provider),
		).WithHideFunc(func() bool { return p.AuthMode != "api" }),
		// Custom-only details (only shown when Provider = custom).
		huh.NewGroup(
			huh.NewInput().
				Title("ANTHROPIC_BASE_URL").
				Description("Examples:\n  https://api.deepseek.com/anthropic\n  https://api.z.ai/coding/paas/v4/anthropic\n  https://manifest.proem.dev").
				Placeholder("https://...").
				Value(&p.BaseURL).
				Validate(validateBaseURL),
			huh.NewSelect[string]().
				Title("Which auth env var does this gateway expect?").
				Description("AUTH_TOKEN  → 'Authorization: Bearer <value>'  (most third-party gateways)\nAPI_KEY     → 'x-api-key: <value>'              (direct api.anthropic.com)\nClaude Code prefers AUTH_TOKEN when both are set.").
				Options(
					huh.NewOption("ANTHROPIC_AUTH_TOKEN — DeepSeek, Z.ai, GLM, Manifest, sub2api, most gateways", "auth_token"),
					huh.NewOption("ANTHROPIC_API_KEY    — direct api.anthropic.com (sk-ant-… key)", "api_key"),
					huh.NewOption("Both                  — defensive: write to both vars", "both"),
				).
				Value(&p.AuthKeyVar),
			huh.NewInput().
				Title("Default model").
				Description("Provider-specific name. Examples:\n  deepseek-v4-pro, glm-4.6, claude-sonnet-4-5, auto").
				Value(&p.Model).
				Validate(validateNonEmpty("model")),
		).WithHideFunc(func() bool { return p.AuthMode != "api" || p.Provider != "custom" }),
		// API key — always shown in api mode, regardless of provider.
		huh.NewGroup(
			huh.NewInput().
				Title("API key / auth token").
				Description("Stored in config.toml in plain text. Press enter when done.").
				EchoMode(huh.EchoModePassword).
				Value(&p.APIKey).
				Validate(validateNonEmpty("API key")),
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
		// Tavily WebSearch simulation toggle (API-mode only — OAuth profiles
		// already have native WebSearch).
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Tavily-backed WebSearch simulation?").
				Description("API-mode profiles can't use Claude's native WebSearch/WebFetch.\nTavily (tavily.com) provides a substitute via plain HTTP — cpm will\ninject a self-contained usage hint into this profile's CLAUDE.md so\nthe agent knows to fall back to Tavily when web search is needed.").
				Affirmative("Yes, enable").
				Negative("No, skip").
				Value(&p.TavilyEnabled),
		).WithHideFunc(func() bool { return p.AuthMode != "api" }),
		huh.NewGroup(
			huh.NewInput().
				Title("TAVILY_API_KEY").
				Description("Get one at https://app.tavily.com (free tier available).\nStored in config.toml in plain text.").
				EchoMode(huh.EchoModePassword).
				Value(&p.TavilyAPIKey).
				Validate(validateNonEmpty("Tavily API key")),
		).WithHideFunc(func() bool { return p.AuthMode != "api" || !p.TavilyEnabled }),
	)

	if err := form.Run(); err != nil {
		return p, err
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	p.BaseURL = strings.TrimSpace(p.BaseURL)
	p.Model = strings.TrimSpace(p.Model)
	p.TavilyAPIKey = strings.TrimSpace(p.TavilyAPIKey)
	applyProviderPreset(&p)
	return p, nil
}

func renderConfigTOML(sourceDir, binDir string, profiles []onboardProfile) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("source_dir = %q\n", sourceDir))
	b.WriteString(fmt.Sprintf("bin_dir = %q\n", binDir))

	for _, p := range profiles {
		b.WriteString(renderProfileTOMLBlock(p))
	}
	return b.String()
}

// renderProfileTOMLBlock renders a single profile section (with leading
// blank line) — shared by onboard's full-file writer and `cpm add`'s
// append-only writer so both keep the same TOML shape.
func renderProfileTOMLBlock(p onboardProfile) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n[profiles.%s]\n", p.Name))
	if p.Description != "" {
		b.WriteString(fmt.Sprintf("description = %q\n", p.Description))
	}
	if p.Model != "" {
		b.WriteString(fmt.Sprintf("model = %q\n", p.Model))
	}
	needEnv := p.AuthMode == "api" || (p.TavilyEnabled && p.TavilyAPIKey != "")
	if needEnv {
		b.WriteString(fmt.Sprintf("\n[profiles.%s.env]\n", p.Name))
	}
	if p.AuthMode == "api" {
		b.WriteString(fmt.Sprintf("ANTHROPIC_BASE_URL = %q\n", p.BaseURL))
		switch p.AuthKeyVar {
		case "auth_token":
			b.WriteString(fmt.Sprintf("ANTHROPIC_AUTH_TOKEN = %q\n", p.APIKey))
		case "both":
			b.WriteString(fmt.Sprintf("ANTHROPIC_API_KEY = %q\n", p.APIKey))
			b.WriteString(fmt.Sprintf("ANTHROPIC_AUTH_TOKEN = %q\n", p.APIKey))
		default: // "api_key" or unset (legacy)
			b.WriteString(fmt.Sprintf("ANTHROPIC_API_KEY = %q\n", p.APIKey))
		}
		// Provider preset extra env vars (deterministic order from the
		// preset definition's ExtraEnv slice).
		if preset, ok := providerPresets[p.Provider]; ok {
			for _, ev := range preset.ExtraEnv {
				b.WriteString(fmt.Sprintf("%s = %q\n", ev.Key, ev.Value))
			}
		}
	}
	if p.TavilyEnabled && p.TavilyAPIKey != "" {
		b.WriteString(fmt.Sprintf("TAVILY_API_KEY = %q\n", p.TavilyAPIKey))
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
