package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// runPrune removes failed summary entries from journal files.
// With --dry-run, only reports what would be removed.
func runPrune(dryRun bool) {
	var files []string
	for _, dir := range allJournalDirs() {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		files = append(files, matches...)
	}
	if len(files) == 0 {
		fmt.Println("No journal files found.")
		return
	}

	totalPruned := 0
	totalKept := 0
	filesModified := 0

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		if !strings.Contains(string(content), "Summary generation failed") {
			continue
		}

		sections := splitSections(string(content))
		var kept []section
		pruned := 0

		for _, s := range sections {
			if s.isHeader {
				kept = append(kept, s)
				continue
			}
			if strings.Contains(s.text, "Summary generation failed") {
				pruned++
				if dryRun {
					// Extract the heading for display
					heading := extractHeading(s.text)
					sessionID := extractSessionID(s.text)
					fmt.Printf("  %s: %s [%s]\n", filepath.Base(f), heading, sessionID)
				}
			} else {
				kept = append(kept, s)
			}
		}

		if pruned == 0 {
			continue
		}

		totalPruned += pruned
		totalKept += len(kept) - 1 // subtract header
		filesModified++

		if dryRun {
			continue
		}

		// Rebuild file
		var b strings.Builder
		for i, s := range kept {
			if i > 0 && !s.isHeader {
				b.WriteString("---\n")
			}
			b.WriteString(s.text)
		}

		result := b.String()
		// Clean up trailing separators
		result = strings.TrimRight(result, "\n")
		result += "\n"

		if err := os.WriteFile(f, []byte(result), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to write %s: %v\n", filepath.Base(f), err)
			continue
		}
		fmt.Printf("  %s: removed %d failed entries, kept %d\n", filepath.Base(f), pruned, len(kept)-1)
	}

	if totalPruned == 0 {
		fmt.Println("No failed entries found.")
		return
	}

	if dryRun {
		fmt.Printf("\nWould remove %d failed entries across %d files.\n", totalPruned, filesModified)
		fmt.Println("Run without --dry-run to apply.")
	} else {
		fmt.Printf("\nPruned %d failed entries across %d files.\n", totalPruned, filesModified)
	}
}

type section struct {
	text     string
	isHeader bool
}

// splitSections splits a journal file into its header and entry sections.
func splitSections(content string) []section {
	// Split on "---" lines (the separator between entries)
	parts := regexp.MustCompile(`(?m)^---\s*$`).Split(content, -1)
	var sections []section
	for i, p := range parts {
		if i == 0 {
			// First part is the file header (# Claude Code Journal — ...)
			sections = append(sections, section{text: p, isHeader: true})
		} else {
			sections = append(sections, section{text: p, isHeader: false})
		}
	}
	return sections
}

func extractHeading(text string) string {
	re := regexp.MustCompile(`(?m)^## (.+)$`)
	if m := re.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return "unknown"
}

func extractSessionID(text string) string {
	re := regexp.MustCompile(`<code>([a-f0-9-]{36})</code>`)
	if m := re.FindStringSubmatch(text); len(m) > 1 {
		id := m[1]
		if len(id) > 8 {
			return id[:8]
		}
		return id
	}
	return "?"
}
