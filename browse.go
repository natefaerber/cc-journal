package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// showToday prints today's journal entries to stdout.
func showToday() {
	showDate(time.Now().Format("2006-01-02"))
}

// showDate prints a specific date's journal entries to stdout.
func showDate(date string) {
	file := filepath.Join(journalDir(), date+".md")
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No journal entries for %s.\n", date)
		os.Exit(1)
	}
	fmt.Print(string(content))
}

// listEntries prints all journal files with sizes.
func listEntries() {
	dir := journalDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No journal directory found.")
		os.Exit(1)
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No journal entries yet.")
		os.Exit(1)
	}

	sort.Strings(files)
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		info, _ := os.Stat(f)
		if info != nil {
			fmt.Printf("%s  (%d bytes)\n", name, info.Size())
		}
	}
}

// showWeek prints the current week's journal entries.
func showWeek(targetDate string) {
	target := time.Now()
	if targetDate != "" {
		var err error
		target, err = time.Parse("2006-01-02", targetDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid date: %s\n", targetDate)
			os.Exit(1)
		}
	}

	// Find first day of the week
	ws := startOfWeek(target)

	fmt.Printf("Week of %s\n\n", ws.Format("2006-01-02"))

	found := false
	for i := 0; i < 7; i++ {
		day := ws.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		file := filepath.Join(journalDir(), date+".md")
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		found = true
		fmt.Print(string(content))
		fmt.Println()
	}

	if !found {
		fmt.Fprintf(os.Stderr, "No journal entries for week of %s.\n", ws.Format("2006-01-02"))
	}
}

// generateRollup creates an AI-powered weekly rollup.
func generateRollup(targetDate string) {
	target := time.Now()
	if targetDate != "" {
		var err error
		target, err = time.Parse("2006-01-02", targetDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid date: %s\n", targetDate)
			os.Exit(1)
		}
	}

	monday := startOfWeek(target)

	// Collect daily contents
	var combined []string
	for i := 0; i < 7; i++ {
		day := monday.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		file := filepath.Join(journalDir(), date+".md")
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		combined = append(combined, fmt.Sprintf("## %s\n%s", date, string(content)))
	}

	if len(combined) == 0 {
		fmt.Fprintf(os.Stderr, "No journal entries for week of %s.\n", monday.Format("2006-01-02"))
		os.Exit(1)
	}

	apiKey, err := getAPIKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	allContent := strings.Join(combined, "\n\n")
	if len(allContent) > maxTranscriptChars {
		allContent = allContent[:maxTranscriptChars]
	}

	prompt := strings.NewReplacer(
		"{{.Week}}", monday.Format("2006-01-02"),
		"{{.Content}}", allContent,
	).Replace(loadPrompt("rollup"))

	fmt.Println("Generating weekly rollup...")

	summary, err := callAnthropicAPIRaw(apiKey, prompt, 2048)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate rollup: %v\n", err)
		os.Exit(1)
	}

	// Save to file
	_, week := monday.ISOWeek()
	rollupFile := filepath.Join(journalDir(), fmt.Sprintf("%d-W%02d-rollup.md", monday.Year(), week))
	os.WriteFile(rollupFile, []byte(summary+"\n"), 0o644)

	fmt.Println(summary)
	fmt.Printf("\nSaved to %s\n", rollupFile)
}
