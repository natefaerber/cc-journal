package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
}

// runHook is the SessionEnd hook entry point. Reads JSON from stdin, summarizes, appends to journal.
func runHook() {
	var input hookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		os.Exit(0)
	}
	if input.TranscriptPath == "" {
		os.Exit(0)
	}
	if _, err := os.Stat(input.TranscriptPath); os.IsNotExist(err) {
		os.Exit(0)
	}

	// Check exclude list
	if input.CWD != "" && isExcluded(input.CWD) {
		os.Exit(0)
	}

	// Skip if denied
	if isDenied(input.SessionID) {
		os.Exit(0)
	}

	meta, err := parseTranscript(input.TranscriptPath)
	if err != nil || len(meta.Messages) == 0 {
		os.Exit(0)
	}

	// Replace existing entry (e.g. mid-session /summarize snapshot)
	if isSessionJournaled(input.SessionID) {
		targetDate := time.Now().Format("2006-01-02")
		if meta.LastTime != "" {
			if t, err := time.Parse(time.RFC3339Nano, meta.LastTime); err == nil {
				targetDate = t.Local().Format("2006-01-02")
			}
		}
		replaceWithStub(input.SessionID, targetDate)
	}
	// Override with hook-provided values
	if input.CWD != "" {
		meta.CWD = input.CWD
		meta.Project = filepath.Base(input.CWD)
	}
	meta.SessionID = input.SessionID

	apiKey, err := getAPIKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Extract external links from transcript
	meta.Links = extractLinksFromTranscript(meta.Messages)
	// Also extract issue keys from message text
	for _, m := range meta.Messages {
		if issueLinks := extractIssueKeysFromText(m.Text); len(issueLinks) > 0 {
			meta.Links = deduplicateLinks(meta.Links, issueLinks)
		}
	}

	transcript := buildTranscriptText(meta.Messages)
	summary, summaryTokens, err := callAnthropicAPI(apiKey, transcript, meta.Project, meta.BranchDisplay())
	if err == nil {
		meta.Tokens.SummaryInputTokens = summaryTokens.SummaryInputTokens
		meta.Tokens.SummaryOutputTokens = summaryTokens.SummaryOutputTokens
	}
	if err != nil {
		// Fallback: list user prompts
		var prompts []string
		for _, m := range meta.Messages {
			if m.Role == "user" {
				text := m.Text
				if len(text) > 200 {
					text = text[:200]
				}
				prompts = append(prompts, "- "+text)
				if len(prompts) >= 20 {
					break
				}
			}
		}
		summary = "**Prompts:**\n" + strings.Join(prompts, "\n")
	}

	if err := appendToJournal(meta, summary); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR writing journal: %v\n", err)
		os.Exit(1)
	}
}

// isExcluded checks if a cwd matches any exclude prefix from config.
func isExcluded(cwd string) bool {
	for _, prefix := range cfg.Exclude {
		if strings.HasPrefix(cwd, prefix) {
			return true
		}
	}
	return false
}
