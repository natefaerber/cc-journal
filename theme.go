package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Theme defines all visual tokens for the UI.
type Theme struct {
	Name   string      `yaml:"name"`
	Colors ThemeColors `yaml:"colors"`
	Fonts  ThemeFonts  `yaml:"fonts"`
}

// ThemeColors maps to CSS custom properties in the @theme block.
type ThemeColors struct {
	Background       string `yaml:"background"`
	Foreground       string `yaml:"foreground"`
	Card             string `yaml:"card"`
	CardForeground   string `yaml:"card_foreground"`
	Muted            string `yaml:"muted"`
	MutedForeground  string `yaml:"muted_foreground"`
	Border           string `yaml:"border"`
	Accent           string `yaml:"accent"`
	AccentForeground string `yaml:"accent_foreground"`
	Secondary        string `yaml:"secondary"`
	Warning          string `yaml:"warning"`
	Ring             string `yaml:"ring"`
	Danger           string `yaml:"danger"`
	Activity1        string `yaml:"activity_1"`
	Activity2        string `yaml:"activity_2"`
	Activity3        string `yaml:"activity_3"`
	Activity4        string `yaml:"activity_4"`
}

// ThemeFonts defines font families and the Google Fonts import URL.
type ThemeFonts struct {
	Body      string `yaml:"body"`
	Sans      string `yaml:"sans"`
	Mono      string `yaml:"mono"`
	GoogleURL string `yaml:"google_url"`
}

var builtinThemes = map[string]Theme{
	"warm": {
		Name: "warm",
		Colors: ThemeColors{
			Background:       "#f5f0e8",
			Foreground:       "#1a1a1a",
			Card:             "#fffdf7",
			CardForeground:   "#1a1a1a",
			Muted:            "#ece7dc",
			MutedForeground:  "#78716c",
			Border:           "#d6cfc4",
			Accent:           "#2563eb",
			AccentForeground: "#ffffff",
			Secondary:        "#7c3aed",
			Warning:          "#92400e",
			Ring:             "#2563eb",
			Danger:           "#ef4444",
			Activity1:        "#dbeafe",
			Activity2:        "#93c5fd",
			Activity3:        "#3b82f6",
			Activity4:        "#2563eb",
		},
		Fonts: ThemeFonts{
			Body:      `"Source Serif 4", Georgia, "Times New Roman", serif`,
			Sans:      `-apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif`,
			Mono:      `"SF Mono", "Fira Code", "Cascadia Code", "JetBrains Mono", monospace`,
			GoogleURL: "https://fonts.googleapis.com/css2?family=Source+Serif+4:ital,opsz,wght@0,8..60,400;0,8..60,600;0,8..60,700;1,8..60,400&display=swap",
		},
	},
	"dark": {
		Name: "dark",
		Colors: ThemeColors{
			Background:       "#0f0f0f",
			Foreground:       "#e0e0e0",
			Card:             "#1a1a1a",
			CardForeground:   "#e0e0e0",
			Muted:            "#262626",
			MutedForeground:  "#888888",
			Border:           "#333333",
			Accent:           "#60a5fa",
			AccentForeground: "#0f0f0f",
			Secondary:        "#a78bfa",
			Warning:          "#fbbf24",
			Ring:             "#60a5fa",
			Danger:           "#f87171",
			Activity1:        "#1e3a5f",
			Activity2:        "#2563eb",
			Activity3:        "#3b82f6",
			Activity4:        "#60a5fa",
		},
		Fonts: ThemeFonts{
			Body:      `"Source Serif 4", Georgia, "Times New Roman", serif`,
			Sans:      `-apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif`,
			Mono:      `"SF Mono", "Fira Code", "Cascadia Code", "JetBrains Mono", monospace`,
			GoogleURL: "https://fonts.googleapis.com/css2?family=Source+Serif+4:ital,opsz,wght@0,8..60,400;0,8..60,600;0,8..60,700;1,8..60,400&display=swap",
		},
	},
}

// currentTheme is the active theme, set at startup and on SIGHUP.
var currentTheme = builtinThemes["warm"]

// loadTheme loads a theme by name: checks user dir first, then built-ins.
func loadTheme(name string) (Theme, error) {
	if name == "" {
		name = "warm"
	}

	// Check user theme dir
	themePath := filepath.Join(configDir(), "themes", name+".yaml")
	if data, err := os.ReadFile(themePath); err == nil {
		var t Theme
		if err := yaml.Unmarshal(data, &t); err != nil {
			return Theme{}, fmt.Errorf("parsing theme %s: %w", themePath, err)
		}
		return t, nil
	}

	// Fall back to built-in
	if t, ok := builtinThemes[name]; ok {
		return t, nil
	}

	return Theme{}, fmt.Errorf("theme %q not found (available: warm, dark)", name)
}

// listThemes returns all available theme names (built-in + user).
func listThemes() []string {
	seen := make(map[string]bool)
	var names []string

	// Built-in themes
	for name := range builtinThemes {
		seen[name] = true
		names = append(names, name)
	}

	// User themes
	themesDir := filepath.Join(configDir(), "themes")
	entries, _ := os.ReadDir(themesDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}
		name = name[:len(name)-5]
		if !seen[name] {
			names = append(names, name)
		}
	}

	sort.Strings(names)
	return names
}

// CSS returns the @theme CSS variable block.
func (t Theme) CSS() template.CSS {
	return template.CSS(fmt.Sprintf(`--color-background: %s;
      --color-foreground: %s;
      --color-card: %s;
      --color-card-foreground: %s;
      --color-muted: %s;
      --color-muted-foreground: %s;
      --color-border: %s;
      --color-accent: %s;
      --color-accent-foreground: %s;
      --color-secondary: %s;
      --color-warning: %s;
      --color-ring: %s;
      --color-danger: %s;
      --color-activity-1: %s;
      --color-activity-2: %s;
      --color-activity-3: %s;
      --color-activity-4: %s;`,
		t.Colors.Background, t.Colors.Foreground, t.Colors.Card, t.Colors.CardForeground,
		t.Colors.Muted, t.Colors.MutedForeground, t.Colors.Border,
		t.Colors.Accent, t.Colors.AccentForeground, t.Colors.Secondary,
		t.Colors.Warning, t.Colors.Ring, t.Colors.Danger,
		t.Colors.Activity1, t.Colors.Activity2, t.Colors.Activity3, t.Colors.Activity4))
}
