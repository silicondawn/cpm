package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Fields copied from source ~/.claude.json into each profile's
// .claude.json. Onboarding flags let the new profile skip the first-run
// onboarding flow. mcpServers brings the user's MCP setup along.
// Anything user/account/credential-specific is deliberately excluded
// (userID, hasAvailableSubscription, claudeCodeFirstTokenDate, etc.) —
// those should be discovered per profile via OAuth.
var syncedClaudeJSONFields = []string{
	"hasCompletedOnboarding",
	"lastOnboardingVersion",
	"firstStartTime",
	"installMethod",
	"hasIdeOnboardingBeenShown",
	"mcpServers",
}

// SyncMCPServers seeds a profile's .claude.json from the source
// ~/.claude.json: it carries forward onboarding flags (so the new
// profile doesn't replay the first-run wizard) and mcpServers (so MCP
// works out of the box). It creates the profile file if missing.
//
// The name is preserved for backwards compatibility with main.go even
// though the function now syncs more than just mcpServers.
func SyncMCPServers(profileDir string) error {
	home, _ := os.UserHomeDir()
	sourcePath := filepath.Join(home, ".claude.json")

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil // No source file, skip silently
	}

	var source map[string]any
	if err := json.Unmarshal(sourceData, &source); err != nil {
		return nil
	}

	profilePath := filepath.Join(profileDir, ".claude.json")
	profileData, err := os.ReadFile(profilePath)
	var profile map[string]any
	if err != nil {
		// Profile .claude.json doesn't exist yet — start fresh.
		profile = map[string]any{}
	} else if err := json.Unmarshal(profileData, &profile); err != nil {
		profile = map[string]any{}
	}

	changed := false
	mcpCount := 0
	for _, key := range syncedClaudeJSONFields {
		val, present := source[key]
		if !present {
			continue
		}
		existingJSON, _ := json.Marshal(profile[key])
		newJSON, _ := json.Marshal(val)
		if string(existingJSON) == string(newJSON) {
			continue
		}
		profile[key] = val
		changed = true
		if key == "mcpServers" {
			if m, ok := val.(map[string]any); ok {
				mcpCount = len(m)
			}
		}
	}

	if !changed {
		return nil
	}

	out, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(profilePath, append(out, '\n'), 0o644); err != nil {
		return err
	}

	if mcpCount > 0 {
		fmt.Printf("  seeded .claude.json (onboarding flags + %d MCP server%s)\n", mcpCount, pluralS(mcpCount))
	} else {
		fmt.Printf("  seeded .claude.json (onboarding flags)\n")
	}
	return nil
}

func PatchAttribution(profileDir string, attr *Attribution) error {
	if attr == nil {
		return nil
	}

	settingsPath := filepath.Join(profileDir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}

	attrMap := map[string]string{}
	if attr.Commit != "" {
		attrMap["commit"] = attr.Commit
	}
	if attr.PR != "" {
		attrMap["pr"] = attr.PR
	}

	// Check if already matches
	existingJSON, _ := json.Marshal(settings["attribution"])
	newJSON, _ := json.Marshal(attrMap)
	if string(existingJSON) == string(newJSON) {
		return nil
	}

	settings["attribution"] = attrMap

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil {
		return err
	}

	fmt.Println("  patched attribution in settings.json")
	return nil
}

// Tools that depend on Anthropic-hosted server-side execution and are
// silently unsupported by most Anthropic-compatible third-party gateways
// (DeepSeek bridge, Z.ai, GLM, sub2api, etc.). When a profile authenticates
// via ANTHROPIC_BASE_URL + ANTHROPIC_API_KEY we deny them up front so
// Claude Code stops offering / invoking them in that profile.
var apiModeDeniedTools = []string{"WebSearch", "WebFetch"}

// IsAPIModeProfile reports whether a profile authenticates via an
// Anthropic-compatible gateway (ANTHROPIC_BASE_URL / ANTHROPIC_API_KEY in
// profile.env). OAuth profiles return false.
func IsAPIModeProfile(p *Profile) bool {
	if p == nil {
		return false
	}
	for _, k := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_API_KEY"} {
		if v, ok := p.Env[k]; ok && v != "" {
			return true
		}
	}
	return false
}

// DenyUnsupportedTools patches the profile's settings.json so that
// permissions.deny includes WebSearch and WebFetch when this is an
// API-mode profile. No-op on OAuth profiles. Idempotent — does nothing if
// the tools are already in the deny list.
func DenyUnsupportedTools(profileDir string, profile *Profile) error {
	if !IsAPIModeProfile(profile) {
		return nil
	}

	settingsPath := filepath.Join(profileDir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil // settings.json missing; nothing to patch
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}

	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
	}
	rawDeny, _ := perms["deny"].([]any)

	denied := make(map[string]bool, len(rawDeny))
	out := make([]any, 0, len(rawDeny)+len(apiModeDeniedTools))
	for _, v := range rawDeny {
		s, ok := v.(string)
		if !ok || denied[s] {
			continue
		}
		denied[s] = true
		out = append(out, s)
	}

	added := false
	for _, tool := range apiModeDeniedTools {
		if denied[tool] {
			continue
		}
		denied[tool] = true
		out = append(out, tool)
		added = true
	}
	if !added {
		return nil
	}

	perms["deny"] = out
	settings["permissions"] = perms

	pretty, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath, append(pretty, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("  denied unsupported tools for API-mode profile: %s\n", strings.Join(apiModeDeniedTools, ", "))
	return nil
}

type DivergedFile struct {
	Profile  string
	Filename string
	Details  string
}

func CheckDivergence(cfg *Config, profilesBase string) []DivergedFile {
	var diverged []DivergedFile

	for name, profile := range cfg.Profiles {
		profileDir := filepath.Join(profilesBase, name)

		for _, filename := range copyFiles {
			src := filepath.Join(cfg.SourceDir, filename)
			dst := filepath.Join(profileDir, filename)

			srcData, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			dstData, err := os.ReadFile(dst)
			if err != nil {
				continue
			}

			// For settings.json, apply attribution patch before comparing
			expectedData := srcData
			if filename == "settings.json" && profile.Attribution != nil {
				expectedData = applyAttributionToSource(srcData, profile.Attribution)
			}

			// CLAUDE.md may carry the cpm-managed Tavily block — strip it
			// from the profile copy so we compare actual user-authored
			// content against source.
			dstCompare := dstData
			if filename == "CLAUDE.md" {
				dstCompare = []byte(stripTavilyBlock(string(dstData)))
			}

			if string(expectedData) != string(dstCompare) {
				// Check if profile has additions not in source
				srcLines := strings.Split(string(expectedData), "\n")
				dstLines := strings.Split(string(dstCompare), "\n")

				hasAdditions := false
				for _, dl := range dstLines {
					found := false
					for _, sl := range srcLines {
						if dl == sl {
							found = true
							break
						}
					}
					if !found && strings.TrimSpace(dl) != "" {
						hasAdditions = true
						break
					}
				}

				if hasAdditions {
					diverged = append(diverged, DivergedFile{
						Profile:  name,
						Filename: filename,
						Details:  fmt.Sprintf("Profile '%s' — %s has local changes", name, filename),
					})
				}
			}
		}
	}

	return diverged
}

func applyAttributionToSource(data []byte, attr *Attribution) []byte {
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return data
	}

	attrMap := map[string]string{}
	if attr.Commit != "" {
		attrMap["commit"] = attr.Commit
	}
	if attr.PR != "" {
		attrMap["pr"] = attr.PR
	}
	settings["attribution"] = attrMap

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return data
	}
	return append(out, '\n')
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
