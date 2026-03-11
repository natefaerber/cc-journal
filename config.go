package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	// GitHub repos for linking PR references found in transcripts.
	// First repo is used for ambiguous "PR #N" references.
	GitHubRepos []string `yaml:"github_repos"`
}

// Config holds all cc-journal configuration.
type Config struct {
	JournalDir string      `yaml:"journal_dir"`
	PromptDir  string      `yaml:"prompt_dir"`
	Exclude    []string    `yaml:"exclude"`
	Model      string      `yaml:"model"`
	APIKey     string      `yaml:"api_key"`
	Slack      SlackConfig `yaml:"slack"`
	Links      LinksConfig `yaml:"links"`
}

var defaultConfig = Config{
	Model: "claude-sonnet-4-20250514",
}

// configDir returns $XDG_CONFIG_HOME/cc-journal, defaulting to ~/.config/cc-journal.
func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cc-journal")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cc-journal")
}

// configPath returns the path to config.yaml.
func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

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

	// Default prompt dir
	if cfg.PromptDir == "" {
		cfg.PromptDir = filepath.Join(configDir(), "prompts")
	}

	// Apply defaults for empty fields
	if cfg.Model == "" {
		cfg.Model = defaultConfig.Model
	}

	return cfg
}
