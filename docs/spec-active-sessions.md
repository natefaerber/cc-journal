# Active Sessions Tracking — Spec

## Overview

Track live Claude Code sessions so the menu bar app can show "in progress" indicators next to active projects. Uses SessionStart/SessionEnd hooks to maintain a state file that the app watches.

## Architecture

```
┌──────────────────────┐
│  Claude Code         │  ← SessionStart hook fires
│  (active session)    │
└──────────┬───────────┘
           │ writes
┌──────────▼───────────┐
│  active-sessions.json│  ← State file
│  ~/.config/cc-journal│
└──────────┬───────────┘
           │ FSEvents
┌──────────▼───────────┐
│  Menu Bar App        │  ← Shows live indicators
│  (popover)           │
└──────────────────────┘
```

## State File

**Location**: `~/.config/cc-journal/active-sessions.json`

```json
{
  "sessions": [
    {
      "session_id": "abc123-def456-...",
      "project": "cc-journal",
      "cwd": "/Users/nate/_Work/github/natefaerber/cc-journal",
      "branch": "feat/menubar-app",
      "started_at": "2026-03-18T14:00:00Z",
      "pid": 12345
    }
  ]
}
```

### Fields

- **session_id**: Claude Code session UUID
- **project**: Derived from cwd (directory basename or git remote name)
- **cwd**: Working directory of the session
- **branch**: Current git branch (if in a git repo)
- **started_at**: ISO 8601 timestamp when session started
- **pid**: Process ID of the Claude Code process (for stale detection)

## CLI Changes

### Hook: `cc-journal hook --event session-start`

Called by Claude Code's SessionStart hook. Reads session metadata from stdin (same JSON format as SessionEnd), adds an entry to `active-sessions.json`.

```sh
# Claude Code hook config (~/.claude/settings.json)
{
  "hooks": {
    "SessionStart": [
      {
        "type": "command",
        "command": "cc-journal hook --event session-start"
      }
    ]
  }
}
```

### Hook: `cc-journal hook --event session-end`

Called by Claude Code's SessionEnd hook. Removes the session entry from `active-sessions.json`. The existing `cc-journal hook` (which journals the session) continues to work — this is an additional event type.

### Stale Session Cleanup

Sessions can become stale if Claude Code crashes without firing SessionEnd. On each read of `active-sessions.json`:

1. Check if the `pid` is still running (`kill(pid, 0)`)
2. Remove entries with dead PIDs
3. Remove entries older than 24 hours as a safety net

This cleanup runs in both the CLI (on hook invocation) and the menu bar app (on file read).

## Menu Bar App Changes

### Popover: Active Session Indicators

Entries in the popover get a visual indicator when there's a matching active session:

```
┌─────────────────────────────────────┐
│ 🔵 cc-journal (feat/menubar-app)   │
│    Active — started 14:00           │
│                          ▶ Resume   │
├─────────────────────────────────────┤
│ 🟢 chatty (main) 12:00–12:45       │
│    Fixed auth module tests...       │
│                          ▶ Resume   │
└─────────────────────────────────────┘
```

- **Blue pulsing dot**: Active session in progress
- **Green dot**: Completed session with AI summary (existing)
- **Yellow dot**: Completed session without AI summary (existing)

### Active Session Card (No Journal Entry Yet)

Active sessions that haven't been journaled yet (no SessionEnd) show as a special card at the top of the entry list:

- Project name and branch from `active-sessions.json`
- "Active — started HH:MM" instead of a time range
- No summary (session is still in progress)
- Resume button still works (copies `cd CWD && claude --resume SESSION_ID`)

### File Watching

The app already watches `~/claude-journal/` for journal file changes. Additionally watch `~/.config/cc-journal/active-sessions.json` for active session changes. Both use the same FSEvents pattern.

### Menu Bar Icon Animation

When at least one session is active:
- Subtle pulse animation on the menu bar icon
- Or swap to a different SF Symbol (e.g., `book.fill` → `book.and.wrench.fill`)

## Implementation Phases

### Phase 1: State file + hooks
- Add `--event session-start` and `--event session-end` to `cc-journal hook`
- File locking for concurrent writes (multiple Claude Code instances)
- Stale session cleanup

### Phase 2: Menu bar integration
- Read `active-sessions.json` in the app
- Show active indicators in popover
- Watch the state file for changes

### Phase 3: Polish
- Menu bar icon animation when sessions are active
- "X active sessions" count in popover header
- Notification when a session ends and gets journaled

## File Locking

Multiple Claude Code instances may fire hooks concurrently. Use `flock(2)` (or Go's `os.OpenFile` with `O_EXCL`) on a `.lock` file next to `active-sessions.json` to serialize writes.

## Open Questions

1. **Should active sessions appear in `cc-journal today --json`?** Probably not — they're not journaled yet. A separate `cc-journal active --json` command might be cleaner.
2. **Multi-machine?** If the config dir is synced, active sessions from other machines would show up with dead PIDs. The stale cleanup handles this, but we could also include a hostname field.
