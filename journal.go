package main

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Entry struct {
	Date         string         `json:"date"`
	Project      string         `json:"project"`
	Branch       string         `json:"branch"`
	TimeRange    string         `json:"time_range"`
	SessionID    string         `json:"session_id"`
	Cwd          string         `json:"cwd"`
	Summary      string         `json:"summary"`
	HasAISummary bool           `json:"has_ai_summary"`
	Links        []ExternalLink `json:"links,omitempty"`
	Tokens       TokenUsage     `json:"tokens,omitempty"`
}

type ProjectCount struct {
	Name     string `json:"name"`
	Count    int    `json:"count"`
	LastDate string `json:"last_date"`
}

type Stats struct {
	TotalSessions       int    `json:"total_sessions"`
	TotalDays           int    `json:"total_days"`
	TotalProjects       int    `json:"total_projects"`
	ThisWeek            int    `json:"this_week"`
	Streak              int    `json:"streak"`
	MostActive          string `json:"most_active"`
	WeekStartLabel      string `json:"week_start_label"`
	SessionInputTokens  int64  `json:"session_input_tokens"`
	SessionOutputTokens int64  `json:"session_output_tokens"`
	SummaryInputTokens  int64  `json:"summary_input_tokens"`
	SummaryOutputTokens int64  `json:"summary_output_tokens"`
}

type Bar struct {
	Date      string  `json:"date"`
	Count     int     `json:"count"`
	HeightPct float64 `json:"height_pct"`
	ShowLabel bool    `json:"show_label"`
	Label     string  `json:"label"`
}

type HeatmapDay struct {
	Date     string
	Count    int
	Level    int
	IsFuture bool
}

type DashboardData struct {
	Stats          Stats
	RecentProjects []ProjectCount // last 3 days
	Projects       []ProjectCount // all time
	Bars           []Bar
	Heatmap        []HeatmapDay
	Recent         []Entry
}

type JournalData struct {
	Entries    []Entry
	DailyFiles []string
	Projects   []ProjectCount
}

var (
	headingRe   = regexp.MustCompile(`(?m)^##\s+(.+?)\s+\((.+?)\)\s+—\s+(.+?)$`)
	sessionIDRe = regexp.MustCompile(`<code>([a-f0-9-]+)</code>`)
	cwdRe       = regexp.MustCompile(`<code>(/[^<]+)</code>`)
	tokensRe    = regexp.MustCompile(`<code>tokens:in=(\d+),out=(\d+),cache_create=(\d+),cache_read=(\d+),summary_in=(\d+),summary_out=(\d+)</code>`)
	summaryRe   = regexp.MustCompile(`(?ms)^##.+?\n\n(.*?)(?:\n<details>|\z)`)
	separatorRe = regexp.MustCompile(`(?m)^---\s*$`)
	linkRe      = regexp.MustCompile(`- \[(.+?)\]\((.+?)\)`)
)

func journalDir() string {
	return cfg.JournalDir
}

// readDateFromAllDirs reads and concatenates a date's journal file from all journal dirs.
// Returns nil if no file was found in any directory.
func readDateFromAllDirs(date string) []byte {
	var combined []byte
	for _, dir := range allJournalDirs() {
		content, err := os.ReadFile(filepath.Join(dir, date+".md"))
		if err != nil {
			continue
		}
		combined = append(combined, content...)
	}
	if len(combined) == 0 {
		return nil
	}
	return combined
}

// parseJournalDir parses journal entries from a single directory.
func parseJournalDir(dir string) ([]Entry, []string) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var allEntries []Entry
	var dailyFiles []string

	files := make([]string, 0)
	for _, e := range dirEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !strings.Contains(e.Name(), "rollup") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, fname := range files {
		dailyFiles = append(dailyFiles, fname)
		date := strings.TrimSuffix(fname, ".md")
		content, err := os.ReadFile(filepath.Join(dir, fname))
		if err != nil {
			continue
		}

		sections := separatorRe.Split(string(content), -1)
		for _, section := range sections {
			matches := headingRe.FindStringSubmatch(section)
			if matches == nil {
				continue
			}

			project := strings.TrimSpace(matches[1])
			branch := strings.TrimSpace(matches[2])
			timeRange := strings.TrimSpace(matches[3])

			sessionID := ""
			if m := sessionIDRe.FindStringSubmatch(section); m != nil {
				sessionID = m[1]
			}

			cwd := ""
			if m := cwdRe.FindStringSubmatch(section); m != nil {
				cwd = m[1]
			}

			summary := ""
			if m := summaryRe.FindStringSubmatch(section); m != nil {
				summary = strings.TrimSpace(m[1])
			}

			// Parse stored links from <details><summary>Links</summary> block
			var links []ExternalLink
			if strings.Contains(section, "<summary>Links</summary>") {
				for _, m := range linkRe.FindAllStringSubmatch(section, -1) {
					link := classifyURL(m[2])
					if link != nil {
						links = append(links, *link)
					} else {
						// Fallback: store as-is
						links = append(links, ExternalLink{Label: m[1], URL: m[2]})
					}
				}
			}
			// Parse token usage
			var tokens TokenUsage
			if m := tokensRe.FindStringSubmatch(section); m != nil {
				tokens.InputTokens, _ = strconv.ParseInt(m[1], 10, 64)
				tokens.OutputTokens, _ = strconv.ParseInt(m[2], 10, 64)
				tokens.CacheCreationInputTokens, _ = strconv.ParseInt(m[3], 10, 64)
				tokens.CacheReadInputTokens, _ = strconv.ParseInt(m[4], 10, 64)
				tokens.SummaryInputTokens, _ = strconv.ParseInt(m[5], 10, 64)
				tokens.SummaryOutputTokens, _ = strconv.ParseInt(m[6], 10, 64)
			}

			// Also extract issue keys from summary text
			if issueLinks := extractIssueKeysFromText(summary); len(issueLinks) > 0 {
				seen := make(map[string]bool)
				for _, l := range links {
					seen[l.Label] = true
				}
				for _, l := range issueLinks {
					if !seen[l.Label] {
						links = append(links, l)
					}
				}
			}

			allEntries = append(allEntries, Entry{
				Date:         date,
				Project:      project,
				Branch:       branch,
				TimeRange:    timeRange,
				SessionID:    sessionID,
				Cwd:          cwd,
				Summary:      summary,
				HasAISummary: !strings.HasPrefix(summary, "**Prompts:**"),
				Links:        links,
				Tokens:       tokens,
			})
		}
	}

	return allEntries, dailyFiles
}

// parseJournalFiles aggregates entries from all journal directories (default + profiles).
func parseJournalFiles() JournalData {
	var allEntries []Entry
	dailyFileSet := make(map[string]bool)
	projectCounts := make(map[string]int)

	for _, dir := range allJournalDirs() {
		entries, dailyFiles := parseJournalDir(dir)
		allEntries = append(allEntries, entries...)
		for _, f := range dailyFiles {
			dailyFileSet[f] = true
		}
	}

	for _, e := range allEntries {
		projectCounts[e.Project]++
	}

	// Deduplicate and sort daily files
	var dailyFiles []string
	for f := range dailyFileSet {
		dailyFiles = append(dailyFiles, f)
	}
	sort.Strings(dailyFiles)

	// Track last active date per project
	projectLastDate := make(map[string]string)
	for _, e := range allEntries {
		if e.Date > projectLastDate[e.Project] {
			projectLastDate[e.Project] = e.Date
		}
	}

	// Sort projects by count descending
	projects := make([]ProjectCount, 0, len(projectCounts))
	for name, count := range projectCounts {
		projects = append(projects, ProjectCount{Name: name, Count: count, LastDate: projectLastDate[name]})
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Count > projects[j].Count
	})

	return JournalData{
		Entries:    allEntries,
		DailyFiles: dailyFiles,
		Projects:   projects,
	}
}

func computeStats(data JournalData) Stats {
	now := time.Now()
	today := now.Format("2006-01-02")

	// First day of this week
	weekStart := startOfWeek(now)
	weekStartStr := weekStart.Format("2006-01-02")

	uniqueDays := make(map[string]bool)
	thisWeek := 0
	var sessIn, sessOut, sumIn, sumOut int64
	for _, e := range data.Entries {
		uniqueDays[e.Date] = true
		if e.Date >= weekStartStr {
			thisWeek++
		}
		sessIn += e.Tokens.InputTokens + e.Tokens.CacheCreationInputTokens + e.Tokens.CacheReadInputTokens
		sessOut += e.Tokens.OutputTokens
		sumIn += e.Tokens.SummaryInputTokens
		sumOut += e.Tokens.SummaryOutputTokens
	}

	// Streak
	dates := make([]string, 0, len(uniqueDays))
	for d := range uniqueDays {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	streak := 0
	check := today
	for _, d := range dates {
		if d == check {
			streak++
			t, _ := time.Parse("2006-01-02", check)
			check = t.AddDate(0, 0, -1).Format("2006-01-02")
		} else if d < check {
			break
		}
	}

	mostActive := "n/a"
	if len(data.Projects) > 0 {
		mostActive = data.Projects[0].Name
	}

	return Stats{
		TotalSessions:       len(data.Entries),
		TotalDays:           len(uniqueDays),
		TotalProjects:       len(data.Projects),
		ThisWeek:            thisWeek,
		Streak:              streak,
		MostActive:          mostActive,
		WeekStartLabel:      weekStart.Format("Jan 02"),
		SessionInputTokens:  sessIn,
		SessionOutputTokens: sessOut,
		SummaryInputTokens:  sumIn,
		SummaryOutputTokens: sumOut,
	}
}

func computeActivity(entries []Entry, days int) []Bar {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	counts := make(map[string]int)
	for _, e := range entries {
		counts[e.Date]++
	}

	maxCount := 1
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	bars := make([]Bar, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		count := counts[dateStr]
		heightPct := math.Max(float64(count)/float64(maxCount)*100, 2)
		showLabel := d.Weekday() == weekStartDay() || d.Day() == 1

		bars = append(bars, Bar{
			Date:      dateStr,
			Count:     count,
			HeightPct: math.Round(heightPct*10) / 10,
			ShowLabel: showLabel,
			Label:     d.Format("Jan 02"),
		})
	}
	return bars
}

func computeHeatmap(entries []Entry, weeks int) []HeatmapDay {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	counts := make(map[string]int)
	for _, e := range entries {
		counts[e.Date]++
	}

	maxCount := 1
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	// Start from first day of (weeks) weeks ago
	thisWeekStart := startOfWeek(today)
	start := thisWeekStart.AddDate(0, 0, -(weeks-1)*7)

	days := make([]HeatmapDay, 0, weeks*7)
	for i := 0; i < weeks*7; i++ {
		d := start.AddDate(0, 0, i)
		dateStr := d.Format("2006-01-02")
		count := counts[dateStr]

		level := 0
		if count > 0 {
			ratio := float64(count) / float64(maxCount)
			switch {
			case ratio <= 0.25:
				level = 1
			case ratio <= 0.5:
				level = 2
			case ratio <= 0.75:
				level = 3
			default:
				level = 4
			}
		}

		days = append(days, HeatmapDay{
			Date:     dateStr,
			Count:    count,
			Level:    level,
			IsFuture: d.After(today),
		})
	}
	return days
}

func buildDashboard(data JournalData) DashboardData {
	stats := computeStats(data)
	bars := computeActivity(data.Entries, 28)
	heatmap := computeHeatmap(data.Entries, 8)

	// Recent entries
	recent := make([]Entry, len(data.Entries))
	copy(recent, data.Entries)
	sort.Slice(recent, func(i, j int) bool {
		if recent[i].Date != recent[j].Date {
			return recent[i].Date > recent[j].Date
		}
		return recent[i].TimeRange > recent[j].TimeRange
	})
	if len(recent) > 15 {
		recent = recent[:15]
	}

	// Projects from last 3 days for dashboard
	cutoff := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	recentProjCounts := make(map[string]int)
	for _, e := range data.Entries {
		if e.Date >= cutoff {
			recentProjCounts[e.Project]++
		}
	}
	var recentProjects []ProjectCount
	for name, count := range recentProjCounts {
		recentProjects = append(recentProjects, ProjectCount{Name: name, Count: count})
	}
	sort.Slice(recentProjects, func(i, j int) bool {
		return recentProjects[i].Count > recentProjects[j].Count
	})

	return DashboardData{
		Stats:          stats,
		RecentProjects: recentProjects,
		Projects:       data.Projects,
		Bars:           bars,
		Heatmap:        heatmap,
		Recent:         recent,
	}
}

// SummaryPreview returns a truncated, cleaned summary for table display.
// Extracts the first Done bullet from structured summaries, falling back to first content line.
func (e Entry) SummaryPreview() string {
	s := e.Summary
	if len(s) == 0 {
		return ""
	}

	// Try to extract first Done bullet from structured summary
	sections := parseSummarySections(s)
	if len(sections.Done) > 0 {
		preview := sections.Done[0]
		// Strip markdown bold
		boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
		preview = boldRe.ReplaceAllString(preview, "$1")
		if len(preview) > 120 {
			preview = preview[:117] + "..."
		}
		return preview
	}

	// Fallback: first non-heading, non-empty line
	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "###") || strings.HasPrefix(line, "**Prompts:**") {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		line = boldRe.ReplaceAllString(line, "$1")
		if len(line) > 120 {
			line = line[:117] + "..."
		}
		return line
	}
	return ""
}
