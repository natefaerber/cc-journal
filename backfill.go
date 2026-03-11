package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// runBackfill retroactively summarizes existing Claude Code sessions.
func runBackfill(days int, dryRun bool) {
	home, _ := os.UserHomeDir()
	projectsDir := filepath.Join(home, ".claude", "projects")

	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No Claude Code projects found.")
		os.Exit(1)
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Find all session JSONL files
	var sessionFiles []struct {
		path    string
		modTime time.Time
	}

	dirs, _ := os.ReadDir(projectsDir)
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(projectsDir, d.Name(), "*.jsonl"))
		for _, f := range files {
			if strings.Contains(f, "subagent") {
				continue
			}
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				continue
			}
			sessionFiles = append(sessionFiles, struct {
				path    string
				modTime time.Time
			}{f, info.ModTime()})
		}
	}

	sort.Slice(sessionFiles, func(i, j int) bool {
		return sessionFiles[i].modTime.Before(sessionFiles[j].modTime)
	})

	fmt.Printf("Found %d session files from the last %d days.\n\n", len(sessionFiles), days)

	// Collect existing session IDs from journal and deny list
	existingIDs := collectExistingIDs()
	denied := loadDenied()

	skipped := 0
	summarized := 0

	var apiKey string
	if !dryRun {
		var err error
		apiKey, err = getAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	for _, sf := range sessionFiles {
		sessionID := strings.TrimSuffix(filepath.Base(sf.path), ".jsonl")
		if existingIDs[sessionID] || denied[sessionID] {
			skipped++
			continue
		}

		meta, err := parseTranscript(sf.path)
		if err != nil {
			fmt.Printf("  %s: parse error: %v\n", sessionID[:8], err)
			continue
		}
		if len(meta.Messages) == 0 {
			fmt.Printf("  %s: no messages, skipping\n", sessionID[:8])
			continue
		}

		// Get first user message for display
		firstUserMsg := ""
		for _, m := range meta.Messages {
			if m.Role == "user" {
				firstUserMsg = m.Text
				if len(firstUserMsg) > 100 {
					firstUserMsg = firstUserMsg[:100]
				}
				break
			}
		}

		fmt.Printf("  %s  %s (%s)  %s...\n", sessionID[:8], meta.Project, meta.GitBranch, firstUserMsg)

		if dryRun {
			continue
		}

		// Extract external links
		meta.Links = extractLinksFromTranscript(meta.Messages)
		for _, m := range meta.Messages {
			if issueLinks := extractIssueKeysFromText(m.Text); len(issueLinks) > 0 {
				meta.Links = deduplicateLinks(meta.Links, issueLinks)
			}
		}

		transcript := buildTranscriptText(meta.Messages)
		summary, err := callAnthropicAPI(apiKey, transcript, meta.Project, meta.GitBranch)
		if err != nil {
			fmt.Printf("    Failed: %v\n", err)
			continue
		}

		// Determine journal date from first timestamp
		journalDate := time.Now().Format("2006-01-02")
		if meta.FirstTime != "" {
			if t, err := time.Parse(time.RFC3339Nano, meta.FirstTime); err == nil {
				journalDate = t.Local().Format("2006-01-02")
			}
		}

		// Write to the correct date's journal
		dir := journalDir()
		os.MkdirAll(dir, 0o755)
		journalFile := filepath.Join(dir, journalDate+".md")

		if _, err := os.Stat(journalFile); os.IsNotExist(err) {
			os.WriteFile(journalFile, []byte(fmt.Sprintf("# Claude Code Journal — %s\n\n", journalDate)), 0o644)
		}

		branch := meta.GitBranch
		if branch == "" {
			branch = "n/a"
		}
		cwdLine := ""
		if meta.CWD != "" {
			cwdLine = fmt.Sprintf("\n<code>%s</code>", meta.CWD)
		}

		timeRange := "unknown"
		if meta.FirstTime != "" && meta.LastTime != "" {
			if st, err := time.Parse(time.RFC3339Nano, meta.FirstTime); err == nil {
				if et, err := time.Parse(time.RFC3339Nano, meta.LastTime); err == nil {
					timeRange = fmt.Sprintf("%s–%s", st.Local().Format("15:04"), et.Local().Format("15:04"))
				}
			}
		}

		entry := fmt.Sprintf(`---

## %s (%s) — %s

%s

<details>
<summary>Session ID</summary>
<code>%s</code>%s
</details>

`, meta.Project, branch, timeRange, summary, sessionID, cwdLine)

		f, err := os.OpenFile(journalFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Printf("    Failed to write: %v\n", err)
			continue
		}
		f.WriteString(entry)
		f.Close()

		summarized++
		fmt.Printf("    Summarized → %s.md\n", journalDate)
	}

	fmt.Printf("\nDone. Summarized %d sessions, skipped %d already-journaled.\n", summarized, skipped)
}

// collectExistingIDs scans all journal files for session UUIDs.
func collectExistingIDs() map[string]bool {
	ids := make(map[string]bool)
	dir := journalDir()
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	re := regexp.MustCompile(`<code>([a-f0-9-]{36})</code>`)
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, match := range re.FindAllSubmatch(content, -1) {
			ids[string(match[1])] = true
		}
	}
	return ids
}
