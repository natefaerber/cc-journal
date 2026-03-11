package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SummarySections holds structured sections extracted from an AI-generated summary.
type SummarySections struct {
	Done      []string
	Decisions []string
	Open      []string
}

var sectionRe = regexp.MustCompile(`(?m)^###\s+(.+?)\s*$`)

// parseSummarySections extracts ### Done, ### Decisions, ### Open bullet points from summary markdown.
func parseSummarySections(summary string) SummarySections {
	var s SummarySections
	if summary == "" {
		return s
	}

	indices := sectionRe.FindAllStringSubmatchIndex(summary, -1)
	if len(indices) == 0 {
		return s
	}

	for i, idx := range indices {
		heading := strings.TrimSpace(summary[idx[2]:idx[3]])
		bodyStart := idx[1]
		bodyEnd := len(summary)
		if i+1 < len(indices) {
			bodyEnd = indices[i+1][0]
		}
		body := summary[bodyStart:bodyEnd]
		bullets := extractBullets(body)

		switch strings.ToLower(heading) {
		case "done":
			s.Done = bullets
		case "decisions":
			s.Decisions = bullets
		case "open":
			s.Open = bullets
		}
	}
	return s
}

// extractBullets pulls "- " prefixed lines from text.
func extractBullets(text string) []string {
	var bullets []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			bullets = append(bullets, strings.TrimPrefix(line, "- "))
		}
	}
	return bullets
}

func summarizeForSlack(summary string) string {
	if summary == "" || strings.HasPrefix(summary, "_Summary generation failed") {
		return ""
	}

	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
	text := boldRe.ReplaceAllString(summary, "$1")

	if strings.HasPrefix(text, "Prompts:") {
		lines := strings.Split(text, "\n")
		var prompts []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "-") {
				p := strings.TrimPrefix(l, "- ")
				if len(p) > 80 {
					p = p[:77] + "..."
				}
				prompts = append(prompts, p)
			}
		}
		if len(prompts) > 3 {
			prompts = prompts[:3]
		}
		if len(prompts) > 0 {
			return strings.Join(prompts, "; ")
		}
		if len(text) > 120 {
			return text[:117] + "..."
		}
		return text
	}

	firstLine := strings.SplitN(text, "\n", 2)[0]
	if len(firstLine) > 120 {
		return firstLine[:117] + "..."
	}
	return firstLine
}

type projectGroup struct {
	Project string
	Branch  string
	Entries []Entry
}

func groupByProject(entries []Entry) []projectGroup {
	groups := make(map[string]*projectGroup)
	var order []string
	for _, e := range entries {
		if g, ok := groups[e.Project]; ok {
			g.Entries = append(g.Entries, e)
		} else {
			groups[e.Project] = &projectGroup{
				Project: e.Project,
				Branch:  e.Branch,
				Entries: []Entry{e},
			}
			order = append(order, e.Project)
		}
	}
	result := make([]projectGroup, 0, len(order))
	for _, name := range order {
		result = append(result, *groups[name])
	}
	return result
}

// collectBranches returns a sorted, deduplicated list of branch names from entries.
func collectBranches(entries []Entry) []string {
	seen := make(map[string]bool)
	for _, e := range entries {
		// Branch field may contain comma-separated branches from multi-branch tracking
		for _, b := range strings.Split(e.Branch, ", ") {
			b = strings.TrimSpace(b)
			if b != "" && b != "n/a" {
				seen[b] = true
			}
		}
	}
	list := make([]string, 0, len(seen))
	for b := range seen {
		list = append(list, b)
	}
	sort.Strings(list)
	return list
}

// sumDurations parses time ranges (e.g. "09:30–11:45") from entries and returns total duration.
func sumDurations(entries []Entry) time.Duration {
	var total time.Duration
	for _, e := range entries {
		total += parseDurationValue(e.TimeRange)
	}
	return total
}

// parseDurationValue parses a "HH:MM–HH:MM" time range into a duration.
func parseDurationValue(timeRange string) time.Duration {
	// Handle en-dash and regular dash
	timeRange = strings.ReplaceAll(timeRange, "–", "-")
	timeRange = strings.ReplaceAll(timeRange, "—", "-")
	parts := strings.SplitN(timeRange, "-", 2)
	if len(parts) != 2 {
		return 0
	}
	start := parseHHMM(strings.TrimSpace(parts[0]))
	end := parseHHMM(strings.TrimSpace(parts[1]))
	if start < 0 || end < 0 {
		return 0
	}
	d := end - start
	if d < 0 {
		d += 24 * time.Hour // crosses midnight
	}
	return d
}

func parseHHMM(s string) time.Duration {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return -1
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
}

// formatDuration returns a human-friendly duration like "~2h15m".
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("~%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("~%dh", h)
	}
	return fmt.Sprintf("~%dm", m)
}

// truncate shortens a string to n characters with ellipsis.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

// collectLinks gathers deduplicated links from entries.
func collectLinks(entries []Entry) []ExternalLink {
	seen := make(map[string]bool)
	var links []ExternalLink
	for _, e := range entries {
		for _, l := range e.Links {
			if !seen[l.URL] {
				seen[l.URL] = true
				links = append(links, l)
			}
		}
	}
	return links
}

// doneBullets extracts Done items from entries using structured parsing, falling back to summarizeForSlack.
func doneBullets(entries []Entry) []string {
	var bullets []string
	seen := make(map[string]bool)
	for _, e := range entries {
		sections := parseSummarySections(e.Summary)
		if len(sections.Done) > 0 {
			for _, d := range sections.Done {
				d = truncate(d, 120)
				if !seen[d] {
					seen[d] = true
					bullets = append(bullets, d)
				}
			}
		} else if s := summarizeForSlack(e.Summary); s != "" && !seen[s] {
			seen[s] = true
			bullets = append(bullets, s)
		}
	}
	return bullets
}

// openItems extracts Open items from all entries.
func openItems(entries []Entry, includeProject bool) []string {
	var items []string
	seen := make(map[string]bool)
	for _, e := range entries {
		sections := parseSummarySections(e.Summary)
		for _, o := range sections.Open {
			o = truncate(o, 120)
			if !seen[o] {
				seen[o] = true
				if includeProject {
					items = append(items, fmt.Sprintf("%s: %s", e.Project, o))
				} else {
					items = append(items, o)
				}
			}
		}
	}
	return items
}

// decisionItems extracts Decisions items from all entries.
func decisionItems(entries []Entry) []string {
	var items []string
	seen := make(map[string]bool)
	for _, e := range entries {
		sections := parseSummarySections(e.Summary)
		for _, d := range sections.Decisions {
			d = truncate(d, 120)
			if !seen[d] {
				seen[d] = true
				items = append(items, fmt.Sprintf("%s: %s", e.Project, d))
			}
		}
	}
	return items
}

// weekRange returns the Monday and Sunday of the week containing the given date.
func weekRange(t time.Time) (start, end time.Time) {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := t.AddDate(0, 0, -(weekday - 1))
	sunday := monday.AddDate(0, 0, 6)
	return monday, sunday
}

func formatDaily(target time.Time) string {
	today := target.Format("2006-01-02")

	// Yesterday (skip weekends)
	yesterday := target.AddDate(0, 0, -1)
	if target.Weekday() == time.Monday {
		yesterday = target.AddDate(0, 0, -3) // Friday
	}
	yesterdayStr := yesterday.Format("2006-01-02")

	data := parseJournalFiles()

	var yesterdayEntries, todayEntries []Entry
	for _, e := range data.Entries {
		if e.Date == yesterdayStr {
			yesterdayEntries = append(yesterdayEntries, e)
		}
		if e.Date == today {
			todayEntries = append(todayEntries, e)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "*Daily Standup — %s*\n\n", target.Format("Monday, Jan 02"))

	b.WriteString("*Yesterday:*\n")
	if len(yesterdayEntries) > 0 {
		for _, g := range groupByProject(yesterdayEntries) {
			branches := collectBranches(g.Entries)
			branchStr := "n/a"
			if len(branches) > 0 {
				branchStr = strings.Join(branches, ", ")
			}
			dur := sumDurations(g.Entries)
			durStr := formatDuration(dur)

			header := fmt.Sprintf("  `%s` (%s)", g.Project, branchStr)
			if durStr != "" {
				header += " " + durStr
			}
			fmt.Fprintf(&b, "%s\n", header)

			for _, bul := range doneBullets(g.Entries) {
				fmt.Fprintf(&b, "    • %s\n", bul)
			}
		}
	} else {
		fmt.Fprintf(&b, "  No sessions recorded (%s)\n", yesterdayStr)
	}
	b.WriteString("\n")

	b.WriteString("*Today:*\n")
	if len(todayEntries) > 0 {
		for _, g := range groupByProject(todayEntries) {
			branches := collectBranches(g.Entries)
			branchStr := "n/a"
			if len(branches) > 0 {
				branchStr = strings.Join(branches, ", ")
			}
			dur := sumDurations(g.Entries)
			durStr := formatDuration(dur)

			header := fmt.Sprintf("  `%s` (%s)", g.Project, branchStr)
			if durStr != "" {
				header += " " + durStr
			}
			fmt.Fprintf(&b, "%s\n", header)

			for _, bul := range doneBullets(g.Entries) {
				fmt.Fprintf(&b, "    • %s\n", bul)
			}
		}
	} else {
		b.WriteString("  (no sessions yet)\n")
	}
	b.WriteString("\n")

	// Collect open items from both days
	allEntries := append(yesterdayEntries, todayEntries...)
	open := openItems(allEntries, true)
	b.WriteString("*Open Items:*\n")
	if len(open) > 0 {
		for _, item := range open {
			fmt.Fprintf(&b, "  • %s\n", item)
		}
	} else {
		b.WriteString("  None\n")
	}

	// Links section
	links := collectLinks(allEntries)
	if len(links) > 0 {
		b.WriteString("\n*Links:*\n")
		for _, l := range links {
			fmt.Fprintf(&b, "  • %s: %s\n", l.Label, l.URL)
		}
	}

	return b.String()
}

func formatWeekly(start, end time.Time) string {
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	data := parseJournalFiles()

	var weekEntries []Entry
	for _, e := range data.Entries {
		if e.Date >= startStr && e.Date <= endStr {
			weekEntries = append(weekEntries, e)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "*Weekly Status — Week of %s*\n\n", start.Format("Jan 02, 2006"))

	if len(weekEntries) == 0 {
		b.WriteString("No sessions recorded this week.\n")
		return b.String()
	}

	groups := groupByProject(weekEntries)
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].Entries) > len(groups[j].Entries)
	})

	// Accomplishments
	b.WriteString("*Accomplishments:*\n")
	for _, g := range groups {
		branches := collectBranches(g.Entries)
		branchStr := ""
		if len(branches) > 0 {
			branchStr = " (" + strings.Join(branches, ", ") + ")"
		}
		dur := sumDurations(g.Entries)
		durStr := formatDuration(dur)

		count := len(g.Entries)
		plural := "s"
		if count == 1 {
			plural = ""
		}

		header := fmt.Sprintf("  *`%s`* — %d session%s%s", g.Project, count, plural, branchStr)
		if durStr != "" {
			header += " " + durStr
		}
		fmt.Fprintf(&b, "%s\n", header)

		for _, bul := range doneBullets(g.Entries) {
			fmt.Fprintf(&b, "    • %s\n", bul)
		}
	}
	b.WriteString("\n")

	// Key Decisions
	decisions := decisionItems(weekEntries)
	if len(decisions) > 0 {
		b.WriteString("*Key Decisions:*\n")
		for _, d := range decisions {
			fmt.Fprintf(&b, "  • %s\n", d)
		}
		b.WriteString("\n")
	}

	// Open/Carry-Forward
	open := openItems(weekEntries, true)
	if len(open) > 0 {
		b.WriteString("*Open/Carry-Forward:*\n")
		for _, item := range open {
			fmt.Fprintf(&b, "  • %s\n", item)
		}
		b.WriteString("\n")
	}

	// Links
	links := collectLinks(weekEntries)
	if len(links) > 0 {
		b.WriteString("*Links:*\n")
		for _, l := range links {
			fmt.Fprintf(&b, "  • %s: %s\n", l.Label, l.URL)
		}
		b.WriteString("\n")
	}

	activeDays := make(map[string]bool)
	for _, e := range weekEntries {
		activeDays[e.Date] = true
	}
	daysPlural := "s"
	if len(activeDays) == 1 {
		daysPlural = ""
	}
	fmt.Fprintf(&b, "_%d sessions across %d project%s, %d active day%s_\n",
		len(weekEntries), len(groups), plural(len(groups)), len(activeDays), daysPlural)

	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func copyToClipboard(text string) error {
	var name string
	var args []string
	switch {
	case clipboardAvailable("pbcopy"):
		name = "pbcopy"
	case clipboardAvailable("xclip"):
		name, args = "xclip", []string{"-selection", "clipboard"}
	case clipboardAvailable("xsel"):
		name, args = "xsel", []string{"--clipboard", "--input"}
	default:
		return fmt.Errorf("no clipboard command found (install pbcopy, xclip, or xsel)")
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func clipboardAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// sendToSlack sends text to a Slack channel using the configured command.
// channelOverride takes precedence over the config default.
func sendToSlack(text, channelOverride string) error {
	command := cfg.Slack.Command
	if command == "" {
		return fmt.Errorf("slack.command not set in config (%s)", configPath())
	}

	channel := channelOverride
	if channel == "" {
		channel = cfg.Slack.Channel
	}
	if channel == "" {
		return fmt.Errorf("no Slack channel specified (use --slack '#channel' or set slack.channel in config)")
	}

	cmd := exec.Command(command, channel, text)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
