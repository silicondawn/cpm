package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel markers wrapping the cpm-managed Tavily section inside a
// profile's CLAUDE.md so we can idempotently insert / update / remove it.
const (
	tavilyHintStart = "<!-- cpm:tavily-start -->"
	tavilyHintEnd   = "<!-- cpm:tavily-end -->"
)

// Self-contained Tavily usage hint. Intentionally depends on NOTHING
// beyond the TAVILY_API_KEY env var: no skill, no plugin, no MCP server,
// no CLI — just curl. So this works for a fresh user who scoop-installed
// cpm with no other Claude Code customization.
const tavilyHintBody = `## Web Search via Tavily (substitute for WebSearch / WebFetch)

This profile authenticates via an Anthropic-compatible API gateway, so
Claude's native ` + "`WebSearch`" + ` and ` + "`WebFetch`" + ` tools are denied (see
` + "`permissions.deny`" + ` in this profile's ` + "`settings.json`" + `). When the user
needs web search or page extraction, fall back to the **Tavily HTTP API**
using the ` + "`TAVILY_API_KEY`" + ` already exported into your environment.

### Search (text query → ranked results with snippets)

` + "```bash" + `
curl -sS -X POST https://api.tavily.com/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TAVILY_API_KEY" \
  -d '{"query": "your search here", "max_results": 5, "search_depth": "advanced"}'
` + "```" + `

### Extract content from one or more URLs (substitute for WebFetch)

` + "```bash" + `
curl -sS -X POST https://api.tavily.com/extract \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TAVILY_API_KEY" \
  -d '{"urls": ["https://example.com/page"]}'
` + "```" + `

Both endpoints return JSON — pipe through ` + "`jq`" + ` to extract fields. The
free tier includes 1000 credits/month; ` + "`search_depth: \"basic\"`" + ` costs 1
credit, ` + "`\"advanced\"`" + ` costs 2.`

// profileHasTavilyKey reports whether profile has a non-empty
// TAVILY_API_KEY in its env (the signal that the hint should be present).
func profileHasTavilyKey(p *Profile) bool {
	if p == nil {
		return false
	}
	v, ok := p.Env["TAVILY_API_KEY"]
	return ok && strings.TrimSpace(v) != ""
}

// InjectTavilyHint maintains the cpm-managed Tavily block inside a
// profile's CLAUDE.md:
//   - if profile has TAVILY_API_KEY in env, insert the block (or replace
//     an existing one so updates roll forward)
//   - if profile lacks TAVILY_API_KEY, remove the block if present
//
// Idempotent. Marker-delimited so user-authored content around the block
// is preserved.
func InjectTavilyHint(profileDir string, profile *Profile) error {
	enabled := profileHasTavilyKey(profile)

	path := filepath.Join(profileDir, "CLAUDE.md")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := string(raw)
	hasMarker := strings.Contains(existing, tavilyHintStart)

	if !enabled {
		if !hasMarker {
			return nil
		}
		updated := stripTavilyBlock(existing)
		// If our block was the only content (source had no CLAUDE.md so
		// SetupProfile never copied one), delete the empty file rather
		// than leaving a 0-byte artifact behind.
		if strings.TrimSpace(updated) == "" {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Println("  removed Tavily WebSearch hint from CLAUDE.md (now empty, deleted)")
			return nil
		}
		return writeCLAUDEIfChanged(path, existing, updated, "removed Tavily WebSearch hint from CLAUDE.md")
	}

	block := tavilyHintStart + "\n" + tavilyHintBody + "\n" + tavilyHintEnd
	var updated string
	if hasMarker {
		updated = replaceTavilyBlock(existing, block)
	} else {
		base := strings.TrimRight(existing, "\n")
		if base == "" {
			updated = block + "\n"
		} else {
			updated = base + "\n\n" + block + "\n"
		}
	}
	return writeCLAUDEIfChanged(path, existing, updated, "patched Tavily WebSearch hint in CLAUDE.md")
}

// stripTavilyBlock removes the cpm-managed Tavily section (and its
// surrounding blank line, if any) from s. Returns s unchanged if there
// is no start marker.
func stripTavilyBlock(s string) string {
	startIdx := strings.Index(s, tavilyHintStart)
	if startIdx < 0 {
		return s
	}
	endIdx := strings.Index(s[startIdx:], tavilyHintEnd)
	if endIdx < 0 {
		// Malformed (start without end) — strip from start to EOF.
		before := strings.TrimRight(s[:startIdx], "\n")
		if before == "" {
			return ""
		}
		return before + "\n"
	}
	endIdx += startIdx + len(tavilyHintEnd)
	for endIdx < len(s) && (s[endIdx] == '\n' || s[endIdx] == '\r') {
		endIdx++
	}
	before := strings.TrimRight(s[:startIdx], "\n")
	after := s[endIdx:]
	switch {
	case before == "" && after == "":
		return ""
	case before == "":
		return after
	case after == "":
		return before + "\n"
	default:
		return before + "\n\n" + after
	}
}

func replaceTavilyBlock(s, newBlock string) string {
	startIdx := strings.Index(s, tavilyHintStart)
	if startIdx < 0 {
		return s
	}
	endIdx := strings.Index(s[startIdx:], tavilyHintEnd)
	if endIdx < 0 {
		return s
	}
	endIdx += startIdx + len(tavilyHintEnd)
	return s[:startIdx] + newBlock + s[endIdx:]
}

func writeCLAUDEIfChanged(path, before, after, msg string) error {
	if before == after {
		return nil
	}
	if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
		return err
	}
	fmt.Println("  " + msg)
	return nil
}
