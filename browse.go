package main

import (
	"encoding/json"
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

// showDate prints a specific date's journal entries to stdout from all journal dirs.
func showDate(date string) {
	found := false
	for _, dir := range allJournalDirs() {
		file := filepath.Join(dir, date+".md")
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		found = true
		fmt.Print(string(content))
	}
	if !found {
		fmt.Fprintf(os.Stderr, "No journal entries for %s.\n", date)
		os.Exit(1)
	}
}

// listEntries prints all journal files with sizes from all journal dirs.
func listEntries() {
	type fileInfo struct {
		name string
		size int64
	}
	seen := make(map[string]fileInfo)

	for _, dir := range allJournalDirs() {
		files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		for _, f := range files {
			name := strings.TrimSuffix(filepath.Base(f), ".md")
			info, _ := os.Stat(f)
			if info != nil {
				if existing, ok := seen[name]; ok {
					seen[name] = fileInfo{name: name, size: existing.size + info.Size()}
				} else {
					seen[name] = fileInfo{name: name, size: info.Size()}
				}
			}
		}
	}

	if len(seen) == 0 {
		fmt.Fprintln(os.Stderr, "No journal entries yet.")
		os.Exit(1)
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("%s  (%d bytes)\n", n, seen[n].size)
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
		for _, dir := range allJournalDirs() {
			file := filepath.Join(dir, date+".md")
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			found = true
			fmt.Print(string(content))
			fmt.Println()
		}
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

	// Collect daily contents from all journal dirs
	var combined []string
	for i := 0; i < 7; i++ {
		day := monday.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		for _, dir := range allJournalDirs() {
			file := filepath.Join(dir, date+".md")
			content, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			combined = append(combined, fmt.Sprintf("## %s\n%s", date, string(content)))
		}
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

	// Save rollup to the default journal dir (rollups aggregate across all profiles)
	_, week := monday.ISOWeek()
	rollupFile := filepath.Join(cfg.JournalDir, fmt.Sprintf("%d-W%02d-rollup.md", monday.Year(), week))
	if err := os.WriteFile(rollupFile, []byte(summary+"\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write rollup: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(summary)
	fmt.Printf("\nSaved to %s\n", rollupFile)
}

// entriesForDate returns parsed journal entries for a specific date.
func entriesForDate(date string) []Entry {
	data := parseJournalFiles()
	var result []Entry
	for _, e := range data.Entries {
		if e.Date == date {
			result = append(result, e)
		}
	}
	return result
}

// showDateJSON outputs a date's entries as JSON.
func showDateJSON(date string) {
	entries := entriesForDate(date)
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "No journal entries for %s.\n", date)
		os.Exit(1)
	}
	out := struct {
		Date    string  `json:"date"`
		Entries []Entry `json:"entries"`
	}{Date: date, Entries: entries}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// listEntriesJSON outputs the journal file list as JSON.
func listEntriesJSON() {
	type fileEntry struct {
		Date       string `json:"date"`
		Size       int64  `json:"size"`
		EntryCount int    `json:"entry_count"`
	}

	sizeMap := make(map[string]int64)
	for _, dir := range allJournalDirs() {
		files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		for _, f := range files {
			name := strings.TrimSuffix(filepath.Base(f), ".md")
			if strings.Contains(name, "rollup") {
				continue
			}
			info, _ := os.Stat(f)
			if info != nil {
				sizeMap[name] += info.Size()
			}
		}
	}

	if len(sizeMap) == 0 {
		fmt.Fprintln(os.Stderr, "No journal entries yet.")
		os.Exit(1)
	}

	// Count entries per date
	data := parseJournalFiles()
	countMap := make(map[string]int)
	for _, e := range data.Entries {
		countMap[e.Date]++
	}

	names := make([]string, 0, len(sizeMap))
	for n := range sizeMap {
		names = append(names, n)
	}
	sort.Strings(names)

	entries := make([]fileEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, fileEntry{
			Date:       n,
			Size:       sizeMap[n],
			EntryCount: countMap[n],
		})
	}

	out := struct {
		Files []fileEntry `json:"files"`
	}{Files: entries}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// showStats prints human-readable journal statistics.
func showStats() {
	data := parseJournalFiles()
	stats := computeStats(data)
	fmt.Printf("Sessions:  %d\n", stats.TotalSessions)
	fmt.Printf("Days:      %d\n", stats.TotalDays)
	fmt.Printf("Projects:  %d\n", stats.TotalProjects)
	fmt.Printf("This week: %d (since %s)\n", stats.ThisWeek, stats.WeekStartLabel)
	fmt.Printf("Streak:    %d day(s)\n", stats.Streak)
	fmt.Printf("Most active: %s\n", stats.MostActive)
}

// showStatsJSON outputs journal statistics as JSON.
func showStatsJSON() {
	data := parseJournalFiles()
	stats := computeStats(data)
	activity := computeActivity(data.Entries, 28)

	// Build activity array with just date and count
	type activityDay struct {
		Date  string `json:"date"`
		Count int    `json:"count"`
	}
	days := make([]activityDay, 0, len(activity))
	for _, b := range activity {
		days = append(days, activityDay{Date: b.Date, Count: b.Count})
	}

	out := struct {
		Stats
		Activity []activityDay `json:"activity"`
	}{Stats: stats, Activity: days}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
