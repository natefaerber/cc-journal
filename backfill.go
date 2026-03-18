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

// parseSince parses a duration expression like "1d", "2h", "30m" into a cutoff time.
// By default, days align to midnight and hours align to top-of-hour.
// With rolling=true, it uses exact duration subtraction.
func parseSince(expr string, rolling bool) (time.Time, error) {
	expr = strings.TrimPrefix(expr, "-")
	if len(expr) < 2 {
		return time.Time{}, fmt.Errorf("invalid since expression: %q (use e.g. 1d, 2h, 30m)", expr)
	}

	unit := expr[len(expr)-1]
	numStr := expr[:len(expr)-1]
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("invalid since expression: %q", expr)
	}

	now := time.Now()
	switch unit {
	case 'd':
		if rolling {
			return now.Add(-time.Duration(n) * 24 * time.Hour), nil
		}
		// Midnight-aligned: start of today minus (n-1) days
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return midnight.AddDate(0, 0, -(n - 1)), nil
	case 'h':
		if rolling {
			return now.Add(-time.Duration(n) * time.Hour), nil
		}
		// Top-of-hour aligned
		topOfHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		return topOfHour.Add(-time.Duration(n-1) * time.Hour), nil
	case 'm':
		// Minutes are always rolling (alignment isn't useful)
		return now.Add(-time.Duration(n) * time.Minute), nil
	default:
		return time.Time{}, fmt.Errorf("unknown unit %q in since expression (use d, h, or m)", string(unit))
	}
}

// runBackfill retroactively summarizes existing Claude Code sessions.
func runBackfill(cutoff time.Time, dryRun bool, force bool) {
	home, _ := os.UserHomeDir()
	projectsDir := filepath.Join(home, ".claude", "projects")

	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No Claude Code projects found.")
		os.Exit(1)
	}

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

	fmt.Printf("Found %d session files since %s.\n\n", len(sessionFiles), cutoff.Format("2006-01-02 15:04"))

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
		// Always skip denied sessions, even with --force
		if denied[sessionID] {
			skipped++
			continue
		}
		alreadyDone := existingIDs[sessionID]

		if !force && alreadyDone && !dryRun {
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
		if meta.CWD != "" && isExcluded(meta.CWD) {
			continue
		}

		status := "new"
		if alreadyDone {
			if denied[sessionID] {
				status = "denied"
			} else {
				status = "journaled"
			}
		}

		// Calculate how much room is left for the message preview
		// Format: "  [status] sessionID  project (branch)  msg"
		prefixLen := 2 + 1 + len(status) + 2 + len(sessionID) + 2 + len(meta.Project) + 2 + len(meta.BranchDisplay()) + 4
		maxMsg := 100 - prefixLen
		if maxMsg < 20 {
			maxMsg = 20
		}

		// Get first user message for display (first line, truncated)
		firstUserMsg := ""
		for _, m := range meta.Messages {
			if m.Role == "user" {
				firstUserMsg = m.Text
				if nl := strings.IndexByte(firstUserMsg, '\n'); nl >= 0 {
					firstUserMsg = firstUserMsg[:nl]
				}
				if len(firstUserMsg) > maxMsg {
					firstUserMsg = firstUserMsg[:maxMsg-3] + "..."
				}
				break
			}
		}

		fmt.Printf("  [%s] %s  %s (%s)  %s\n", status, sessionID, meta.Project, meta.BranchDisplay(), firstUserMsg)

		if dryRun || (!force && alreadyDone) {
			if alreadyDone {
				skipped++
			}
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
		summary, summaryTokens, err := callAnthropicAPI(apiKey, transcript, meta.Project, meta.BranchDisplay())
		if err == nil {
			meta.Tokens.SummaryInputTokens = summaryTokens.SummaryInputTokens
			meta.Tokens.SummaryOutputTokens = summaryTokens.SummaryOutputTokens
		}
		if err != nil {
			fmt.Printf("    Failed: %v\n", err)
			continue
		}

		// Determine journal date from last timestamp
		journalDate := time.Now().Format("2006-01-02")
		if meta.LastTime != "" {
			if t, err := time.Parse(time.RFC3339Nano, meta.LastTime); err == nil {
				journalDate = t.Local().Format("2006-01-02")
			}
		}

		// Replace existing entry if force re-summarizing
		if force && alreadyDone {
			replaceWithStub(sessionID, journalDate)
		}

		// Write to the correct date's journal (resolved from session cwd)
		dir := resolveJournalDir(meta.CWD)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create journal dir: %v\n", err)
			continue
		}
		journalFile := filepath.Join(dir, journalDate+".md")

		if _, err := os.Stat(journalFile); os.IsNotExist(err) {
			if err := os.WriteFile(journalFile, []byte(fmt.Sprintf("# Claude Code Journal — %s\n\n", journalDate)), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create journal file: %v\n", err)
				continue
			}
		}

		branch := meta.BranchDisplay()
		cwdLine := ""
		if meta.CWD != "" {
			cwdLine = fmt.Sprintf("\n<code>%s</code>", meta.CWD)
		}

		tokensLine := ""
		if meta.Tokens.SessionTokens() > 0 || meta.Tokens.SummaryInputTokens > 0 {
			tokensLine = fmt.Sprintf("\n<code>tokens:in=%d,out=%d,cache_create=%d,cache_read=%d,summary_in=%d,summary_out=%d</code>",
				meta.Tokens.InputTokens, meta.Tokens.OutputTokens,
				meta.Tokens.CacheCreationInputTokens, meta.Tokens.CacheReadInputTokens,
				meta.Tokens.SummaryInputTokens, meta.Tokens.SummaryOutputTokens)
		}

		timeRange := "unknown"
		if meta.FirstTime != "" && meta.LastTime != "" {
			if st, err := time.Parse(time.RFC3339Nano, meta.FirstTime); err == nil {
				if et, err := time.Parse(time.RFC3339Nano, meta.LastTime); err == nil {
					stLocal := st.Local()
					etLocal := et.Local()
					if stLocal.Format("2006-01-02") == etLocal.Format("2006-01-02") {
						timeRange = fmt.Sprintf("%s–%s", stLocal.Format("15:04"), etLocal.Format("15:04"))
					} else {
						timeRange = fmt.Sprintf("%s–%s", stLocal.Format("Jan 02 15:04"), etLocal.Format("Jan 02 15:04"))
					}
				}
			}
		}

		entry := fmt.Sprintf(`---

## %s (%s) — %s

%s

<details>
<summary>Session ID</summary>
<code>%s</code>%s%s
</details>

`, meta.Project, branch, timeRange, summary, sessionID, cwdLine, tokensLine)

		f, err := os.OpenFile(journalFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Printf("    Failed to write: %v\n", err)
			continue
		}
		if _, err := f.WriteString(entry); err != nil {
			fmt.Printf("    Failed to write entry: %v\n", err)
			_ = f.Close()
			continue
		}
		if err := f.Close(); err != nil {
			fmt.Printf("    Failed to close file: %v\n", err)
			continue
		}

		summarized++
		fmt.Printf("    Summarized → %s.md\n", journalDate)
	}

	if dryRun {
		newCount := len(sessionFiles) - skipped
		fmt.Printf("\nDry run: %d new, %d already journaled/denied.\n", newCount, skipped)
		if skipped > 0 {
			fmt.Println("To re-summarize a session: cc-journal summarize SESSION_ID --force")
		}
	} else {
		fmt.Printf("\nDone. Summarized %d sessions, skipped %d already-journaled.\n", summarized, skipped)
	}
}

// collectExistingIDs scans all journal directories for session UUIDs.
func collectExistingIDs() map[string]bool {
	ids := make(map[string]bool)
	re := regexp.MustCompile(`<code>([a-f0-9-]{36})</code>`)
	for _, dir := range allJournalDirs() {
		files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		for _, f := range files {
			content, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			for _, match := range re.FindAllSubmatch(content, -1) {
				ids[string(match[1])] = true
			}
		}
	}
	return ids
}
