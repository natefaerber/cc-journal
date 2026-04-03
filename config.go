package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SlackConfig holds Slack integration settings.
type SlackConfig struct {
	Command string `yaml:"command"`
	Channel string `yaml:"channel"`
}

// LinksConfig holds external link integration settings.
type LinksConfig struct {
	// Issues maps issue key prefixes to base URLs.
	// e.g. "PROJ" → "https://linear.app/your-org/issue"
	// Pattern "PROJ-123" becomes a link to "{url}/PROJ-123"
	Issues map[string]string `yaml:"issues"`

	// Confluence base URL for auto-linking Confluence pages found in transcripts.
	Confluence string `yaml:"confluence"`

	// GitHubDiscover auto-detects GitHub org/repo from session context
	// to resolve bare "PR #N" references.
	GitHubDiscover bool `yaml:"github_discover"`
}

// Profile maps a directory prefix to config overrides.
// Sessions whose cwd starts with Match use the profile's JournalDir.
type Profile struct {
	Match      string      `yaml:"match"`
	JournalDir string      `yaml:"journal_dir"`
	ClaudeDir  string      `yaml:"claude_dir"`
	Links      LinksConfig `yaml:"links"`
}

// Config holds all cc-journal configuration.
type Config struct {
	JournalDir string      `yaml:"journal_dir"`
	ClaudeDir  string      `yaml:"claude_dir"` // path to .claude directory (default: ~/.claude)
	PromptDir  string      `yaml:"prompt_dir"`
	Exclude    []string    `yaml:"exclude"`
	Model      string      `yaml:"model"`
	APIKey     string      `yaml:"api_key"`
	Theme      string      `yaml:"theme"`      // theme name: "warm" (default), "dark", or custom
	WeekStart  string      `yaml:"week_start"` // "monday" (default) or "sunday"
	Slack      SlackConfig `yaml:"slack"`
	Links      LinksConfig `yaml:"links"`
	Profiles   []Profile   `yaml:"profiles"`
}

var defaultConfig = Config{
	Model: "claude-sonnet-4-20250514",
}

// configDir returns the directory containing the config file.
// If --config was provided, returns its parent directory.
// Otherwise uses $XDG_CONFIG_HOME/cc-journal, defaulting to ~/.config/cc-journal.
func configDir() string {
	if configOverride != "" {
		return filepath.Dir(configOverride)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cc-journal")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cc-journal")
}

// configPath returns the path to config.yaml.
// If --config was provided, that path is used instead.
func configPath() string {
	if configOverride != "" {
		return configOverride
	}
	return filepath.Join(configDir(), "config.yaml")
}

// configOverride is set by the --config flag to use a custom config file path.
var configOverride string

// cfg is the global configuration, initialized in main().
var cfg Config

// loadConfig reads config.yaml and applies defaults.
// Environment variables override file values.
func loadConfig() Config {
	cfg := defaultConfig

	data, err := os.ReadFile(configPath())
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", configPath(), err)
		}
	}

	// Expand ~ in paths
	home, _ := os.UserHomeDir()
	if cfg.JournalDir != "" && strings.HasPrefix(cfg.JournalDir, "~/") {
		cfg.JournalDir = filepath.Join(home, cfg.JournalDir[2:])
	}
	if cfg.ClaudeDir != "" && strings.HasPrefix(cfg.ClaudeDir, "~/") {
		cfg.ClaudeDir = filepath.Join(home, cfg.ClaudeDir[2:])
	}
	if cfg.PromptDir != "" && strings.HasPrefix(cfg.PromptDir, "~/") {
		cfg.PromptDir = filepath.Join(home, cfg.PromptDir[2:])
	}
	for i, ex := range cfg.Exclude {
		if strings.HasPrefix(ex, "~/") {
			cfg.Exclude[i] = filepath.Join(home, ex[2:])
		}
	}

	// Env overrides
	if d := os.Getenv("JOURNAL_DIR"); d != "" {
		cfg.JournalDir = d
	}
	if k := os.Getenv("CC_JOURNAL_API_KEY"); k != "" {
		cfg.APIKey = k
	} else if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
		cfg.APIKey = k
	}

	// Default journal dir
	if cfg.JournalDir == "" {
		cfg.JournalDir = filepath.Join(home, "claude-journal")
	}

	// Default claude dir
	if cfg.ClaudeDir == "" {
		cfg.ClaudeDir = filepath.Join(home, ".claude")
	}

	// Default prompt dir
	if cfg.PromptDir == "" {
		cfg.PromptDir = filepath.Join(configDir(), "prompts")
	}

	// Apply defaults for empty fields
	if cfg.Model == "" {
		cfg.Model = defaultConfig.Model
	}

	// Expand ~ in profile paths
	for i := range cfg.Profiles {
		if strings.HasPrefix(cfg.Profiles[i].Match, "~/") {
			cfg.Profiles[i].Match = filepath.Join(home, cfg.Profiles[i].Match[2:])
		}
		if strings.HasPrefix(cfg.Profiles[i].JournalDir, "~/") {
			cfg.Profiles[i].JournalDir = filepath.Join(home, cfg.Profiles[i].JournalDir[2:])
		}
		if strings.HasPrefix(cfg.Profiles[i].ClaudeDir, "~/") {
			cfg.Profiles[i].ClaudeDir = filepath.Join(home, cfg.Profiles[i].ClaudeDir[2:])
		}
	}

	// Normalize week_start
	cfg.WeekStart = strings.ToLower(strings.TrimSpace(cfg.WeekStart))
	if cfg.WeekStart != "sunday" {
		cfg.WeekStart = "monday"
	}

	return cfg
}

// resolveJournalDir returns the journal directory for a given session cwd.
// It checks profiles in order; first match wins. Falls back to the default journal dir.
// resolveProfile returns the best-matching profile for a given cwd.
// The longest (most specific) matching prefix wins, regardless of config order.
// Returns nil if no profile matches.
func resolveProfile(cwd string) *Profile {
	if cwd == "" {
		return nil
	}
	var best *Profile
	bestLen := 0
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.Match != "" && strings.HasPrefix(cwd, p.Match) && len(p.Match) > bestLen {
			best = p
			bestLen = len(p.Match)
		}
	}
	return best
}

// resolveJournalDir returns the journal directory for a given session cwd.
// The longest matching profile prefix wins. Falls back to the default journal dir.
func resolveJournalDir(cwd string) string {
	if p := resolveProfile(cwd); p != nil && p.JournalDir != "" {
		return p.JournalDir
	}
	return cfg.JournalDir
}

// allClaudeDirs returns all unique Claude data directories (default + profiles).
func allClaudeDirs() []string {
	seen := map[string]bool{cfg.ClaudeDir: true}
	dirs := []string{cfg.ClaudeDir}
	for _, p := range cfg.Profiles {
		if p.ClaudeDir != "" && !seen[p.ClaudeDir] {
			seen[p.ClaudeDir] = true
			dirs = append(dirs, p.ClaudeDir)
		}
	}
	return dirs
}

// allProjectsDirs returns all unique projects directories across all claude dirs.
func allProjectsDirs() []string {
	var dirs []string
	for _, cd := range allClaudeDirs() {
		dirs = append(dirs, filepath.Join(cd, "projects"))
	}
	return dirs
}

// allJournalDirs returns all unique journal directories (default + profiles).
func allJournalDirs() []string {
	seen := map[string]bool{cfg.JournalDir: true}
	dirs := []string{cfg.JournalDir}
	for _, p := range cfg.Profiles {
		if p.JournalDir != "" && !seen[p.JournalDir] {
			seen[p.JournalDir] = true
			dirs = append(dirs, p.JournalDir)
		}
	}
	return dirs
}

// weekStartDay returns time.Monday or time.Sunday based on config.
func weekStartDay() time.Weekday {
	if cfg.WeekStart == "sunday" {
		return time.Sunday
	}
	return time.Monday
}

// startOfWeek returns the first day of the week containing t, based on config.
func startOfWeek(t time.Time) time.Time {
	d := t.Weekday()
	start := weekStartDay()
	offset := int(d) - int(start)
	if offset < 0 {
		offset += 7
	}
	return t.AddDate(0, 0, -offset)
}
