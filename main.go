package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v3"
)

var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func main() {
	cfg = loadConfig()

	app := &cli.Command{
		Name:    "cc-journal",
		Usage:   "Claude Code developer journal",
		Version: fmt.Sprintf("%s (%s) built %s", version, commit, buildTime),
		Suggest: true,
		Commands: []*cli.Command{
			// Site commands
			{
				Name:     "serve",
				Usage:    "Start dev server with live reload. Send SIGHUP to reload config.",
				Category: "Site",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "port", Value: 8000, Usage: "port to listen on"},
					&cli.StringFlag{Name: "templates", Usage: "custom templates directory"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					serve(int(cmd.Int("port")), cmd.String("templates"))
					return nil
				},
			},
			{
				Name:     "build",
				Usage:    "Generate static HTML site",
				Category: "Site",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "out", Value: "public", Usage: "output directory"},
					&cli.StringFlag{Name: "templates", Usage: "custom templates directory"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					build(cmd.String("out"), cmd.String("templates"))
					return nil
				},
			},

			// Journal commands
			{
				Name:     "hook",
				Usage:    "SessionEnd hook (reads JSON from stdin)",
				Category: "Journal",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runHook()
					return nil
				},
			},
			{
				Name:      "summarize",
				Usage:     "Summarize a session and write to journal",
				Category:  "Journal",
				ArgsUsage: "[SESSION_ID]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "session", Usage: "session ID to summarize"},
					&cli.BoolFlag{Name: "force", Usage: "re-summarize even if already journaled"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					sessionID := cmd.String("session")
					if sessionID == "" && cmd.NArg() > 0 {
						sessionID = cmd.Args().First()
					}
					summarizeSession(sessionID, cmd.Bool("force"))
					return nil
				},
			},
			{
				Name:     "backfill",
				Usage:    "Retroactively summarize recent sessions",
				Category: "Journal",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "days", Value: 30, Usage: "number of days to look back"},
					&cli.BoolFlag{Name: "dry-run", Usage: "show what would be summarized"},
					&cli.BoolFlag{Name: "force", Usage: "re-summarize already-journaled sessions"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runBackfill(int(cmd.Int("days")), cmd.Bool("dry-run"), cmd.Bool("force"))
					return nil
				},
			},
			{
				Name:     "prune",
				Usage:    "Remove failed summary entries",
				Category: "Journal",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "dry-run", Usage: "show what would be pruned"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runPrune(cmd.Bool("dry-run"))
					return nil
				},
			},
			{
				Name:      "remove",
				Usage:     "Delete entry and deny from future backfills",
				Category:  "Journal",
				ArgsUsage: "<SESSION_ID>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					sessionID := ""
					if cmd.NArg() > 0 {
						sessionID = cmd.Args().First()
					}
					runRemove(sessionID)
					return nil
				},
			},

			// Browse commands
			{
				Name:     "today",
				Usage:    "Print today's journal entries",
				Category: "Browse",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					showToday()
					return nil
				},
			},
			{
				Name:      "show",
				Usage:     "Print a specific date's journal entries",
				Category:  "Browse",
				ArgsUsage: "<DATE>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.NArg() == 0 {
						return fmt.Errorf("date required (YYYY-MM-DD)")
					}
					showDate(cmd.Args().First())
					return nil
				},
			},
			{
				Name:     "list",
				Usage:    "List all journal files",
				Category: "Browse",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					listEntries()
					return nil
				},
			},
			{
				Name:      "search",
				Usage:     "Search journal entries by text",
				Category:  "Browse",
				ArgsUsage: "<QUERY>...",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "project", Aliases: []string{"p"}, Usage: "filter by project name"},
					&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 20, Usage: "max results"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runSearchCLI(cmd.Args().Slice(), cmd.String("project"), int(cmd.Int("limit")))
					return nil
				},
			},
			{
				Name:      "week",
				Usage:     "Print this week's entries or generate rollup",
				Category:  "Browse",
				ArgsUsage: "[DATE]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "date", Usage: "week containing this date"},
					&cli.BoolFlag{Name: "rollup", Usage: "generate AI-powered weekly rollup"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					date := cmd.String("date")
					if date == "" && cmd.NArg() > 0 {
						date = cmd.Args().First()
					}
					if cmd.Bool("rollup") {
						generateRollup(date)
					} else {
						showWeek(date)
					}
					return nil
				},
			},
			{
				Name:      "rollup",
				Usage:     "Generate AI-powered weekly rollup",
				Category:  "Browse",
				ArgsUsage: "[DATE]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					date := ""
					if cmd.NArg() > 0 {
						date = cmd.Args().First()
					}
					generateRollup(date)
					return nil
				},
			},

			// Report commands
			{
				Name:      "standup",
				Usage:     "Print daily standup report",
				Category:  "Report",
				ArgsUsage: "[DATE]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "date", Usage: "date (YYYY-MM-DD, default: today)"},
					&cli.BoolFlag{Name: "copy", Usage: "copy to clipboard"},
					&cli.StringFlag{Name: "slack", Usage: "send to Slack channel"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					target := time.Now()
					dateStr := cmd.String("date")
					if dateStr == "" && cmd.NArg() > 0 {
						dateStr = cmd.Args().First()
					}
					if dateStr != "" {
						t, err := time.Parse("2006-01-02", dateStr)
						if err != nil {
							return fmt.Errorf("invalid date: %s (use YYYY-MM-DD)", dateStr)
						}
						target = t
					}
					output := formatDaily(target)
					fmt.Print(output)
					return handleOutput(cmd, output)
				},
			},
			{
				Name:     "weekly",
				Usage:    "Print weekly status report",
				Category: "Report",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "start", Usage: "start date (YYYY-MM-DD)"},
					&cli.StringFlag{Name: "end", Usage: "end date (YYYY-MM-DD)"},
					&cli.BoolFlag{Name: "copy", Usage: "copy to clipboard"},
					&cli.StringFlag{Name: "slack", Usage: "send to Slack channel"},
				},
				ArgsUsage: "[START]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					now := time.Now()
					start, end := weekRange(now)
					startStr := cmd.String("start")
					if startStr == "" && cmd.NArg() > 0 {
						startStr = cmd.Args().First()
					}
					if startStr != "" {
						t, err := time.Parse("2006-01-02", startStr)
						if err != nil {
							return fmt.Errorf("invalid start date: %s (use YYYY-MM-DD)", startStr)
						}
						start = t
					}
					if endStr := cmd.String("end"); endStr != "" {
						t, err := time.Parse("2006-01-02", endStr)
						if err != nil {
							return fmt.Errorf("invalid end date: %s (use YYYY-MM-DD)", endStr)
						}
						end = t
					}
					output := formatWeekly(start, end)
					fmt.Print(output)
					return handleOutput(cmd, output)
				},
			},

			// Setup commands
			{
				Name:     "init",
				Usage:    "Export default templates, prompts, and Claude Code commands",
				Category: "Setup",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "templates", Usage: "export HTML templates only"},
					&cli.BoolFlag{Name: "prompts", Usage: "export prompt templates only"},
					&cli.BoolFlag{Name: "commands", Usage: "install Claude Code slash commands"},
					&cli.BoolFlag{Name: "all", Usage: "export templates, prompts, and commands"},
					&cli.BoolFlag{Name: "force", Usage: "overwrite existing files"},
					&cli.BoolFlag{Name: "stdout", Usage: "print to stdout instead of writing files"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runInitCLI(cmd)
					return nil
				},
			},

			// Debug (hidden)
			{
				Name:   "debug-key",
				Hidden: true,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					debugKey()
					return nil
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// handleOutput processes --copy and --slack flags for report commands.
func handleOutput(cmd *cli.Command, output string) error {
	if cmd.Bool("copy") {
		if err := copyToClipboard(output); err != nil {
			return fmt.Errorf("failed to copy: %w", err)
		}
		fmt.Println("\n\nCopied to clipboard")
	}

	if cmd.IsSet("slack") {
		channel := cmd.String("slack")
		if err := sendToSlack(output, channel); err != nil {
			return fmt.Errorf("failed to send to Slack: %w", err)
		}
	}
	return nil
}

// debugKey tests API key configuration.
func debugKey() {
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
}
