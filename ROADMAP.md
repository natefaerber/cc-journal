# cc-journal Roadmap

## Status Legend

- [ ] Not started
- [x] Done
- 🔄 In progress

---

## Reports & Summaries

- [x] **Better default standup reports** — Richer daily summaries: group by project, highlight blockers, show PR links, include time spent
- [x] **Better default weekly reports** — Executive-style weekly: accomplishments, decisions, open/carry-forward items, duration per project, links
- [x] **Rendered report view** — Standup/weekly pages default to rendered markdown with Raw/Rendered toggle and Copy button; URLs auto-linked
- [x] **Configurable week start** — `week_start: sunday` or `monday` in config; all week boundaries, reports, and dashboard respect the setting
- [x] **Customizable standup/weekly reports** — User-defined report templates (sections, grouping, tone) via `cc-journal init --prompts` and Go text/template syntax
- [ ] **Monthly rollups** — Auto-generated monthly summary with trends and highlights
- [ ] **Report format options** — Output reports as markdown, plain text, or HTML (currently markdown-only)

## API & Integrations

- [ ] **MCP server** — Expose journal data as an MCP server so Claude Code (or other agents) can query session history, search entries, generate reports on demand
- [ ] **REST API** — General-purpose API beyond the current `/api/palette` endpoint: query sessions, filter by project/date, CRUD entries
- [ ] **Slack improvements** — Richer Slack formatting (blocks API), thread replies, scheduled posting
- [ ] **GitHub integration** — Auto-link PRs merged during sessions, show PR status in dashboard
- [ ] **Linear/Jira enrichment** — Pull issue titles/status from Linear/Jira to enrich journal entries beyond just linking

## Usage Tracking

- [x] **Token tracking** — Track input/output/cache tokens per session from JSONL transcripts plus summarizer API usage; displayed in dashboard cards, session tables, daily view metadata, and standup/weekly reports

## Dashboard & UI

- [x] **Search** — Full-text search via CLI (`cc-journal search`), `/api/search` endpoint, and command palette with debounced API fallback
- [x] **Session info panel** — Right-side drawer on session click showing full token breakdown, estimated cost (Sonnet pricing), duration, links, and session metadata. Available on dashboard, project, and daily views with keyboard shortcut (`i`)
- [x] **Projects page** — Dedicated `/projects` page with all projects, session counts, last active dates, client-side filter box (`/` to focus), j/k/o navigation. Dashboard shows last 3 days only with "View all" link
- [x] **Keyboard shortcuts** — `?` help modal, g-chords for all pages (gi/gp/gd/gs/gw), p/n prev/next, j/k session/row navigation, o to open, click-to-focus sessions
- [x] **Command palette** — Cmd+K palette with pages, projects, dates, sessions, and search results
- [ ] **Filtering & facets** — Filter dashboard by project, date range, tags
- [ ] **Tags/labels** — Tag sessions (e.g., bugfix, feature, refactor) for better categorization
- [ ] **Session timeline view** — Visual timeline of sessions within a day
- [ ] **Trends & analytics** — Coding velocity charts, project distribution over time, streak tracking
- [ ] **Theme support** — Switchable themes (light/dark/system) with CSS custom properties; allow users to define custom themes via config or template overrides
- [ ] **Mobile-friendly layout** — Responsive design improvements for phone/tablet

## Journal Quality

- [x] **Track all branches per session** — Scan every JSONL entry's `gitBranch` + `cwd` fields (not just the first) to capture all `(project, branch)` pairs touched during a session
- [x] **Summary replacement** — `--force` replaces existing entries instead of creating duplicates. Cross-day replacements leave redirect stubs in the old day's journal. SessionEnd hook auto-replaces mid-session `/summarize` snapshots
- [x] **Multi-day session support** — Journal entries write to the session's last day. Time range headers show dates when sessions span multiple days (e.g., `Mar 11 09:15–Mar 13 14:30`)
- [ ] **Smarter summarization prompts** — Improve default prompts for more actionable, concise summaries
- [ ] **Multi-model support** — Allow different models for different tasks (e.g., fast model for backfill, best model for daily summaries)
- [ ] **Session linking** — Detect when sessions are continuations of previous work and link them

## CLI & Developer Experience

- [x] **urfave/cli v3** — Proper flag validation, auto-generated help, command categories, typo suggestions (`Suggest: true`), `--version` flag
- [x] **Usage CLI spec** — Shell completions (bash/zsh/fish) and markdown docs generated from `cc-journal.usage.kdl`. Must stay in sync with urfave definitions in `main.go`
- [x] **Mise build tasks** — `mise run build`, `check`, `dev`, `dev:revert`, `serve`, `serve:stop`, `clean`, `render`
- [x] **SIGHUP config reload** — `kill -HUP` reloads config.yaml while serve is running; templates reload from disk per-request automatically
- [x] **Backfill `--since`** — Duration expressions (`1d`, `2h`, `30m`) with midnight/hour alignment by default, `--rolling` for exact. Replaced `--days`
- [x] **`init --commands`** — Install Claude Code slash commands (e.g., `/summarize`) to `~/.claude/commands/`
- [x] **hk pre-commit hooks** — go-fmt, go-vet, golangci-lint via hk. CI uses `jdx/mise-action` + `hk run check --all` for identical local/CI checks
- [x] **Dependabot** — Weekly checks for Go module and GitHub Actions updates
- [ ] **`cc-journal watch`** — Live-reload dashboard during development
- [ ] **Plugin system** — Hooks for custom post-processing (e.g., auto-post to blog, sync to Notion)
- [ ] **Export formats** — Export journal to CSV, JSON, or static site (beyond current static HTML)
- [ ] **Backup & sync** — Optional git auto-commit for journal directory
- [ ] **Changelog** — Use git-cliff for detailed changelog creation

## Distribution & Setup

- [ ] **Homebrew tap** — `brew install natefaerber/tap/cc-journal`
- [ ] **One-line hook setup** — `cc-journal init --hook` to auto-configure the Claude Code SessionEnd hook
- [ ] **Config validation** — `cc-journal doctor` to verify config, API key, hook setup, and journal directory

---

## Priority

**Near-term focus:**

1. MCP server
2. Filtering & facets
3. Theme support (dark mode)

**Medium-term:**
4. Homebrew tap + easier setup (`init --hook`, `doctor`)
5. Trends & analytics
6. Tags/labels

**Long-term:**
7. Plugin system
8. Multi-model support
9. Monthly rollups
