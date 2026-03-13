package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed templates/*
var embeddedTemplates embed.FS

var funcMap = template.FuncMap{
	"sub": func(a, b int) int { return a - b },
	"formatTokens": func(n int64) string {
		if n >= 1_000_000 {
			return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
		}
		if n >= 1_000 {
			return fmt.Sprintf("%.1fk", float64(n)/1_000)
		}
		return fmt.Sprintf("%d", n)
	},
	"sessionTokens":    func(t TokenUsage) int64 { return t.SessionTokens() },
	"sessionInputTokens": func(t TokenUsage) int64 {
		return t.InputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
	},
	"sessionOutputTokens": func(t TokenUsage) int64 { return t.OutputTokens },
	"levelClass": func(level int) string {
		if level == 0 {
			return ""
		}
		return fmt.Sprintf("l%d", level)
	},
	"serviceIcon": func(service string) string {
		switch service {
		case "github":
			return "GH"
		case "linear":
			return "Li"
		case "jira":
			return "Ji"
		case "confluence":
			return "Co"
		default:
			return "Lk"
		}
	},
}

// loadTemplate loads a specific page template with the base layout.
// It first checks the override directory, then falls back to embedded.
func loadTemplate(name string, overrideDir string) (*template.Template, error) {
	embeddedFS, _ := fs.Sub(embeddedTemplates, "templates")

	readTemplate := func(filename string) (string, error) {
		// Check override first
		if overrideDir != "" {
			path := filepath.Join(overrideDir, filename)
			if data, err := os.ReadFile(path); err == nil {
				return string(data), nil
			}
		}
		// Fall back to embedded
		data, err := fs.ReadFile(embeddedFS, filename)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	baseStr, err := readTemplate("base.html")
	if err != nil {
		return nil, fmt.Errorf("reading base.html: %w", err)
	}

	pageStr, err := readTemplate(name)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}

	tmpl, err := template.New("base").Funcs(funcMap).Parse(baseStr)
	if err != nil {
		return nil, fmt.Errorf("parsing base.html: %w", err)
	}

	tmpl, err = tmpl.Parse(pageStr)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}

	return tmpl, nil
}

// renderReportMarkdown converts Slack-flavored report text to rendered HTML.
// Slack uses *bold* and _italic_; standard markdown uses **bold** and *italic*.
func renderReportMarkdown(src string) template.HTML {
	// Convert Slack *bold* to markdown **bold** (but not inside backticks)
	// Match *text* that isn't inside backticks and isn't **already doubled**
	slackBold := regexp.MustCompile(`(?m)(^|[^*\x60])\*([^*\n]+)\*([^*]|$)`)
	converted := slackBold.ReplaceAllString(src, "${1}**${2}**${3}")
	// Convert Slack _italic_ to markdown *italic*
	slackItalic := regexp.MustCompile(`(?m)(^|[\s(])_([^_\n]+)_([^_]|$)`)
	converted = slackItalic.ReplaceAllString(converted, "${1}*${2}*${3}")
	// Convert bullet lines with • to markdown - bullets
	converted = strings.ReplaceAll(converted, "    • ", "  - ")
	converted = strings.ReplaceAll(converted, "  • ", "- ")
	return renderMarkdown(converted)
}

func renderMarkdown(src string) template.HTML {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Linkify),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML("<p>Error rendering markdown</p>")
	}
	return template.HTML(buf.String())
}

type DailyInfo struct {
	Date     string
	Count    int
	Projects []string
	TimeSpan string // e.g. "09:15–17:30"
}

type PageData struct {
	Title       string
	Content     template.HTML
	Dashboard   *DashboardData
	Dates       []string
	Days        []DailyInfo
	ReportText  string
	ReportHTML  template.HTML
	ProjectName string
	Entries     []Entry
	PrevDate    string
	NextDate    string
	PrevURL        string // previous report URL for nav
	NextURL        string // next report URL for nav
	ReportType     string // "standup" or "weekly"
	ReportSubtitle string // date display for report header
	ReportDateVal  string // YYYY-MM-DD for date picker (standup)
	ReportStartVal string // YYYY-MM-DD for date picker (weekly start)
	ReportEndVal   string // YYYY-MM-DD for date picker (weekly end)
}

func renderPage(name string, overrideDir string, data PageData) ([]byte, error) {
	tmpl, err := loadTemplate(name, overrideDir)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func getDailyInfos(data JournalData) []DailyInfo {
	// Group entries by date
	byDate := make(map[string][]Entry)
	for _, e := range data.Entries {
		byDate[e.Date] = append(byDate[e.Date], e)
	}

	dates := getDates(data)
	infos := make([]DailyInfo, 0, len(dates))
	for _, d := range dates {
		entries := byDate[d]
		projects := make(map[string]bool)
		var earliest, latest string
		for _, e := range entries {
			projects[e.Project] = true
			if e.TimeRange != "" {
				parts := strings.SplitN(e.TimeRange, "–", 2)
				if len(parts) == 2 {
					if earliest == "" || parts[0] < earliest {
						earliest = parts[0]
					}
					if parts[1] > latest {
						latest = parts[1]
					}
				}
			}
		}
		var projList []string
		for p := range projects {
			projList = append(projList, p)
		}
		sort.Strings(projList)
		span := ""
		if earliest != "" && latest != "" {
			span = earliest + "–" + latest
		}
		infos = append(infos, DailyInfo{
			Date:     d,
			Count:    len(entries),
			Projects: projList,
			TimeSpan: span,
		})
	}
	return infos
}

func getDates(data JournalData) []string {
	dates := make([]string, len(data.DailyFiles))
	for i, f := range data.DailyFiles {
		dates[i] = strings.TrimSuffix(f, ".md")
	}
	// Reverse for newest first
	for i, j := 0, len(dates)-1; i < j; i, j = i+1, j-1 {
		dates[i], dates[j] = dates[j], dates[i]
	}
	return dates
}

// adjacentDates finds the previous and next dates relative to the given date.
// dates is sorted newest-first, so "next" is earlier in the slice (newer) and "prev" is later (older).
func adjacentDates(date string, dates []string) (prev, next string) {
	for i, d := range dates {
		if d == date {
			if i > 0 {
				next = dates[i-1] // newer
			}
			if i < len(dates)-1 {
				prev = dates[i+1] // older
			}
			return
		}
	}
	return
}

func serve(port int, templatesDir string) {
	http.HandleFunc("/api/remove", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		sessionID := r.FormValue("session_id")
		if sessionID == "" {
			http.Error(w, "Missing session_id", 400)
			return
		}
		removed := removeFromJournal(sessionID)
		if err := addToDenyList(sessionID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"removed":%d,"denied":true}`, removed)
	})

	http.HandleFunc("/api/palette", func(w http.ResponseWriter, r *http.Request) {
		data := parseJournalFiles()
		type PaletteItem struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			Desc  string `json:"desc"`
			URL   string `json:"url"`
		}
		var items []PaletteItem

		// Pages
		items = append(items,
			PaletteItem{"page", "Dashboard", "Overview and stats", "/"},
			PaletteItem{"page", "Daily Entries", "Browse by date", "/daily"},
			PaletteItem{"page", "Standup", "Today's standup report", "/standup"},
			PaletteItem{"page", "Weekly", "Weekly status report", "/weekly"},
		)

		// Projects
		projCounts := make(map[string]int)
		for _, e := range data.Entries {
			projCounts[e.Project]++
		}
		for p, c := range projCounts {
			items = append(items, PaletteItem{"project", p, fmt.Sprintf("%d sessions", c), "/project/" + p})
		}

		// Daily entries
		for _, info := range getDailyInfos(data) {
			desc := fmt.Sprintf("%d sessions", info.Count)
			if len(info.Projects) > 0 {
				desc += " — " + strings.Join(info.Projects, ", ")
			}
			items = append(items, PaletteItem{"date", info.Date, desc, "/daily/" + info.Date})
		}

		// Individual sessions (recent 50)
		limit := 50
		if len(data.Entries) < limit {
			limit = len(data.Entries)
		}
		recent := data.Entries[len(data.Entries)-limit:]
		for i := len(recent) - 1; i >= 0; i-- {
			e := recent[i]
			items = append(items, PaletteItem{
				"session",
				e.Project + " (" + e.Branch + ")",
				e.Date + " " + e.TimeRange + " — " + e.SummaryPreview(),
				"/daily/" + e.Date + "#" + e.SessionID,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	})

	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		limit := 30
		if v := r.URL.Query().Get("limit"); v != "" {
			fmt.Sscanf(v, "%d", &limit)
			if limit > 100 {
				limit = 100
			}
		}

		data := parseJournalFiles()
		results := searchEntries(data.Entries, q, limit)

		type SearchItem struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			Desc  string `json:"desc"`
			URL   string `json:"url"`
		}
		items := make([]SearchItem, 0, len(results))
		for _, r := range results {
			items = append(items, SearchItem{
				Type:  "search",
				Title: r.Entry.Project + " (" + r.Entry.Branch + ")",
				Desc:  r.Entry.Date + " " + r.Entry.TimeRange + " — " + r.Snippet,
				URL:   "/daily/" + r.Entry.Date + "#" + r.Entry.SessionID,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		data := parseJournalFiles()

		switch {
		case path == "" || path == "index.html":
			dash := buildDashboard(data)
			page := PageData{Title: "Dashboard", Dashboard: &dash, Dates: getDates(data)}
			out, err := renderPage("dashboard.html", templatesDir, page)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(out)

		case path == "daily" || path == "daily/":
			page := PageData{Title: "Daily Entries", Dates: getDates(data), Days: getDailyInfos(data)}
			out, err := renderPage("daily-list.html", templatesDir, page)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(out)

		case strings.HasPrefix(path, "daily/"):
			date := strings.TrimPrefix(path, "daily/")
			date = strings.TrimSuffix(date, "/")
			date = strings.TrimSuffix(date, ".html")

			if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, date); !matched {
				http.NotFound(w, r)
				return
			}

			journalFile := filepath.Join(journalDir(), date+".md")
			content, err := os.ReadFile(journalFile)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			htmlContent := autoLinkAll(renderMarkdown(string(content)))
			dates := getDates(data)
			prevDate, nextDate := adjacentDates(date, dates)
			page := PageData{
				Title:    "Journal — " + date,
				Content:  htmlContent,
				Dates:    dates,
				PrevDate: prevDate,
				NextDate: nextDate,
			}
			out, err := renderPage("daily-entry.html", templatesDir, page)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(out)

		case strings.HasPrefix(path, "project/"):
			projectName := strings.TrimPrefix(path, "project/")
			projectName = strings.TrimSuffix(projectName, "/")
			var entries []Entry
			for _, e := range data.Entries {
				if e.Project == projectName {
					entries = append(entries, e)
				}
			}
			// Newest first
			for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
				entries[i], entries[j] = entries[j], entries[i]
			}
			page := PageData{
				Title:       "Project — " + projectName,
				ProjectName: projectName,
				Entries:     entries,
				Dates:       getDates(data),
			}
			out, err := renderPage("project.html", templatesDir, page)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(out)

		case path == "standup" || path == "standup/":
			targetDate := time.Now()
			if dateStr := r.URL.Query().Get("date"); dateStr != "" {
				if t, err := time.Parse("2006-01-02", dateStr); err == nil {
					targetDate = t
				}
			}
			raw := formatDaily(targetDate)
			// Prev/next skip Sat/Sun (weekends are always Sat+Sun)
			prevDate := targetDate.AddDate(0, 0, -1)
			if targetDate.Weekday() == time.Monday {
				prevDate = targetDate.AddDate(0, 0, -3) // Fri
			} else if targetDate.Weekday() == time.Sunday {
				prevDate = targetDate.AddDate(0, 0, -2) // Fri
			}
			nextDate := targetDate.AddDate(0, 0, 1)
			if targetDate.Weekday() == time.Friday {
				nextDate = targetDate.AddDate(0, 0, 3) // Mon
			} else if targetDate.Weekday() == time.Saturday {
				nextDate = targetDate.AddDate(0, 0, 2) // Mon
			}
			nextURL := ""
			if !nextDate.After(time.Now()) {
				nextURL = "/standup?date=" + nextDate.Format("2006-01-02")
			}
			page := PageData{
				Title:          "Standup",
				ReportText:     raw,
				ReportHTML:     renderReportMarkdown(raw),
				Dates:          getDates(data),
				PrevURL:        "/standup?date=" + prevDate.Format("2006-01-02"),
				NextURL:        nextURL,
				ReportType:     "standup",
				ReportSubtitle: targetDate.Format("Monday, Jan 02, 2006"),
				ReportDateVal:  targetDate.Format("2006-01-02"),
			}
			out, err := renderPage("report.html", templatesDir, page)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(out)

		case path == "weekly" || path == "weekly/":
			now := time.Now()
			startDate, endDate := weekRange(now)
			if s := r.URL.Query().Get("start"); s != "" {
				if t, err := time.Parse("2006-01-02", s); err == nil {
					startDate = t
					if r.URL.Query().Get("end") == "" {
						endDate = startDate.AddDate(0, 0, 6)
					}
				}
			}
			if e := r.URL.Query().Get("end"); e != "" {
				if t, err := time.Parse("2006-01-02", e); err == nil {
					endDate = t
				}
			}
			raw := formatWeekly(startDate, endDate)
			prevStart := startDate.AddDate(0, 0, -7)
			prevEnd := endDate.AddDate(0, 0, -7)
			nextStart := startDate.AddDate(0, 0, 7)
			nextEnd := endDate.AddDate(0, 0, 7)
			nextURL := ""
			if !nextStart.After(now) {
				nextURL = fmt.Sprintf("/weekly?start=%s&end=%s", nextStart.Format("2006-01-02"), nextEnd.Format("2006-01-02"))
			}
			page := PageData{
				Title:          "Weekly",
				ReportText:     raw,
				ReportHTML:     renderReportMarkdown(raw),
				Dates:          getDates(data),
				PrevURL:        fmt.Sprintf("/weekly?start=%s&end=%s", prevStart.Format("2006-01-02"), prevEnd.Format("2006-01-02")),
				NextURL:        nextURL,
				ReportType:     "weekly",
				ReportSubtitle: startDate.Format("Jan 02") + " – " + endDate.Format("Jan 02, 2006"),
				ReportStartVal: startDate.Format("2006-01-02"),
				ReportEndVal:   endDate.Format("2006-01-02"),
			}
			out, err := renderPage("report.html", templatesDir, page)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(out)

		default:
			http.NotFound(w, r)
		}
	})

	// Reload config on SIGHUP (templates reload from disk on each request automatically)
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			cfg = loadConfig()
			fmt.Println("Config reloaded (SIGHUP)")
		}
	}()

	fmt.Printf("Serving at http://localhost:%d\n", port)
	fmt.Println("Press Ctrl+C to stop. Send SIGHUP to reload config.")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func build(outDir string, templatesDir string) {
	data := parseJournalFiles()
	dates := getDates(data)

	os.MkdirAll(outDir, 0o755)
	os.MkdirAll(filepath.Join(outDir, "daily"), 0o755)

	// Dashboard
	dash := buildDashboard(data)
	page := PageData{Title: "Dashboard", Dashboard: &dash, Dates: dates}
	out, _ := renderPage("dashboard.html", templatesDir, page)
	os.WriteFile(filepath.Join(outDir, "index.html"), out, 0o644)

	// Daily list
	listPage := PageData{Title: "Daily Entries", Dates: dates, Days: getDailyInfos(data)}
	out, _ = renderPage("daily-list.html", templatesDir, listPage)
	os.WriteFile(filepath.Join(outDir, "daily", "index.html"), out, 0o644)

	// Each daily entry
	for _, date := range dates {
		journalFile := filepath.Join(journalDir(), date+".md")
		content, err := os.ReadFile(journalFile)
		if err != nil {
			continue
		}
		htmlContent := autoLinkAll(renderMarkdown(string(content)))
		prevDate, nextDate := adjacentDates(date, dates)
		entryPage := PageData{
			Title:    "Journal — " + date,
			Content:  htmlContent,
			Dates:    dates,
			PrevDate: prevDate,
			NextDate: nextDate,
		}
		out, _ = renderPage("daily-entry.html", templatesDir, entryPage)
		os.MkdirAll(filepath.Join(outDir, "daily", date), 0o755)
		os.WriteFile(filepath.Join(outDir, "daily", date, "index.html"), out, 0o644)
	}

	fmt.Printf("Built %d pages to %s/\n", 2+len(dates), outDir)
}
