# cc-journal

A single Go binary that captures, summarizes, and visualizes your Claude Code sessions as a developer journal.

<p align="center">
  <img src="docs/images/architecture.png" alt="cc-journal architecture" width="800">
</p>

```
Claude Code session ends
  → SessionEnd hook reads transcript JSONL
  → Calls Anthropic API for AI summary
  → Appends to ~/claude-journal/YYYY-MM-DD.md
  → cc-journal serves a dashboard or generates static HTML
```

## Install

### From release

Download the latest binary from [Releases](https://github.com/natefaerber/cc-journal/releases) and place it in your PATH:

```sh
# macOS (Apple Silicon)
curl -sL https://github.com/natefaerber/cc-journal/releases/latest/download/cc-journal_darwin_arm64.tar.gz | tar xz
mv cc-journal ~/.local/bin/

# macOS (Intel)
curl -sL https://github.com/natefaerber/cc-journal/releases/latest/download/cc-journal_darwin_amd64.tar.gz | tar xz
mv cc-journal ~/.local/bin/

# Linux (amd64)
curl -sL https://github.com/natefaerber/cc-journal/releases/latest/download/cc-journal_linux_amd64.tar.gz | tar xz
mv cc-journal ~/.local/bin/
```

### With mise

```sh
mise use -g github:natefaerber/cc-journal
```

### With an agent

Paste the following prompt into a coding agent (Claude Code, etc.) to have it install and configure cc-journal end-to-end:

```
Install and configure cc-journal (https://github.com/natefaerber/cc-journal),
a Go CLI that auto-summarizes my Claude Code sessions into a developer journal.
Do the following, checking each step before moving on:

1. INSTALL the binary onto my PATH. Pick whichever fits my machine:
   - `go install github.com/natefaerber/cc-journal@latest` if Go 1.24+ is present, OR
   - download the right release asset for my OS/arch from
     https://github.com/natefaerber/cc-journal/releases/latest
     (cc-journal_{darwin,linux}_{arm64,amd64}.tar.gz), untar it, and move the
     `cc-journal` binary into a directory on my PATH (e.g. ~/.local/bin).
   Verify with `cc-journal --help`.

2. API KEY. cc-journal resolves a key in this order: fnox → CC_JOURNAL_API_KEY →
   ANTHROPIC_API_KEY → config.yaml. Check whether one is already available
   (e.g. `echo $ANTHROPIC_API_KEY`, `fnox get ANTHROPIC_API_KEY`). If none is
   set, STOP and ask me to provide one rather than inventing it. Confirm with
   `cc-journal debug-key`.

3. SESSIONEND HOOK. Merge this into ~/.claude/settings.json without clobbering
   any existing hooks or settings (parse the JSON, add to hooks.SessionEnd):

     {
       "hooks": {
         "SessionEnd": [
           {
             "hooks": [
               {
                 "type": "command",
                 "command": "TMP=$(mktemp /tmp/cc-journal-in.XXXXXX); cat > \"$TMP\"; ( cc-journal hook < \"$TMP\" >/tmp/cc-journal.log 2>&1; rm -f \"$TMP\" ) &",
                 "timeout": 1
               }
             ]
           }
         ]
       }
     }

   The hook reads stdin synchronously, then backgrounds the API call so it does
   not block session exit.

4. (Optional) BACKFILL my recent sessions so the journal isn't empty:
   `cc-journal backfill --days 7 --dry-run` first, then without --dry-run if it
   looks right.

5. VERIFY. End a Claude Code session (or run the backfill above), then show me
   `cc-journal today`. Tell me to run `cc-journal serve` to open the dashboard.

Report what you did at each step. If anything is ambiguous or a key is missing,
ask me instead of guessing.
```

### From source

```sh
go install github.com/natefaerber/cc-journal@latest
```

Or build locally:

```sh
git clone https://github.com/natefaerber/cc-journal.git
cd cc-journal
go build -o cc-journal .
```

## Setup

### 1. API key

cc-journal checks for an API key in this order:

1. [fnox](https://github.com/jdx/fnox) encrypted key store
2. `CC_JOURNAL_API_KEY` environment variable
3. `ANTHROPIC_API_KEY` environment variable
4. `api_key` in config.yaml

```sh
# Option A: fnox (recommended)
fnox set ANTHROPIC_API_KEY

# Option B: dedicated env var
export CC_JOURNAL_API_KEY=sk-ant-...

# Option C: shared env var
export ANTHROPIC_API_KEY=sk-ant-...
```

### 2. SessionEnd hook

Add to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "TMP=$(mktemp /tmp/cc-journal-in.XXXXXX); cat > \"$TMP\"; ( cc-journal hook < \"$TMP\" >/tmp/cc-journal.log 2>&1; rm -f \"$TMP\" ) &",
            "timeout": 1
          }
        ]
      }
    ]
  }
}
```

The hook captures stdin synchronously, then backgrounds the API call so it doesn't block session exit.

### 3. Verify

End a Claude Code session, then check:

```sh
cc-journal today
```

## Commands

### Site

<p align="center">
  <img src="docs/images/ui-flow.png" alt="cc-journal dashboard and UI" width="800">
</p>

```sh
cc-journal serve [--port 8000] [--templates DIR]   # Dev server with live data
cc-journal build [--out public] [--templates DIR]   # Static HTML generation
```

Send `kill -HUP` to the serve process to reload `config.yaml` without restarting. Templates reload from disk on each request automatically.

Routes:

| Route | Description |
|-------|-------------|
| `/` | Dashboard — stats, activity chart, heatmap, projects, recent sessions |
| `/daily` | Date index with session counts, time spans, and project names |
| `/daily/YYYY-MM-DD` | Single day view with session navigation |
| `/project/NAME` | All sessions for a project |
| `/standup` | Daily standup report |
| `/weekly` | Weekly status report |
| `/api/palette` | JSON endpoint for command palette data |

### Journal

```sh
cc-journal hook                              # SessionEnd hook (reads JSON from stdin)
cc-journal summarize [SESSION_ID] [--force]  # On-demand summary
cc-journal backfill [--days 30] [--dry-run]  # Retroactively summarize old sessions
cc-journal prune [--dry-run]                 # Remove failed summary entries
cc-journal remove SESSION_ID                 # Delete entry + deny from future backfills
```

### Browse

```sh
cc-journal today                       # Print today's entries
cc-journal show YYYY-MM-DD             # Print a specific date
cc-journal list                        # List all journal files
cc-journal week [DATE] [--rollup]      # This week's entries or AI rollup
cc-journal rollup [DATE]               # Generate AI weekly rollup
```

### Reports

```sh
cc-journal standup [DATE] [--copy] [--slack [CHANNEL]]          # Daily standup (default: today)
cc-journal weekly  [START] [--end END] [--copy] [--slack [CHANNEL]]  # Weekly status (default: this week)
```

`--copy` copies to clipboard. `--slack` sends to Slack (channel overrides config default).

### Customization

```sh
cc-journal init                                 # Export templates + prompts to filesystem
cc-journal init --templates                     # Export templates only
cc-journal init --prompts                       # Export prompts only
cc-journal init --force                         # Overwrite existing files
cc-journal init --stdout                        # Print to stdout instead of writing files
cc-journal init --templates --stdout            # Print templates to stdout
```

## Keyboard navigation

The web UI supports full keyboard navigation:

| Key | Context | Action |
|-----|---------|--------|
| `Cmd+K` / `Ctrl+K` | Global | Open command palette |
| `j` / `k` | Daily pages | Navigate between sessions or dates |
| `[` / `]` | Daily entry | Previous / next day |
| `y` | Focused session | Copy permalink |
| `r` | Focused session | Copy `claude --resume` command |
| `x` | Focused session | Delete entry |
| `g` / `G` | Global | Scroll to top / bottom |
| `h` / `l` | Global | Browser back / forward |
| `Enter` | Daily list / palette | Open selected item |
| `Escape` | Global | Close palette, clear focus |
| `↑` / `↓` | Palette | Navigate results |

The command palette searches across all pages, projects, dates, and recent sessions.

## Menu bar app

A lightweight macOS menu bar app (`app/`, SwiftUI) gives quick access to stats, today's entries, and common actions without opening a browser. It's a thin shell that shells out to the `cc-journal` binary for data, so it stays in sync as the CLI evolves. Requires macOS 14+.

### Build and install

```sh
mise run app:run          # run in debug (foreground) to try it out
mise run app:install-app  # build CCJournal.app, install to /Applications, restart the login LaunchAgent
```

`app:install-app` assembles `CCJournal.app` (via `app:bundle` → `app/scripts/make-app.sh`), copies it to `/Applications`, and — if the LaunchAgent below is installed — restarts it. Run it again any time after changing the Swift sources to rebuild and re-sync. The app runs as an `LSUIElement` (menu bar only, no Dock icon).

### Using it

Click the cc-journal icon in the menu bar to open the popover. Under **Quick Actions**:

- **Open dashboard** (`D`) — opens `http://localhost:8000`, auto-starting the web server first if it isn't running.
- **Browse entries** (`B`) — same, but opens `/browse`.
- **Start server** / **Stop server** (`R`) — toggle the `cc-journal serve` process manually. The status dot in the popover header shows the state; server logs go to `~/.config/cc-journal/logs/server.log`.

To start the server automatically when the app launches, open the popover's settings (gear icon) → **Server** → enable **Start server on app launch**. Note the app and the web server are separate processes: the app hosts the menu bar UI, and starts the server on demand.

### Start at login (LaunchAgent)

Create `~/Library/LaunchAgents/com.ccjournal.app.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.ccjournal.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Applications/CCJournal.app/Contents/MacOS/CCJournal</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
    <key>ProcessType</key>
    <string>Interactive</string>
</dict>
</plist>
```

Then load it:

```sh
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.ccjournal.app.plist
launchctl kickstart -k gui/$(id -u)/com.ccjournal.app   # start now without waiting for next login
```

`KeepAlive: false` means quitting the app from its menu keeps it quit until the next login; set it to `true` to always relaunch. The agent appears in **System Settings → General → Login Items** under "Allow in the Background". To stop it and disable autostart: `launchctl bootout gui/$(id -u)/com.ccjournal.app`, then delete the plist.

## Journal format

Each `~/claude-journal/YYYY-MM-DD.md` file:

```markdown
# Claude Code Journal — 2026-03-10

---

## my-project (main) — 14:00–15:30

### Done
- Built the auth module

### Decisions
- Chose JWT over session cookies

### Open
- Need to add refresh token logic

<details>
<summary>Session ID</summary>
<code>abc123-...</code>
<code>/path/to/project</code>
<code>tokens:in=161,out=20308,cache_create=251602,cache_read=7955592,summary_in=1500,summary_out=512</code>
</details>
```

Token usage (input, output, cache create, cache read) is parsed from Claude Code JSONL transcripts. Summarizer API tokens are tracked separately. Old entries without token data display as zero.

Weekly rollups are saved as `~/claude-journal/YYYY-WXX-rollup.md`.

## Configuration

All settings live in `$XDG_CONFIG_HOME/cc-journal/config.yaml` (defaults to `~/.config/cc-journal/config.yaml`):

```yaml
# Directory containing daily journal markdown files.
# Default: ~/claude-journal
journal_dir: ~/claude-journal

# Directory containing custom prompt templates.
# Default: ~/.config/cc-journal/prompts
prompt_dir: ~/.config/cc-journal/prompts

# Anthropic model for summarization.
# Default: claude-sonnet-4-20250514
model: claude-sonnet-4-20250514

# API key (prefer fnox or CC_JOURNAL_API_KEY env var instead).
# api_key: sk-ant-...

# First day of the week for reports and dashboard.
# Default: monday
week_start: monday

# Directories to exclude from journal summarization.
exclude:
  - ~/private-project

# Slack integration for report commands.
slack:
  command: /path/to/slack-cli
  channel: "#standup"

# Auto-linking for issue trackers, PRs, and wikis.
links:
  issues:
    LPE: https://linear.app/myorg/issue
    PROJ: https://myorg.atlassian.net/browse
  confluence: https://myorg.atlassian.net/wiki
  github_repos:
    - myorg/main-repo
    - myorg/other-repo
```

Environment variables override config file values:

| Variable | Overrides | Description |
|----------|-----------|-------------|
| `JOURNAL_DIR` | `journal_dir` | Journal file directory |
| `CC_JOURNAL_API_KEY` | `api_key` | Dedicated API key for cc-journal |
| `ANTHROPIC_API_KEY` | `api_key` | Shared Anthropic API key (lower priority) |

## Template customization

Templates are embedded in the binary but can be overridden at runtime:

```sh
# Export defaults as a starting point
cc-journal init --templates

# Or preview without writing files
cc-journal init --templates --stdout

# Edit, then serve with overrides
cc-journal serve --templates ~/.config/cc-journal/templates
```

Templates use [Tailwind CSS v4](https://tailwindcss.com/) (CDN) and [Source Serif 4](https://fonts.google.com/specimen/Source+Serif+4) for a warm, notebook-inspired theme.

| Variable | Default | Description |
|----------|---------|-------------|
| `--color-background` | `#f5f0e8` | Cream page background |
| `--color-foreground` | `#1a1a1a` | Primary text |
| `--color-card` | `#fffdf7` | Card background |
| `--color-accent` | `#2563eb` | Blue accent |
| `--color-secondary` | `#7c3aed` | Purple secondary |

## Prompt customization

All AI and report prompts can be customized per-installation:

```sh
# Export all default prompts
cc-journal init --prompts

# Edit any prompt in ~/.config/cc-journal/prompts/
```

| Prompt | Purpose | Template Variables |
|--------|---------|-------------------|
| `summary.txt` | Session summarization (Anthropic API) | `{{.Project}}`, `{{.Branch}}`, `{{.Transcript}}` |
| `rollup.txt` | Weekly rollup generation (Anthropic API) | `{{.Week}}`, `{{.Content}}` |
| `standup.txt` | Daily standup report format (Go text/template) | `.DateLabel`, `.YesterdayGroups`, `.TodayGroups`, `.OpenItems`, `.Links` |
| `weekly.txt` | Weekly status report format (Go text/template) | `.WeekLabel`, `.Groups`, `.Decisions`, `.OpenItems`, `.Links`, `.TotalSessions`, `.TotalProjects`, `.ActiveDays`, `.TotalTokensIn`, `.TotalTokensOut` |

Report templates (`standup.txt`, `weekly.txt`) use Go's `text/template` syntax with full access to loops, conditionals, and the pre-computed data structs.

## Project structure

```
.
├── main.go              # CLI entry point + command dispatch
├── config.go            # Config loading (XDG_CONFIG_HOME/cc-journal/config.yaml)
├── server.go            # HTTP server, static builder, template loading
├── journal.go           # Journal parsing, stats, dashboard data
├── summarize.go         # Transcript parsing, Anthropic API, prompt loading
├── hook.go              # SessionEnd hook handler
├── backfill.go          # Retroactive session summarization
├── browse.go            # today/show/list/week/rollup commands
├── status.go            # Standup + weekly report formatters
├── links.go             # Auto-linking (issues, PRs, Confluence)
├── deny.go              # Session deny list management
├── prune.go             # Failed entry cleanup
├── init.go              # Template/prompt export (init command)
├── templates/
│   ├── base.html        # Layout, nav, command palette, keyboard nav
│   ├── dashboard.html   # Stats, bar chart, heatmap, projects, recent sessions
│   ├── daily-list.html  # Date index with session counts
│   ├── daily-entry.html # Single day with session focus/nav
│   ├── project.html     # Per-project session list
│   └── report.html      # Standup/weekly report display
├── app/                 # SwiftUI menu bar app (CCJournal.app)
│   ├── Sources/         # Swift sources (popover, CLI bridge, server manager)
│   ├── Info.plist       # Bundle metadata (LSUIElement menu bar app)
│   └── scripts/
│       └── make-app.sh  # Assemble CCJournal.app from the release binary
├── docs/
│   └── spec-menubar.md  # Menu bar app spec
├── .github/
│   └── workflows/
│       ├── ci.yaml      # Build + test + lint on push/PR
│       └── release.yaml # GoReleaser on tag push
├── .goreleaser.yaml     # Cross-platform binary builds
├── install.sh           # Local build + install script
├── go.mod
└── go.sum
```

## Development

```sh
go build -o cc-journal .
go test -v -race ./...
go vet ./...
```

Local install with version info:

```sh
./install.sh
```

### Releasing

Tag a version to trigger the release workflow:

```sh
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser builds binaries for linux/darwin on amd64/arm64 and creates a GitHub release with checksums.

## Dependencies

- **Go 1.24+**
- **[goldmark](https://github.com/yuin/goldmark)** — markdown rendering (+ table extension)
- **[gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)** — config file parsing
- **[Tailwind CSS v4](https://tailwindcss.com/)** — styling (CDN, no build step)
- **[fnox](https://github.com/jdx/fnox)** — secret management (optional)

## Troubleshooting

**Hook not firing?** Ensure `~/.claude/settings.json` has the `SessionEnd` hook (not `Stop` — `Stop` fires after every response). Check `/tmp/cc-journal.log`.

**401 Unauthorized?** Run `cc-journal debug-key` to diagnose. The key priority is: fnox → `CC_JOURNAL_API_KEY` → `ANTHROPIC_API_KEY` → config file. A stale env var can shadow a valid fnox key.

**Template depth errors?** Page templates must not contain `{{template "base" .}}` — the binary calls `ExecuteTemplate("base", data)` directly.

**Backfill skipping sessions?** Sessions on the deny list (via `cc-journal remove`) are permanently excluded. Run `cc-journal backfill --dry-run` to preview what would be processed.
