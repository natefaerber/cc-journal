package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	apiURL             = "https://api.anthropic.com/v1/messages"
	apiVersion         = "2023-06-01"
	maxTranscriptChars = 80_000
)

// TokenUsage tracks token consumption for a session and its summarization.
type TokenUsage struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	SummaryInputTokens       int64
	SummaryOutputTokens      int64
}

// SessionTokens returns total tokens from the Claude Code session (excluding summarizer).
func (t TokenUsage) SessionTokens() int64 {
	return t.InputTokens + t.OutputTokens + t.CacheCreationInputTokens + t.CacheReadInputTokens
}

// TotalTokens returns all tokens including summarizer.
func (t TokenUsage) TotalTokens() int64 {
	return t.SessionTokens() + t.SummaryInputTokens + t.SummaryOutputTokens
}

// transcriptMessage is a single user/assistant message extracted from a JSONL transcript.
type transcriptMessage struct {
	Role string
	Text string
	Time string
}

// BranchInfo represents a unique cwd+branch pair seen during a session.
type BranchInfo struct {
	CWD    string
	Branch string
}

// sessionMeta holds metadata extracted from a session transcript.
type sessionMeta struct {
	SessionID string
	CWD       string
	GitBranch string
	Project   string
	Branches  []BranchInfo
	Messages  []transcriptMessage
	FirstTime string
	LastTime  string
	Links     []ExternalLink
	Tokens    TokenUsage
}

// BranchDisplay returns a comma-separated list of unique branch names, or "n/a" if empty.
func (m *sessionMeta) BranchDisplay() string {
	if len(m.Branches) == 0 {
		if m.GitBranch != "" {
			return m.GitBranch
		}
		return "n/a"
	}
	seen := make(map[string]bool)
	var names []string
	for _, b := range m.Branches {
		if b.Branch != "" && !seen[b.Branch] {
			seen[b.Branch] = true
			names = append(names, b.Branch)
		}
	}
	if len(names) == 0 {
		return "n/a"
	}
	return strings.Join(names, ", ")
}

// getAPIKey retrieves the Anthropic API key.
// Priority: fnox > config/env > error.
func getAPIKey() (string, error) {
	// Try fnox first (most reliable, always fresh)
	if fnoxBin, err := exec.LookPath("fnox"); err == nil {
		out, err := exec.Command(fnoxBin, "get", "ANTHROPIC_API_KEY").Output()
		if err == nil && len(bytes.TrimSpace(out)) > 0 {
			return string(bytes.TrimSpace(out)), nil
		}
	}
	// Fall back to config file or ANTHROPIC_API_KEY env var
	if cfg.APIKey != "" {
		return cfg.APIKey, nil
	}
	return "", fmt.Errorf("no API key found (tried fnox, config, and ANTHROPIC_API_KEY env)")
}

// findSessionTranscript locates the JSONL transcript file for a session ID.
// If sessionID is empty, finds the most recent session for the given cwd.
func findSessionTranscript(sessionID string, cwd string) (string, string, error) {
	home, _ := os.UserHomeDir()
	projectsDir := filepath.Join(home, ".claude", "projects")

	if sessionID != "" {
		// Search all project dirs for this session ID
		matches, _ := filepath.Glob(filepath.Join(projectsDir, "*", sessionID+".jsonl"))
		if len(matches) > 0 {
			return matches[0], sessionID, nil
		}
		return "", "", fmt.Errorf("session %s not found", sessionID)
	}

	// Find most recent session for cwd
	// The project dir name is derived from the cwd path
	var candidates []struct {
		path    string
		modTime time.Time
		id      string
	}

	dirs, _ := os.ReadDir(projectsDir)
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dirPath := filepath.Join(projectsDir, d.Name())
		files, _ := filepath.Glob(filepath.Join(dirPath, "*.jsonl"))
		for _, f := range files {
			if strings.Contains(f, "subagent") {
				continue
			}
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			// If cwd is specified, check if this session's cwd matches
			if cwd != "" {
				fileCwd := peekCwd(f)
				if fileCwd != "" && fileCwd != cwd {
					continue
				}
			}
			sid := strings.TrimSuffix(filepath.Base(f), ".jsonl")
			candidates = append(candidates, struct {
				path    string
				modTime time.Time
				id      string
			}{f, info.ModTime(), sid})
		}
	}

	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no session transcripts found")
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	return candidates[0].path, candidates[0].id, nil
}

// peekCwd reads the first few lines of a JSONL to extract the cwd.
func peekCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for i := 0; i < 20 && scanner.Scan(); i++ {
		var entry map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if cwd, ok := entry["cwd"].(string); ok && cwd != "" {
			return cwd
		}
	}
	return ""
}

// parseTranscript reads a JSONL transcript file and extracts messages and metadata.
func parseTranscript(path string) (*sessionMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	meta := &sessionMeta{
		SessionID: strings.TrimSuffix(filepath.Base(path), ".jsonl"),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	seenBranches := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		// Track all unique cwd+branch pairs
		entryCwd, _ := entry["cwd"].(string)
		entryBranch, _ := entry["gitBranch"].(string)
		if entryBranch != "" {
			key := entryCwd + "|" + entryBranch
			if !seenBranches[key] {
				seenBranches[key] = true
				meta.Branches = append(meta.Branches, BranchInfo{CWD: entryCwd, Branch: entryBranch})
			}
		}

		// Still set first-seen CWD and GitBranch for backward compat
		if meta.GitBranch == "" && entryBranch != "" {
			meta.GitBranch = entryBranch
		}
		if meta.CWD == "" && entryCwd != "" {
			meta.CWD = entryCwd
		}
		if ts, ok := entry["timestamp"].(string); ok && ts != "" {
			if meta.FirstTime == "" {
				meta.FirstTime = ts
			}
			meta.LastTime = ts
		}

		entryType, _ := entry["type"].(string)

		// Accumulate token usage from assistant messages
		if entryType == "assistant" {
			if msg, ok := entry["message"].(map[string]interface{}); ok {
				if usage, ok := msg["usage"].(map[string]interface{}); ok {
					if v, ok := usage["input_tokens"].(float64); ok {
						meta.Tokens.InputTokens += int64(v)
					}
					if v, ok := usage["output_tokens"].(float64); ok {
						meta.Tokens.OutputTokens += int64(v)
					}
					if v, ok := usage["cache_creation_input_tokens"].(float64); ok {
						meta.Tokens.CacheCreationInputTokens += int64(v)
					}
					if v, ok := usage["cache_read_input_tokens"].(float64); ok {
						meta.Tokens.CacheReadInputTokens += int64(v)
					}
				}
			}
		}

		if entryType != "user" && entryType != "assistant" {
			continue
		}
		// Skip tool results
		if entryType == "user" {
			if _, ok := entry["toolUseResult"]; ok {
				continue
			}
		}

		msg, _ := entry["message"].(map[string]interface{})
		if msg == nil {
			continue
		}

		text := extractText(msg)
		if text == "" {
			continue
		}

		meta.Messages = append(meta.Messages, transcriptMessage{
			Role: entryType,
			Text: text,
			Time: fmt.Sprint(entry["timestamp"]),
		})
	}

	if meta.CWD != "" {
		meta.Project = filepath.Base(meta.CWD)
	}

	return meta, nil
}

// extractText pulls text content from a message, summarizing tool use.
func extractText(msg map[string]interface{}) string {
	content := msg["content"]
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, block := range c {
			bm, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			blockType, _ := bm["type"].(string)
			switch blockType {
			case "text":
				if t, ok := bm["text"].(string); ok {
					parts = append(parts, t)
				}
			case "tool_use":
				name, _ := bm["name"].(string)
				input, _ := bm["input"].(map[string]interface{})
				summary := fmt.Sprintf("[Tool: %s]", name)
				if name == "Bash" || name == "bash" {
					if cmd, ok := input["command"].(string); ok {
						if len(cmd) > 120 {
							cmd = cmd[:120]
						}
						summary = fmt.Sprintf("[Bash: %s]", cmd)
					}
				} else if name == "Read" || name == "read" {
					if fp, ok := input["file_path"].(string); ok {
						summary = fmt.Sprintf("[Read: %s]", fp)
					}
				} else if name == "Edit" || name == "edit" || name == "Write" || name == "write" {
					if fp, ok := input["file_path"].(string); ok {
						summary = fmt.Sprintf("[%s: %s]", name, fp)
					}
				} else if strings.HasPrefix(name, "mcp__") {
					// Summarize MCP tool calls
					summary = fmt.Sprintf("[MCP: %s]", name)
				}
				parts = append(parts, summary)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// buildTranscriptText creates a condensed transcript for the API prompt.
func buildTranscriptText(messages []transcriptMessage) string {
	var lines []string
	for _, m := range messages {
		prefix := "USER"
		if m.Role == "assistant" {
			prefix = "CLAUDE"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", prefix, m.Text))
	}
	full := strings.Join(lines, "\n\n")
	if len(full) > maxTranscriptChars {
		half := maxTranscriptChars / 2
		full = full[:half] + "\n\n[... middle of conversation trimmed ...]\n\n" + full[len(full)-half:]
	}
	return full
}

const defaultPromptTemplate = `You are summarizing a Claude Code session for a developer's journal.

Project: {{.Project}}
Branch: {{.Branch}}

Produce a markdown summary using EXACTLY this template. Do not add a title or any text before the first heading. Use ### for section headings. Keep it concise (under 300 words total). Use past tense. No greetings or filler.

If the session was very short or trivial (e.g. just a question or a single small change), collapse everything into just the "### Done" section with 1-2 bullet points and omit the other sections.

### Done
- Concrete outcome 1 (feature built, bug fixed, file modified)
- Concrete outcome 2
- ...

### Decisions
- Notable technical choice or trade-off (omit this section if none)

### Open
- Anything left unfinished or flagged for follow-up (omit this section if none)

<transcript>
{{.Transcript}}
</transcript>`

const defaultRollupTemplate = `You are creating a weekly development summary from a developer's daily Claude Code journal entries.

Week of: {{.Week}}

Below are the daily entries. Produce a weekly rollup in markdown with:

1. **Highlights** — the 3-5 most significant things accomplished this week
2. **Projects touched** — group work by project/repo
3. **Patterns** — any recurring themes
4. **Carry-forward** — open threads or items to pick up next week

Keep it concise and actionable.

<daily_entries>
{{.Content}}
</daily_entries>`

// loadPrompt loads a named prompt from the prompt directory.
// Falls back to the embedded default for "summary". Other names return empty on miss.
func loadPrompt(name string) string {
	path := filepath.Join(cfg.PromptDir, name+".txt")
	data, err := os.ReadFile(path)
	if err == nil && len(bytes.TrimSpace(data)) > 0 {
		return string(data)
	}
	// Built-in defaults
	switch name {
	case "summary":
		return defaultPromptTemplate
	case "rollup":
		return defaultRollupTemplate
	case "standup":
		return defaultStandupTemplate
	case "weekly":
		return defaultWeeklyTemplate
	}
	return ""
}

// apiResponse is the parsed Anthropic API response.
type apiResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage"`
}

// callAnthropicAPI sends the transcript to the API for summarization.
func callAnthropicAPI(apiKey, transcript, project, branch string) (string, TokenUsage, error) {
	tmplStr := loadPrompt("summary")
	prompt := strings.NewReplacer(
		"{{.Project}}", project,
		"{{.Branch}}", branch,
		"{{.Transcript}}", transcript,
	).Replace(tmplStr)

	body := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": 1024,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBody))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", TokenUsage{}, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", TokenUsage{}, fmt.Errorf("failed to parse API response: %w", err)
	}
	if len(result.Content) == 0 {
		return "", TokenUsage{}, fmt.Errorf("empty API response")
	}
	tokens := TokenUsage{
		SummaryInputTokens:  result.Usage.InputTokens,
		SummaryOutputTokens: result.Usage.OutputTokens,
	}
	return result.Content[0].Text, tokens, nil
}

// callAnthropicAPIRaw sends a raw prompt to the API with a custom max_tokens.
func callAnthropicAPIRaw(apiKey, prompt string, maxTokens int) (string, error) {
	body := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": maxTokens,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBody))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse API response: %w", err)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty API response")
	}
	return result.Content[0].Text, nil
}

// appendToJournal writes a session summary to the journal file.
func appendToJournal(meta *sessionMeta, summary string) error {
	dir := journalDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating journal dir: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	journalFile := filepath.Join(dir, today+".md")

	// Parse time range
	timeRange := "unknown"
	if meta.FirstTime != "" && meta.LastTime != "" {
		if st, err := time.Parse(time.RFC3339Nano, meta.FirstTime); err == nil {
			if et, err := time.Parse(time.RFC3339Nano, meta.LastTime); err == nil {
				timeRange = fmt.Sprintf("%s–%s", st.Local().Format("15:04"), et.Local().Format("15:04"))
			}
		}
	}

	// Create file with header if new
	if _, err := os.Stat(journalFile); os.IsNotExist(err) {
		if err := os.WriteFile(journalFile, []byte(fmt.Sprintf("# Claude Code Journal — %s\n\n", today)), 0o644); err != nil {
			return fmt.Errorf("creating journal file: %w", err)
		}
	}

	branch := meta.BranchDisplay()

	cwdLine := ""
	if meta.CWD != "" {
		cwdLine = fmt.Sprintf("\n<code>%s</code>", meta.CWD)
	}

	tokensLine := ""
	if meta.Tokens.SessionTokens() > 0 || meta.Tokens.SummaryInputTokens > 0 {
		tokensLine = fmt.Sprintf("\n<code>tokens:in=%d,out=%d,cache_create=%d,cache_read=%d,summary_in=%d,summary_out=%d</code>",
			meta.Tokens.InputTokens, meta.Tokens.OutputTokens,
			meta.Tokens.CacheCreationInputTokens, meta.Tokens.CacheReadInputTokens,
			meta.Tokens.SummaryInputTokens, meta.Tokens.SummaryOutputTokens)
	}

	linksBlock := formatLinksForJournal(meta.Links)

	entry := fmt.Sprintf(`---

## %s (%s) — %s

%s

%s<details>
<summary>Session ID</summary>
<code>%s</code>%s%s
</details>

`, meta.Project, branch, timeRange, summary, linksBlock, meta.SessionID, cwdLine, tokensLine)

	f, err := os.OpenFile(journalFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(entry)
	return err
}

// isSessionJournaled checks if a session ID already has a successful summary.
func isSessionJournaled(sessionID string) bool {
	dir := journalDir()
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	idPattern := regexp.MustCompile(`<code>` + regexp.QuoteMeta(sessionID) + `</code>`)
	failPattern := regexp.MustCompile(`Summary generation failed`)

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		matches := idPattern.FindAllIndex(content, -1)
		for _, m := range matches {
			start := m[0] - 500
			if start < 0 {
				start = 0
			}
			section := content[start:m[0]]
			if !failPattern.Match(section) {
				return true
			}
		}
	}
	return false
}

// summarizeSession is the main entry point for the summarize command.
func summarizeSession(sessionID string, force bool) {
	cwd, _ := os.Getwd()

	fmt.Println("Finding session transcript...")
	path, sid, err := findSessionTranscript(sessionID, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Session: %s\n", sid)

	if isSessionJournaled(sid) {
		if !force {
			fmt.Println("Session already has a summary in the journal. Use --force to re-summarize.")
			return
		}
		fmt.Println("Replacing existing journal entry...")
		removeFromJournal(sid)
	}

	fmt.Println("Parsing transcript...")
	meta, err := parseTranscript(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing transcript: %v\n", err)
		os.Exit(1)
	}

	if len(meta.Messages) == 0 {
		fmt.Println("No messages found in transcript.")
		return
	}
	fmt.Printf("Project: %s (%s), %d messages\n", meta.Project, meta.BranchDisplay(), len(meta.Messages))

	fmt.Println("Getting API key...")
	apiKey, err := getAPIKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Extract external links
	meta.Links = extractLinksFromTranscript(meta.Messages)
	for _, m := range meta.Messages {
		if issueLinks := extractIssueKeysFromText(m.Text); len(issueLinks) > 0 {
			meta.Links = deduplicateLinks(meta.Links, issueLinks)
		}
	}
	if len(meta.Links) > 0 {
		fmt.Printf("Found %d external links\n", len(meta.Links))
	}

	fmt.Println("Generating summary...")
	transcript := buildTranscriptText(meta.Messages)
	summary, summaryTokens, err := callAnthropicAPI(apiKey, transcript, meta.Project, meta.BranchDisplay())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	meta.Tokens.SummaryInputTokens = summaryTokens.SummaryInputTokens
	meta.Tokens.SummaryOutputTokens = summaryTokens.SummaryOutputTokens

	if err := appendToJournal(meta, summary); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing journal: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Summary written to ~/claude-journal/%s.md\n", time.Now().Format("2006-01-02"))
	fmt.Println()
	fmt.Println(summary)
}
