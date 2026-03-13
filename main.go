package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("cc-journal %s (%s) built %s\n", version, commit, buildTime)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "debug-key" {
		cfg = loadConfig()
		fmt.Printf("cfg.APIKey set: %v (len=%d)\n", cfg.APIKey != "", len(cfg.APIKey))
		if cfg.APIKey != "" {
			fmt.Printf("cfg.APIKey prefix: %s...\n", cfg.APIKey[:min(15, len(cfg.APIKey))])
		}
		key, err := getAPIKey()
		if err != nil {
			fmt.Printf("getAPIKey error: %v\n", err)
			return
		}
		fmt.Printf("getAPIKey result: %s... (len=%d)\n", key[:min(15, len(key))], len(key))
		// Quick validation
		req, _ := http.NewRequest("POST", apiURL, bytes.NewReader([]byte(`{"model":"claude-sonnet-4-20250514","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)))
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", apiVersion)
		req.Header.Set("content-type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("API test error: %v\n", err)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		fmt.Printf("API test: HTTP %d\n", resp.StatusCode)
		return
	}

	cfg = loadConfig()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "serve":
		port := 8000
		for i, a := range args {
			if a == "--port" && i+1 < len(args) {
				if p, err := strconv.Atoi(args[i+1]); err == nil {
					port = p
				}
			}
		}
		templatesDir := flagValue(args, "--templates")
		serve(port, templatesDir)

	case "build":
		outDir := flagValue(args, "--out")
		if outDir == "" {
			outDir = "public"
		}
		templatesDir := flagValue(args, "--templates")
		build(outDir, templatesDir)

	case "standup":
		target := time.Now()
		if dateArg := positionalOrFlag(args, "--date"); dateArg != "" {
			if t, err := time.Parse("2006-01-02", dateArg); err == nil {
				target = t
			} else {
				fmt.Fprintf(os.Stderr, "Invalid date: %s (use YYYY-MM-DD)\n", dateArg)
				os.Exit(1)
			}
		}
		output := formatDaily(target)
		fmt.Print(output)
		handleOutputActions(args, output)

	case "weekly":
		now := time.Now()
		start, end := weekRange(now)
		if startArg := positionalOrFlag(args, "--start"); startArg != "" {
			if t, err := time.Parse("2006-01-02", startArg); err == nil {
				start = t
			} else {
				fmt.Fprintf(os.Stderr, "Invalid start date: %s (use YYYY-MM-DD)\n", startArg)
				os.Exit(1)
			}
		}
		if endArg := flagValue(args, "--end"); endArg != "" {
			if t, err := time.Parse("2006-01-02", endArg); err == nil {
				end = t
			} else {
				fmt.Fprintf(os.Stderr, "Invalid end date: %s (use YYYY-MM-DD)\n", endArg)
				os.Exit(1)
			}
		}
		output := formatWeekly(start, end)
		fmt.Print(output)
		handleOutputActions(args, output)

	case "summarize":
		sessionID := flagValue(args, "--session")
		if sessionID == "" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			sessionID = args[0]
		}
		force := hasFlag(args, "--force")
		summarizeSession(sessionID, force)

	case "hook":
		runHook()

	case "backfill":
		days := 30
		if v := flagValue(args, "--days"); v != "" {
			if d, err := strconv.Atoi(v); err == nil {
				days = d
			}
		}
		dryRun := hasFlag(args, "--dry-run")
		force := hasFlag(args, "--force")
		runBackfill(days, dryRun, force)

	case "today":
		showToday()

	case "show":
		date := ""
		if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			date = args[0]
		}
		if date == "" {
			fmt.Fprintln(os.Stderr, "Usage: cc-journal-site show YYYY-MM-DD")
			os.Exit(1)
		}
		showDate(date)

	case "list":
		listEntries()

	case "week":
		date := flagValue(args, "--date")
		if date == "" && len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			date = args[0]
		}
		if hasFlag(args, "--rollup") {
			generateRollup(date)
		} else {
			showWeek(date)
		}

	case "rollup":
		date := ""
		if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			date = args[0]
		}
		generateRollup(date)

	case "prune":
		dryRun := hasFlag(args, "--dry-run")
		runPrune(dryRun)

	case "remove":
		sessionID := ""
		if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
			sessionID = args[0]
		}
		runRemove(sessionID)

	case "init":
		runInit(args)

	case "search":
		runSearch(args)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `cc-journal-site — Claude Code developer journal

Usage:
  cc-journal-site <command> [options]

Site Commands:
  serve     [--port 8000] [--templates DIR]  Start dev server with live reload
  build     [--out public] [--templates DIR]  Generate static HTML

Journal Commands:
  hook                          SessionEnd hook (reads JSON from stdin)
  summarize [SESSION_ID]        Summarize a session and write to journal
  backfill  [--days 30] [--dry-run] [--force]  Summarize existing sessions
  prune     [--dry-run]         Remove failed summary entries
  remove    SESSION_ID          Delete entry + deny from future backfills

Browse Commands:
  today                         Print today's journal entries
  show DATE                     Print a specific date's entries
  list                          List all journal files
  search QUERY [--project P] [--limit N]  Search entries by text
  week [DATE] [--rollup]        Print this week's entries (or generate rollup)
  rollup [DATE]                 Generate AI-powered weekly rollup

Report Commands:
  standup   [DATE] [--copy] [--slack [CHANNEL]]  Print daily standup (default: today)
  weekly    [START] [--end END] [--copy] [--slack [CHANNEL]]  Print weekly status (default: this week)

Setup:
  init      [--templates] [--prompts] [--force] [--stdout]  Export defaults for customization

Other:
  version                       Print version information
`)
}

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// positionalOrFlag returns the first positional arg (non-flag) or the value of the named flag.
func positionalOrFlag(args []string, flag string) string {
	if v := flagValue(args, flag); v != "" {
		return v
	}
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			return a
		}
	}
	return ""
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// handleOutputActions processes --copy and --slack flags for report commands.
func handleOutputActions(args []string, output string) {
	if hasFlag(args, "--copy") {
		if err := copyToClipboard(output); err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to copy: %v\n", err)
		} else {
			fmt.Println("\n\nCopied to clipboard")
		}
	}

	if hasFlag(args, "--slack") {
		// --slack '#channel' overrides config default
		channel := flagValue(args, "--slack")
		if channel != "" && strings.HasPrefix(channel, "--") {
			channel = "" // next arg is another flag, not a channel
		}
		if err := sendToSlack(output, channel); err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to send to Slack: %v\n", err)
			os.Exit(1)
		}
	}
}
