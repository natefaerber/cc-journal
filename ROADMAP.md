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

- [ ] **Search** — Full-text search across all journal entries
- [ ] **Filtering & facets** — Filter dashboard by project, date range, tags
- [ ] **Tags/labels** — Tag sessions (e.g., bugfix, feature, refactor) for better categorization
- [ ] **Session timeline view** — Visual timeline of sessions within a day
- [ ] **Trends & analytics** — Coding velocity charts, project distribution over time, streak tracking
- [ ] **Theme support** — Switchable themes (light/dark/system) with CSS custom properties; allow users to define custom themes via config or template overrides
- [ ] **Session info panel** — Slide-over panel on session click (like iOS Photos EXIF) showing full token breakdown (input, output, cache create, cache read, summary), cost estimate, duration, links, and session metadata without relying on hover tooltips
- [ ] **Mobile-friendly layout** — Responsive design improvements for phone/tablet

## Journal Quality

- [x] **Track all branches per session** — Scan every JSONL entry's `gitBranch` + `cwd` fields (not just the first) to capture all `(project, branch)` pairs touched during a session. Show multiple branches in entry headers when applicable. Fixes `n/a` branches for sessions started outside git repos that later `cd` into one.
- [ ] **Smarter summarization prompts** — Improve default prompts for more actionable, concise summaries
- [ ] **Multi-model support** — Allow different models for different tasks (e.g., fast model for backfill, best model for daily summaries)
- [ ] **Summary regeneration** — Re-summarize a session with updated prompts without losing metadata
- [ ] **Session linking** — Detect when sessions are continuations of previous work and link them

## Developer Experience

- [x] **Mise build tasks** — `mise run build`, `check`, `dev`, `dev:revert`, `serve`, `serve:stop`, `clean`
- [x] **Usage CLI spec** — Shell completions (bash/zsh/fish) and markdown docs generated from `cc-journal.usage.kdl`
- [ ] **`cc-journal watch`** — Live-reload dashboard during development
- [ ] **Plugin system** — Hooks for custom post-processing (e.g., auto-post to blog, sync to Notion)
- [ ] **Export formats** — Export journal to CSV, JSON, or static site (beyond current static HTML)
- [ ] **Backup & sync** — Optional git auto-commit for journal directory
- [ ] **Changelog** — use git-cliff for detailed changelog creation

## Distribution & Setup

- [ ] **Homebrew tap** — `brew install natefaerber/tap/cc-journal`
- [ ] **One-line hook setup** — `cc-journal init --hook` to auto-configure the Claude Code SessionEnd hook
- [ ] **Config validation** — `cc-journal doctor` to verify config, API key, hook setup, and journal directory

---

## Priority

**Near-term focus:**

1. Token tracking
2. MCP server or REST API
3. Search & filtering

**Medium-term:**
4. Summary regeneration
5. Homebrew tap + easier setup
6. Theme support

**Long-term:**
7. Trends & analytics
8. Plugin system
9. Multi-model support
