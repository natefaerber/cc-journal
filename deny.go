package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const denyFile = ".denied"

// denyPath returns the path to the deny list file.
func denyPath() string {
	return filepath.Join(journalDir(), denyFile)
}

// loadDenied reads the deny list and returns a set of session IDs.
func loadDenied() map[string]bool {
	ids := make(map[string]bool)
	data, err := os.ReadFile(denyPath())
	if err != nil {
		return ids
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids[line] = true
	}
	return ids
}

// isDenied checks if a session ID is in the deny list.
func isDenied(sessionID string) bool {
	return loadDenied()[sessionID]
}

// addToDenyList appends a session ID to the deny list file.
func addToDenyList(sessionID string) error {
	if isDenied(sessionID) {
		return nil
	}

	// Create file with header if new
	if _, err := os.Stat(denyPath()); os.IsNotExist(err) {
		header := "# Session IDs excluded from backfill and hook processing.\n# Remove a line to allow re-summarization.\n\n"
		if err := os.WriteFile(denyPath(), []byte(header), 0o644); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(denyPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintln(f, sessionID)
	return err
}

// removeFromJournal deletes a session entry from all journal files.
// Returns the number of entries removed.
func removeFromJournal(sessionID string) int {
	dir := journalDir()
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	idPattern := regexp.MustCompile(`<code>` + regexp.QuoteMeta(sessionID) + `</code>`)
	removed := 0

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if !idPattern.Match(content) {
			continue
		}

		sections := splitSections(string(content))
		var kept []section
		for _, s := range sections {
			if s.isHeader {
				kept = append(kept, s)
				continue
			}
			if idPattern.MatchString(s.text) {
				removed++
			} else {
				kept = append(kept, s)
			}
		}

		if removed > 0 {
			var b strings.Builder
			for i, s := range kept {
				if i > 0 && !s.isHeader {
					b.WriteString("---\n")
				}
				b.WriteString(s.text)
			}
			result := strings.TrimRight(b.String(), "\n") + "\n"
			_ = os.WriteFile(f, []byte(result), 0o644)
		}
	}

	return removed
}

// replaceWithStub replaces a session's entry with a redirect stub pointing to newDate.
// Returns the date of the file the entry was found in, or "" if not found.
func replaceWithStub(sessionID, newDate string) string {
	dir := journalDir()
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	idPattern := regexp.MustCompile(`<code>` + regexp.QuoteMeta(sessionID) + `</code>`)

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if !idPattern.Match(content) {
			continue
		}

		oldDate := strings.TrimSuffix(filepath.Base(f), ".md")

		// Same day — just remove, caller will write the replacement
		if oldDate == newDate {
			removeFromJournal(sessionID)
			return oldDate
		}

		// Different day — replace entry with a redirect stub
		sections := splitSections(string(content))
		var result []section
		for _, s := range sections {
			if s.isHeader {
				result = append(result, s)
				continue
			}
			if idPattern.MatchString(s.text) {
				// Extract the heading line to preserve project/branch/time
				heading := extractHeading(s.text)
				stub := fmt.Sprintf(`

## %s

*Re-summarized → [%s](/daily/%s#%s)*

<details>
<summary>Session ID</summary>
<code>%s</code>
</details>

`, heading, newDate, newDate, sessionID, sessionID)
				result = append(result, section{text: stub, isHeader: false})
			} else {
				result = append(result, s)
			}
		}

		var b strings.Builder
		for i, s := range result {
			if i > 0 && !s.isHeader {
				b.WriteString("---")
			}
			b.WriteString(s.text)
		}
		out := strings.TrimRight(b.String(), "\n") + "\n"
		_ = os.WriteFile(f, []byte(out), 0o644)

		return oldDate
	}
	return ""
}

// runRemove removes a session entry and adds it to the deny list.
func runRemove(sessionID string) {
	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "Usage: cc-journal remove SESSION_ID")
		os.Exit(1)
	}

	removed := removeFromJournal(sessionID)
	if removed > 0 {
		fmt.Printf("Removed %d entry/entries for session %s.\n", removed, sessionID[:min(8, len(sessionID))])
	} else {
		fmt.Printf("No journal entry found for session %s.\n", sessionID[:min(8, len(sessionID))])
	}

	if err := addToDenyList(sessionID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update deny list: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added to %s (excluded from future backfills).\n", denyPath())
}
