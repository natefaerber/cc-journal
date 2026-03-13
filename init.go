package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// builtinPrompts maps prompt names to their default content.
var builtinPrompts = map[string]string{
	"summary": defaultPromptTemplate,
	"rollup":  defaultRollupTemplate,
	"standup": defaultStandupTemplate,
	"weekly":  defaultWeeklyTemplate,
}

func runInit(args []string) {
	doTemplates := hasFlag(args, "--templates") || hasFlag(args, "--all")
	doPrompts := hasFlag(args, "--prompts") || hasFlag(args, "--all")
	force := hasFlag(args, "--force")
	toStdout := hasFlag(args, "--stdout")

	if !doTemplates && !doPrompts {
		doTemplates = true
		doPrompts = true
	}

	if toStdout {
		initStdout(doTemplates, doPrompts)
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
}

// initStdout prints templates and/or prompts to stdout, separated by filename headers.
func initStdout(doTemplates, doPrompts bool) {
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
}
