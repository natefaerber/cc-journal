package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// SearchResult represents a journal entry that matched a search query.
type SearchResult struct {
	Entry   Entry
	Snippet string
	Field   string // "summary", "project", "branch"
}

// searchEntries performs case-insensitive substring search across entries.
func searchEntries(entries []Entry, query string, limit int) []SearchResult {
	q := strings.ToLower(query)
	var results []SearchResult

	for _, e := range entries {
		if r := matchEntry(e, q); r != nil {
			results = append(results, *r)
		}
	}

	// Newest first
	sort.Slice(results, func(i, j int) bool {
		if results[i].Entry.Date != results[j].Entry.Date {
			return results[i].Entry.Date > results[j].Entry.Date
		}
		return results[i].Entry.TimeRange > results[j].Entry.TimeRange
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func matchEntry(e Entry, q string) *SearchResult {
	// Check summary first (most useful matches)
	if strings.Contains(strings.ToLower(e.Summary), q) {
		return &SearchResult{
			Entry:   e,
			Snippet: extractSnippet(e.Summary, q, 120),
			Field:   "summary",
		}
	}
	if strings.Contains(strings.ToLower(e.Project), q) {
		return &SearchResult{
			Entry:   e,
			Snippet: e.SummaryPreview(),
			Field:   "project",
		}
	}
	if strings.Contains(strings.ToLower(e.Branch), q) {
		return &SearchResult{
			Entry:   e,
			Snippet: e.SummaryPreview(),
			Field:   "branch",
		}
	}
	return nil
}

// stripMarkdown removes common markdown formatting for clean snippets.
var mdStripRe = regexp.MustCompile(`(?m)^#{1,4}\s+|^\s*[-*]\s+|\*\*(.+?)\*\*`)

func extractSnippet(text, query string, window int) string {
	// Clean markdown
	clean := mdStripRe.ReplaceAllStringFunc(text, func(s string) string {
		if strings.HasPrefix(s, "**") {
			return strings.Trim(s, "*")
		}
		return ""
	})
	clean = strings.Join(strings.Fields(clean), " ")

	lower := strings.ToLower(clean)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx < 0 {
		if len(clean) > window {
			return clean[:window] + "..."
		}
		return clean
	}

	half := window / 2
	start := idx - half
	end := idx + len(query) + half

	prefix := ""
	suffix := ""
	if start < 0 {
		start = 0
	} else {
		prefix = "..."
	}
	if end > len(clean) {
		end = len(clean)
	} else {
		suffix = "..."
	}

	return prefix + clean[start:end] + suffix
}

// runSearchCLI is the CLI entry point for the search command.
func runSearchCLI(queryParts []string, project string, limit int) {
	query := strings.Join(queryParts, " ")
	if query == "" {
		fmt.Fprintln(os.Stderr, "Usage: cc-journal search <QUERY> [--project PROJECT] [--limit N]")
		os.Exit(1)
	}

	data := parseJournalFiles()
	results := searchEntries(data.Entries, query, 0)

	// Filter by project if specified
	if project != "" {
		p := strings.ToLower(project)
		filtered := results[:0]
		for _, r := range results {
			if strings.ToLower(r.Entry.Project) == p {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	if len(results) == 0 {
		fmt.Printf("No results for %q\n", query)
		return
	}

	for _, r := range results {
		fmt.Printf("%s  %s (%s)  %s\n", r.Entry.Date, r.Entry.Project, r.Entry.Branch, r.Entry.SessionID[:8])
		fmt.Printf("  %s\n\n", r.Snippet)
	}
	fmt.Printf("Found %d result%s for %q\n", len(results), plural(len(results)), query)
}
