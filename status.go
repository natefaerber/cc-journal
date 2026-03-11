package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

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

func formatDaily() string {
	now := time.Now()
	today := now.Format("2006-01-02")

	// Yesterday (skip weekends)
	yesterday := now.AddDate(0, 0, -1)
	if now.Weekday() == time.Monday {
		yesterday = now.AddDate(0, 0, -3) // Friday
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
	fmt.Fprintf(&b, "*Daily Standup — %s*\n\n", now.Format("Monday, Jan 02"))

	b.WriteString("*Yesterday:*\n")
	if len(yesterdayEntries) > 0 {
		for _, g := range groupByProject(yesterdayEntries) {
			var bullets []string
			for _, e := range g.Entries {
				if s := summarizeForSlack(e.Summary); s != "" {
					bullets = append(bullets, s)
				}
			}
			if len(bullets) > 0 {
				fmt.Fprintf(&b, "  `%s` (%s)\n", g.Project, g.Entries[0].Branch)
				for _, bul := range bullets {
					fmt.Fprintf(&b, "    • %s\n", bul)
				}
			}
		}
	} else {
		fmt.Fprintf(&b, "  No sessions recorded (%s)\n", yesterdayStr)
	}
	b.WriteString("\n")

	b.WriteString("*Today:*\n")
	if len(todayEntries) > 0 {
		for _, g := range groupByProject(todayEntries) {
			var bullets []string
			for _, e := range g.Entries {
				if s := summarizeForSlack(e.Summary); s != "" {
					bullets = append(bullets, s)
				}
			}
			if len(bullets) > 0 {
				fmt.Fprintf(&b, "  `%s` (%s)\n", g.Project, g.Entries[0].Branch)
				for _, bul := range bullets {
					fmt.Fprintf(&b, "    • %s\n", bul)
				}
			}
		}
	} else {
		b.WriteString("  (no sessions yet)\n")
	}
	b.WriteString("\n")

	b.WriteString("*Blockers:*\n")
	b.WriteString("  None\n")

	return b.String()
}

func formatWeekly() string {
	now := time.Now()
	today := now.Format("2006-01-02")

	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -(weekday - 1))

	data := parseJournalFiles()

	var weekEntries []Entry
	for _, e := range data.Entries {
		if e.Date >= monday.Format("2006-01-02") && e.Date <= today {
			weekEntries = append(weekEntries, e)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "*Weekly Status — Week of %s*\n\n", monday.Format("Jan 02, 2006"))

	if len(weekEntries) == 0 {
		b.WriteString("No sessions recorded this week.\n")
		return b.String()
	}

	groups := groupByProject(weekEntries)
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].Entries) > len(groups[j].Entries)
	})

	for _, g := range groups {
		branches := make(map[string]bool)
		for _, e := range g.Entries {
			branches[e.Branch] = true
		}
		branchList := make([]string, 0, len(branches))
		for br := range branches {
			branchList = append(branchList, br)
		}
		sort.Strings(branchList)

		count := len(g.Entries)
		plural := "s"
		if count == 1 {
			plural = ""
		}
		fmt.Fprintf(&b, "*`%s`* — %d session%s (%s)\n", g.Project, count, plural, strings.Join(branchList, ", "))

		seen := make(map[string]bool)
		for _, e := range g.Entries {
			if s := summarizeForSlack(e.Summary); s != "" && !seen[s] {
				seen[s] = true
				fmt.Fprintf(&b, "  • %s\n", s)
			}
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
	fmt.Fprintf(&b, "_Total: %d sessions across %d projects, %d active day%s_\n",
		len(weekEntries), len(groups), len(activeDays), daysPlural)

	return b.String()
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
