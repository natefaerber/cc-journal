package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

// builtinPrompts maps prompt names to their default content.
var builtinPrompts = map[string]string{
	"summary": defaultPromptTemplate,
	"rollup":  defaultRollupTemplate,
	"standup": defaultStandupTemplate,
	"weekly":  defaultWeeklyTemplate,
}

// builtinCommands maps Claude Code slash command names to their content.
var builtinCommands = map[string]string{
	"summarize": `Summarize the current Claude Code session and write it to the journal.

Run the following command to generate a summary of this session:

` + "```" + `
cc-journal summarize $ARGUMENTS
` + "```" + `

If no session ID is provided as an argument, it will find the most recent session for the current working directory.

If the command reports the session is already journaled, ask the user if they want to replace the existing summary. If they confirm, re-run with --force:

` + "```" + `
cc-journal summarize $ARGUMENTS --force
` + "```" + `

After running, show the user the output. If the command succeeds, let them know the summary was written to their journal.
`,
}

const defaultConfigYAML = `# cc-journal configuration
# See: https://github.com/natefaerber/cc-journal

# Journal storage directory
# journal_dir: ~/claude-journal

# AI model for summarization
# model: claude-sonnet-4-20250514

# API key (prefer ANTHROPIC_API_KEY env var instead)
# api_key: sk-ant-...

# Theme: warm (default), dark, or a custom theme name
# theme: warm

# Week start day: monday (default) or sunday
# week_start: monday

# Directories to exclude from summarization
# exclude:
#   - ~/private-project

# Custom prompt templates directory
# prompt_dir: ~/.config/cc-journal/prompts

# Slack integration
# slack:
#   command: slack-send
#   channel: "#standup"

# External link detection
# links:
#   issues:
#     LPE: https://linear.app/your-org/issue
#   confluence: https://your-org.atlassian.net/wiki
#   github_repos:
#     - https://github.com/your-org/your-repo  # first repo used for bare "PR #N" links
`

func runInitCLI(cmd *cli.Command) {
	doTemplates := cmd.Bool("templates") || cmd.Bool("all")
	doPrompts := cmd.Bool("prompts") || cmd.Bool("all")
	doCommands := cmd.Bool("commands") || cmd.Bool("all")
	doThemes := cmd.Bool("themes") || cmd.Bool("all")
	doConfig := cmd.Bool("config")
	force := cmd.Bool("force")
	toStdout := cmd.Bool("stdout")

	if !doTemplates && !doPrompts && !doCommands && !doThemes && !doConfig {
		doTemplates = true
		doPrompts = true
		doCommands = true
		doThemes = true
	}

	if toStdout {
		initStdout(doTemplates, doPrompts, doCommands, doThemes, doConfig)
		return
	}

	writeFile := func(path string, data []byte) {
		if !force {
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("  exists: %s (use --force to overwrite)\n", path)
				return
			}
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
			return
		}
		fmt.Printf("  wrote:  %s\n", path)
	}

	if doConfig {
		cfgPath := configPath()
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create config dir: %v\n", err)
			return
		}
		writeFile(cfgPath, []byte(defaultConfigYAML))
	}

	if doPrompts {
		if err := os.MkdirAll(cfg.PromptDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create prompt dir: %v\n", err)
			return
		}
		for name, content := range builtinPrompts {
			writeFile(filepath.Join(cfg.PromptDir, name+".txt"), []byte(content))
		}
		fmt.Printf("\nPrompts dir: %s\n", cfg.PromptDir)
	}

	if doTemplates {
		cfgDir := configDir()
		templatesDir := filepath.Join(cfgDir, "templates")
		if err := os.MkdirAll(templatesDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create templates dir: %v\n", err)
			return
		}

		embeddedFS, _ := fs.Sub(embeddedTemplates, "templates")
		entries, _ := fs.ReadDir(embeddedFS, ".")
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := fs.ReadFile(embeddedFS, e.Name())
			if err != nil {
				continue
			}
			writeFile(filepath.Join(templatesDir, e.Name()), data)
		}
		fmt.Printf("\nTo use custom templates:\n")
		fmt.Printf("  cc-journal serve --templates %s\n", templatesDir)
	}

	if doCommands {
		home, _ := os.UserHomeDir()
		commandsDir := filepath.Join(home, ".claude", "commands")
		if err := os.MkdirAll(commandsDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create commands dir: %v\n", err)
			return
		}
		for name, content := range builtinCommands {
			writeFile(filepath.Join(commandsDir, name+".md"), []byte(content))
		}
		fmt.Printf("\nClaude Code commands: %s\n", commandsDir)
		fmt.Println("  Use /summarize in Claude Code to journal the current session.")
	}

	if doThemes {
		themesDir := filepath.Join(configDir(), "themes")
		if err := os.MkdirAll(themesDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create themes dir: %v\n", err)
			return
		}
		for name, theme := range builtinThemes {
			data, err := yaml.Marshal(theme)
			if err != nil {
				continue
			}
			writeFile(filepath.Join(themesDir, name+".yaml"), data)
		}
		fmt.Printf("\nThemes dir: %s\n", themesDir)
		fmt.Println("  Set theme in config.yaml: theme: dark")
		fmt.Println("  Or create a custom theme YAML in the themes directory.")
	}
}

// initStdout prints templates and/or prompts to stdout, separated by filename headers.
func initStdout(doTemplates, doPrompts, doCommands, doThemes, doConfig bool) {
	if doConfig {
		fmt.Print("=== config.yaml ===\n")
		fmt.Print(defaultConfigYAML)
	}

	if doPrompts {
		for name, content := range builtinPrompts {
			fmt.Printf("=== prompt: %s.txt ===\n", name)
			fmt.Print(content)
			if len(content) > 0 && content[len(content)-1] != '\n' {
				fmt.Println()
			}
		}
	}

	if doTemplates {
		embeddedFS, _ := fs.Sub(embeddedTemplates, "templates")
		entries, _ := fs.ReadDir(embeddedFS, ".")
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := fs.ReadFile(embeddedFS, e.Name())
			if err != nil {
				continue
			}
			fmt.Printf("=== template: %s ===\n", e.Name())
			_, _ = os.Stdout.Write(data)
			if len(data) > 0 && data[len(data)-1] != '\n' {
				fmt.Println()
			}
		}
	}

	if doCommands {
		for name, content := range builtinCommands {
			fmt.Printf("=== command: %s.md ===\n", name)
			fmt.Print(content)
			if len(content) > 0 && content[len(content)-1] != '\n' {
				fmt.Println()
			}
		}
	}

	if doThemes {
		for name, theme := range builtinThemes {
			data, err := yaml.Marshal(theme)
			if err != nil {
				continue
			}
			fmt.Printf("=== theme: %s.yaml ===\n", name)
			_, _ = os.Stdout.Write(data)
			fmt.Println()
		}
	}
}
