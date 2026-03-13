package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
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
cc-journal summarize $ARGUMENTS --force
` + "```" + `

If no session ID is provided as an argument, it will find the most recent session for the current working directory.

After running, show the user the output. If the command succeeds, let them know the summary was written to their journal.
`,
}

func runInitCLI(cmd *cli.Command) {
	doTemplates := cmd.Bool("templates") || cmd.Bool("all")
	doPrompts := cmd.Bool("prompts") || cmd.Bool("all")
	doCommands := cmd.Bool("commands") || cmd.Bool("all")
	force := cmd.Bool("force")
	toStdout := cmd.Bool("stdout")

	if !doTemplates && !doPrompts && !doCommands {
		doTemplates = true
		doPrompts = true
		doCommands = true
	}

	if toStdout {
		initStdout(doTemplates, doPrompts, doCommands)
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
}

// initStdout prints templates and/or prompts to stdout, separated by filename headers.
func initStdout(doTemplates, doPrompts, doCommands bool) {
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
}
