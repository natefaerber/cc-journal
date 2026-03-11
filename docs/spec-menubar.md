# cc-journal Menu Bar App — Spec

## Overview

A lightweight macOS menu bar app that provides quick access to journal entries, session stats, and common actions without opening a browser or terminal.

## Tech Stack Decision

**Recommended: SwiftUI + Go CLI**

The menu bar app is a thin SwiftUI shell that calls the existing `cc-journal` binary for data. This avoids duplicating parsing logic and stays in sync as the CLI evolves.

Alternatives considered:
- **Pure Go (systray/fyne)** — Cross-platform but poor macOS polish, no native popover support
- **Tauri** — Good but adds Rust + web stack for a simple menu
- **Electron** — Too heavy for a menu bar utility

## Architecture

```
┌──────────────────────┐
│  SwiftUI Menu Bar    │  ← Native macOS app
│  (popover + menus)   │
└──────────┬───────────┘
           │ shell out / JSON
┌──────────▼───────────┐
│  cc-journal CLI      │  ← Existing Go binary
│  (new: --json flag)  │
└──────────┬───────────┘
           │ reads
┌──────────▼───────────┐
│  ~/claude-journal/   │  ← Markdown files
│  *.md                │
└──────────────────────┘
```

The CLI needs a new output mode (`--json`) on key commands so the Swift app can parse structured data instead of scraping markdown.

## CLI Changes Required

### New flag: `--json`

Add `--json` output to these commands:

```sh
cc-journal stats --json        # New command: just the stats
cc-journal today --json        # Entries as JSON array
cc-journal list --json         # File list with metadata
cc-journal show DATE --json    # Entries for date as JSON
```

### JSON schemas

**`stats --json`**
```json
{
  "total_sessions": 142,
  "total_days": 38,
  "total_projects": 12,
  "this_week": 8,
  "streak": 5,
  "most_active": "cc-journal",
  "activity": [
    { "date": "2026-03-10", "count": 3 },
    { "date": "2026-03-09", "count": 5 }
  ]
}
```

**`today --json` / `show DATE --json`**
```json
{
  "date": "2026-03-10",
  "entries": [
    {
      "project": "cc-journal",
      "branch": "main",
      "time_range": "14:00–15:30",
      "session_id": "abc123-...",
      "cwd": "/path/to/project",
      "summary": "Built menu bar app spec...",
      "has_ai_summary": true
    }
  ]
}
```

**`list --json`**
```json
{
  "files": [
    { "date": "2026-03-10", "size": 3023, "entry_count": 5 },
    { "date": "2026-03-09", "size": 12446, "entry_count": 8 }
  ]
}
```

## Menu Bar UI

### Icon

A small monochrome icon in the menu bar. Two states:
- **Default**: Journal icon (book/page glyph)
- **New entry**: Brief pulse animation when a new session is journaled (watch `~/claude-journal/` for file changes)

### Popover (click to open)

```
┌─────────────────────────────────────────┐
│  cc-journal               ⚙️  🔗        │
├─────────────────────────────────────────┤
│                                         │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌──────────┐  │
│  │  8  │ │  5  │ │ 142 │ │ ▁▃█▅▂▇▃▁ │  │
│  │this │ │day  │ │total│ │ 4 weeks   │  │
│  │week │ │strk │ │sess │ │           │  │
│  └─────┘ └─────┘ └─────┘ └──────────┘  │
│                                         │
│  Today (3 sessions)                     │
│  ┌─────────────────────────────────────┐│
│  │ 🟢 cc-journal (main) 14:00–15:30   ││
│  │    Built menu bar app spec...       ││
│  │                            ▶ Resume ││
│  ├─────────────────────────────────────┤│
│  │ 🟢 chatty (testing) 12:00–12:45    ││
│  │    Fixed auth module tests...       ││
│  │                            ▶ Resume ││
│  ├─────────────────────────────────────┤│
│  │ 🟡 dotfiles (HEAD) 10:30–10:35     ││
│  │    Quick config update              ││
│  │                            ▶ Resume ││
│  └─────────────────────────────────────┘│
│                                         │
│  ───── Quick Actions ─────              │
│  📋 Copy standup          ⌘S           │
│  📋 Copy weekly           ⌘W           │
│  🌐 Open dashboard        ⌘D           │
│  📖 Browse all entries    ⌘B           │
│                                         │
└─────────────────────────────────────────┘
```

### Components

#### 1. Stats bar

Four compact stat cards in a row:
- **This week** — session count since Monday
- **Streak** — consecutive active days
- **Total** — all-time session count
- **Sparkline** — 28-day activity mini chart

Data source: `cc-journal stats --json`

#### 2. Today's sessions

Scrollable list of today's entries, each showing:
- **Color dot**: green = AI summary, yellow = fallback/short session
- **Project (branch)** and time range
- **Summary preview** — first ~80 chars of the summary
- **Resume button** — copies `cd /path && claude --resume SESSION_ID` to clipboard

Click an entry to expand the full summary in-place.

Data source: `cc-journal today --json`

#### 3. Quick actions

- **Copy standup** — runs `cc-journal standup --copy`, shows checkmark confirmation
- **Copy weekly** — runs `cc-journal weekly --copy`
- **Open dashboard** — launches `cc-journal serve` if not running, opens browser to localhost
- **Browse all entries** — opens a date picker or scrollable list of all dates

#### 4. Settings (gear icon)

- Path to `cc-journal` binary (auto-detected from PATH)
- Journal directory override
- Launch at login toggle
- Keyboard shortcut to open popover (default: `⌘⇧J`)

### Menu bar context menu (right-click)

```
Open Dashboard
Copy Standup
Copy Weekly Report
─────────────
Backfill Sessions...
─────────────
Settings
Quit
```

## Interactions

### Resume session

Clicking the resume button on any entry:
1. Copies `cd /path/to/project && claude --resume SESSION_ID` to clipboard
2. Shows a brief "Copied!" toast
3. Optionally: opens Terminal.app / iTerm / Alacritty with the command (configurable)

### File watching

Use `FSEvents` (macOS native) to watch `~/claude-journal/` for changes. On new/modified `.md` files:
1. Refresh the popover data
2. Brief icon animation (pulse)
3. Optional: macOS notification "Session journaled: project-name"

### Keyboard shortcuts

| Shortcut | Action |
|----------|--------|
| `⌘⇧J` | Toggle popover (global) |
| `⌘S` | Copy standup (when popover open) |
| `⌘W` | Copy weekly (when popover open) |
| `⌘D` | Open dashboard |
| `↑↓` | Navigate entries |
| `Enter` | Expand/collapse entry |
| `⌘C` | Copy resume command for selected entry |

## Data Refresh

- **On popover open**: Always re-fetch stats + today's entries (fast — just file reads)
- **On file change**: Re-fetch if popover is visible
- **Background**: No polling. FSEvents handles file changes. Stats are computed on demand.

## Build & Distribution

### Development

```sh
# SwiftUI app lives alongside the Go code
cc-journal/
├── app/                    # SwiftUI menu bar app
│   ├── CCJournal.xcodeproj
│   ├── CCJournal/
│   │   ├── App.swift       # @main, MenuBarExtra
│   │   ├── PopoverView.swift
│   │   ├── StatsView.swift
│   │   ├── EntryListView.swift
│   │   ├── EntryRow.swift
│   │   ├── SettingsView.swift
│   │   ├── CLIBridge.swift # Shell out to cc-journal CLI
│   │   └── FileWatcher.swift
│   └── CCJournalTests/
├── main.go
├── ...
```

### Distribution

- **Homebrew cask**: Install both the CLI and the menu bar app
- **DMG**: For standalone app distribution
- **Manual**: Build from source with Xcode

### Homebrew formula (future)

```ruby
cask "cc-journal" do
  version "0.1.0"
  url "https://github.com/natefaerber/cc-journal/releases/download/v#{version}/CCJournal.dmg"
  name "cc-journal"
  desc "Menu bar app for Claude Code developer journal"
  homepage "https://github.com/natefaerber/cc-journal"
  depends_on formula: "cc-journal"  # CLI dependency
  app "CCJournal.app"
end
```

## Implementation Phases

### Phase 1: CLI JSON output

Add `--json` flag to `stats`, `today`, `show`, `list` commands. This unblocks the Swift app and is independently useful for scripting.

### Phase 2: Basic menu bar app

- Menu bar icon with popover
- Stats display (this week, streak, total)
- Today's entries with summary previews
- Resume button (copy to clipboard)
- Settings (binary path, launch at login)

### Phase 3: Quick actions

- Copy standup / weekly
- Open dashboard (auto-start server)
- Browse entries with date navigation

### Phase 4: Polish

- FSEvents file watching with icon animation
- Global keyboard shortcut
- Notification on new journal entries
- Sparkline chart in stats bar
- Entry search / filter by project

## Open Questions

1. **Should the dashboard server run persistently?** The menu bar app could manage a background `cc-journal serve` process, or we could embed the Go HTTP server directly via a local socket.

2. **Search?** Full-text search across journal entries would be useful but adds complexity. Could be a Phase 5 feature or delegate to `grep` / `rg`.

3. **Multiple machines?** If journal files are synced (iCloud, Syncthing), the app works anywhere. No special handling needed since it reads plain markdown files.
