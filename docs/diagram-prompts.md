# Diagram Generation Prompts

Image generation prompts for cc-journal architecture diagrams. Feed these to an AI image generation tool (Ideogram, Midjourney, DALL-E, etc.) to produce polished marketing-quality visuals.

All diagrams use a light theme to render well on GitHub markdown (white/light gray backgrounds).

---

## 1. Data Flow Diagram

**Create a polished technical data flow diagram for "cc-journal" — a developer tool that processes AI coding session transcripts into structured journal entries.**

**Visual style:** Clean, modern SaaS documentation aesthetic. Light background (white or very light gray). Rounded-corner cards with soft shadows for each stage. Subtle pastel fills to distinguish stages — think the light, airy feel of Linear's or Vercel's documentation diagrams. Thin directional arrows between stages. Monospace font for code references, clean sans-serif for labels. Generous whitespace.

**Color guidance:** Use soft, distinct pastel hues for each stage of the pipeline — light blues, greens, ambers, lavenders. The palette should feel cohesive and professional, readable on a white GitHub README page. Each stage should be visually distinct but harmonious.

**Layout (left to right, single pipeline):**

**Stage 1 — Source:**
Card: "JSONL Transcript" with subtitle "~/.claude/projects/*.jsonl"

**Stage 2 — Parse:**
Card: "parseTranscript()" — extracts messages, branches, metadata from raw JSONL

**Stage 3 — Intermediate:**
Card: "sessionMeta" with fields listed: Messages, Branches, CWD, Project, Times, Links

**Stage 4 — Summarize:**
Card: "Anthropic API" with subtitle "callAnthropicAPI() + loadPrompt('summary')"

**Stage 5 — Store:**
Card: "Markdown Journal Files" with subtitle "~/claude-journal/YYYY-MM-DD.md"

**Stage 6 — Read:**
Card: "parseJournalFiles()" — regex-based entry extraction from markdown

**Stage 7 — Aggregate:**
Card: "JournalData" with subtitle "{Entries, Projects}"

**Stage 8 — Branch into two outputs:**
- Upper branch: "buildDashboard()" → "DashboardData" (Stats, Heatmap, Bars, Recent) → "Web UI"
- Lower branch: "formatDaily() / formatWeekly()" → "StandupData / WeeklyData" (Groups, OpenItems, Decisions, Links) → "CLI / Slack / Clipboard"

**Additional notes:**
- This should read as a clear left-to-right pipeline that branches at the end
- Each card should feel like a discrete processing stage
- The branch point at JournalData should feel like a fork, not a copy
- Must look good on a white background GitHub README
- Aspect ratio: 16:9
- High quality, crisp edges, legible text

---

## 2. Key Types / Class Diagram

**Create a polished technical type relationship diagram for "cc-journal" showing the core data structures and how they transform into each other.**

**Visual style:** Clean, light-themed developer documentation aesthetic. White or very light gray background. Each type is a card with a soft shadow, showing the type name prominently and its key fields in a smaller font beneath. Connection lines show transformation relationships with labels. Think of a UML class diagram but with modern styling — no UML notation, just clean cards and labeled arrows. Should look great embedded in a GitHub markdown page.

**Color guidance:** Group related types by soft pastel hue. Source/input types in one tone (e.g., light blue), storage types in another (e.g., light green), output types in a third (e.g., light amber or lavender). Keep it cohesive and readable on white.

**Types to show (each as a card with fields):**

**sessionMeta** (input):
- SessionID, CWD, GitBranch, Project
- Branches []BranchInfo
- Messages []transcriptMessage
- FirstTime, LastTime
- Links []ExternalLink
- Method: BranchDisplay()

**Entry** (storage):
- Date, Project, Branch, TimeRange
- SessionID, Cwd, Summary
- HasAISummary bool
- Links []ExternalLink
- Method: SummaryPreview()

**JournalData** (aggregate):
- Entries []Entry
- DailyFiles []string
- Projects []ProjectCount

**DashboardData** (web output):
- Stats, Projects []ProjectCount
- Bars []Bar, Heatmap []HeatmapDay
- Recent []Entry

**StandupData** (report output):
- DateLabel, YesterdayDate
- YesterdayGroups, TodayGroups []ReportGroup
- OpenItems []string, Links []ExternalLink

**WeeklyData** (report output):
- WeekLabel, Groups []ReportGroup
- Decisions, OpenItems []string
- Links []ExternalLink
- TotalSessions, TotalProjects, ActiveDays int

**ReportGroup** (shared):
- Project, Branches, Duration
- Sessions int, Bullets []string

**ExternalLink** (shared):
- Service, Label, URL

**Config** (configuration):
- JournalDir, PromptDir, Model, APIKey
- Exclude []string
- Slack SlackConfig, Links LinksConfig

**Relationships (shown as labeled arrows):**
- sessionMeta → Entry (labeled "appendToJournal")
- Entry →* JournalData (composition)
- JournalData → DashboardData (labeled "buildDashboard")
- JournalData → StandupData (labeled "formatDaily")
- JournalData → WeeklyData (labeled "formatWeekly")
- ReportGroup →* StandupData (composition)
- ReportGroup →* WeeklyData (composition)
- ExternalLink →* Entry, StandupData, WeeklyData (composition)

**Additional notes:**
- This is NOT a UML diagram — it should look like a modern tech blog illustration
- The flow should generally read top-to-bottom: sessionMeta → Entry → JournalData → outputs
- Config should be off to the side, connected to nothing (it's ambient context)
- Must render well on a white GitHub README background
- Aspect ratio: 16:9
- High quality, legible field names

---

## 3. External Integrations Diagram

**Create a polished technical integration diagram for "cc-journal" showing how it connects to external services.**

**Visual style:** Clean, modern, light-themed. White or very light gray background. cc-journal as a central hub with external services radiating outward. Each service gets a small card or badge with an icon and label. Connection lines should indicate direction and mechanism. Soft shadows, pastel accents. Should look like it belongs in polished developer documentation on GitHub.

**Central element:**
Large card: "cc-journal" with subtitle "Go CLI binary"

**External services (radiating outward):**

**Inbound (arrows pointing into cc-journal):**
- "Claude Code" — reads ~/.claude/projects/ JSONL transcripts
- "Anthropic API" — HTTP POST to /v1/messages for summarization and rollup

**Outbound (arrows pointing out from cc-journal):**
- "Slack" — report delivery via configurable command exec
- "Clipboard" — pbcopy (macOS) / xclip / xsel (Linux)

**Bidirectional / Detection:**
- "GitHub" — URL pattern matching from transcripts (PRs, issues)
- "Linear" — issue key extraction from transcripts
- "Jira" — issue key extraction + configurable base URLs
- "Confluence" — page URL extraction from transcripts

**Additional notes:**
- The GitHub/Linear/Jira/Confluence connections are passive (pattern matching, not API calls) — style them differently from active integrations (Anthropic API, Slack), perhaps with dashed lines or a muted color
- Show the mechanism on each connection line (e.g., "HTTP POST", "exec(command)", "JSONL read", "URL pattern match")
- Must look good on a white GitHub README background
- Aspect ratio: 16:9
- High quality
