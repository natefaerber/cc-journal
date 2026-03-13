package main

import (
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"
)

// ExternalLink represents a link to an external service.
type ExternalLink struct {
	Service string // "github", "linear", "jira", "confluence"
	Label   string // Display text, e.g. "PROJ-1292", "PR #4013"
	URL     string // Full URL
}

var (
	// Matches URLs in text
	urlRe = regexp.MustCompile(`https?://[^\s<>")\]]+`)

	// Matches GitHub PR/issue URLs
	githubPRRe    = regexp.MustCompile(`https://github\.com/([^/]+/[^/]+)/pull/(\d+)`)
	githubIssueRe = regexp.MustCompile(`https://github\.com/([^/]+/[^/]+)/issues/(\d+)`)

	// Matches Linear URLs
	linearRe = regexp.MustCompile(`https://linear\.app/[^/]+/issue/([A-Z]+-\d+)`)

	// Matches Jira URLs
	jiraRe = regexp.MustCompile(`https://[^/]+\.atlassian\.net/browse/([A-Z]+-\d+)`)

	// Matches Confluence URLs
	confluenceRe = regexp.MustCompile(`https://[^/]+\.atlassian\.net/wiki/[^\s<>")\]]+`)
)

// extractLinksFromTranscript pulls external service URLs from transcript messages.
func extractLinksFromTranscript(messages []transcriptMessage) []ExternalLink {
	seen := make(map[string]bool)
	var links []ExternalLink

	for _, m := range messages {
		urls := urlRe.FindAllString(m.Text, -1)
		for _, u := range urls {
			// Clean trailing punctuation
			u = strings.TrimRight(u, ".,;:!?)")
			if seen[u] {
				continue
			}
			seen[u] = true

			if link := classifyURL(u); link != nil {
				links = append(links, *link)
			}
		}
	}

	return links
}

// classifyURL identifies a URL as a known external service link.
func classifyURL(u string) *ExternalLink {
	if m := githubPRRe.FindStringSubmatch(u); m != nil {
		return &ExternalLink{Service: "github", Label: fmt.Sprintf("%s PR #%s", m[1], m[2]), URL: u}
	}
	if m := githubIssueRe.FindStringSubmatch(u); m != nil {
		return &ExternalLink{Service: "github", Label: fmt.Sprintf("%s #%s", m[1], m[2]), URL: u}
	}
	if m := linearRe.FindStringSubmatch(u); m != nil {
		return &ExternalLink{Service: "linear", Label: m[1], URL: u}
	}
	if m := jiraRe.FindStringSubmatch(u); m != nil {
		return &ExternalLink{Service: "jira", Label: m[1], URL: u}
	}
	if confluenceRe.MatchString(u) {
		// Extract a readable label from Confluence URL
		label := "Confluence page"
		parts := strings.Split(u, "/")
		for i, p := range parts {
			if p == "pages" && i+2 < len(parts) {
				label = strings.ReplaceAll(parts[i+2], "+", " ")
				break
			}
		}
		return &ExternalLink{Service: "confluence", Label: label, URL: u}
	}
	return nil
}

// extractIssueKeysFromText finds issue key patterns (e.g., PROJ-1292) in text
// and resolves them to links using the configured issue prefixes.
func extractIssueKeysFromText(text string) []ExternalLink {
	if len(cfg.Links.Issues) == 0 {
		return nil
	}

	// Build regex from configured prefixes
	prefixes := make([]string, 0, len(cfg.Links.Issues))
	for prefix := range cfg.Links.Issues {
		prefixes = append(prefixes, regexp.QuoteMeta(prefix))
	}
	sort.Strings(prefixes) // deterministic order
	pattern := regexp.MustCompile(`\b(` + strings.Join(prefixes, "|") + `)-(\d+)\b`)

	seen := make(map[string]bool)
	var links []ExternalLink

	for _, m := range pattern.FindAllStringSubmatch(text, -1) {
		key := m[0] // e.g. "PROJ-1292"
		prefix := m[1]
		if seen[key] {
			continue
		}
		seen[key] = true

		baseURL := cfg.Links.Issues[prefix]
		service := "jira" // default
		if strings.Contains(baseURL, "linear.app") {
			service = "linear"
		}
		links = append(links, ExternalLink{
			Service: service,
			Label:   key,
			URL:     baseURL + "/" + key,
		})
	}

	return links
}

// formatLinksForJournal produces a markdown details block for links.
func formatLinksForJournal(links []ExternalLink) string {
	if len(links) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<details>\n<summary>Links</summary>\n\n")
	for _, l := range links {
		fmt.Fprintf(&b, "- [%s](%s)\n", l.Label, l.URL)
	}
	b.WriteString("\n</details>\n")
	return b.String()
}

// autoLinkIssueKeys replaces issue key patterns in HTML with clickable links.
func autoLinkIssueKeys(html string) string {
	if len(cfg.Links.Issues) == 0 {
		return html
	}

	prefixes := make([]string, 0, len(cfg.Links.Issues))
	for prefix := range cfg.Links.Issues {
		prefixes = append(prefixes, regexp.QuoteMeta(prefix))
	}
	sort.Strings(prefixes)

	// Match issue keys that aren't already inside an <a> tag or href attribute
	pattern := regexp.MustCompile(`(?i)\b(` + strings.Join(prefixes, "|") + `)-(\d+)\b`)

	// Simple approach: only replace keys not already inside a link
	result := pattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := pattern.FindStringSubmatch(match)
		if parts == nil {
			return match
		}
		prefix := strings.ToUpper(parts[1])
		baseURL, ok := cfg.Links.Issues[prefix]
		if !ok {
			// Try original case
			baseURL, ok = cfg.Links.Issues[parts[1]]
			if !ok {
				return match
			}
		}
		key := prefix + "-" + parts[2]
		url := baseURL + "/" + key
		return fmt.Sprintf(`<a href="%s" target="_blank" class="text-secondary hover:text-accent no-underline">%s</a>`, url, key)
	})

	return result
}

// autoLinkPRs replaces "PR #N" patterns with GitHub links if a default repo is configured.
func autoLinkPRs(html string) string {
	if len(cfg.Links.GitHubRepos) == 0 {
		return html
	}
	baseRepo := cfg.Links.GitHubRepos[0]
	prPattern := regexp.MustCompile(`PR #(\d+)`)
	return prPattern.ReplaceAllStringFunc(html, func(match string) string {
		parts := prPattern.FindStringSubmatch(match)
		if parts == nil {
			return match
		}
		url := baseRepo + "/pull/" + parts[1]
		return fmt.Sprintf(`<a href="%s" target="_blank" class="text-secondary hover:text-accent no-underline">PR #%s</a>`, url, parts[1])
	})
}

// deduplicateLinks adds new links to existing, skipping duplicates by label.
func deduplicateLinks(existing, additional []ExternalLink) []ExternalLink {
	seen := make(map[string]bool)
	for _, l := range existing {
		seen[l.Label] = true
	}
	for _, l := range additional {
		if !seen[l.Label] {
			existing = append(existing, l)
			seen[l.Label] = true
		}
	}
	return existing
}

// autoLinkAll applies all auto-linking to rendered HTML.
func autoLinkAll(htmlStr template.HTML) template.HTML {
	s := string(htmlStr)
	s = autoLinkIssueKeys(s)
	s = autoLinkPRs(s)
	return template.HTML(s)
}
