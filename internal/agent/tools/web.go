package tools

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - execWebFetch — URL fetching with HTML text extraction
//   - execWebSearch — DuckDuckGo search
//   - HTML parsing helpers (extractText, walkText, parseSearchResults,
//     collectSearchResults)
//   - Response size and search result constants
//
// MUST NOT GO HERE:
//   - Tool definitions (defs.go)
//   - File I/O
//   - Anything that imports the agent package
//
// Q: Should I add a new search provider?
// A: Replace or extend execWebSearch here. The tool definition in
//    defs.go stays the same.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

const (
	// maxResponseBodyBytes is the max bytes read from HTTP response bodies.
	maxResponseBodyBytes = 512 * 1024 // 512 KB

	// maxExtractedTextLen is the max character length for extracted HTML text.
	maxExtractedTextLen = 50_000

	// maxSearchResults is the max number of search results to return.
	maxSearchResults = 10
)

func execWebFetch(ctx context.Context, client *http.Client, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		URL string `json:"url"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.URL, nil)
	if err != nil {
		return "invalid URL: " + err.Error(), nil
	}
	req.Header.Set("User-Agent", "AQL-Agent/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "fetch error: " + err.Error(), nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return "read error: " + err.Error(), nil
	}
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode >= 400 {
		return fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)), nil
	}
	if strings.Contains(contentType, "text/html") {
		return extractText(string(body)), nil
	}
	return string(body), nil
}

// skipElements are HTML elements whose content is not useful as text.
var skipElements = map[string]bool{"script": true, "style": true, "noscript": true}

// blockElements are HTML elements that produce line breaks in extracted text.
var blockElements = map[string]bool{
	"p": true, "br": true, "div": true, "li": true, "tr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

func extractText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	var sb strings.Builder
	walkText(doc, &sb)
	result := sb.String()
	if len(result) > maxExtractedTextLen {
		result = result[:maxExtractedTextLen] + "\n... (truncated)"
	}
	return strings.TrimSpace(result)
}

// walkText recursively extracts visible text from an HTML node tree.
func walkText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.ElementNode && skipElements[n.Data] {
		return
	}
	if n.Type == html.TextNode {
		if text := strings.TrimSpace(n.Data); text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkText(c, sb)
	}
	if n.Type == html.ElementNode && blockElements[n.Data] {
		sb.WriteString("\n")
	}
}

func execWebSearch(ctx context.Context, client *http.Client, input json.RawMessage) (string, error) {
	params, errMsg := parseInput[struct {
		Query string `json:"query"`
	}](input)
	if errMsg != "" {
		return errMsg, nil
	}
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(params.Query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "search error: " + err.Error(), nil
	}
	req.Header.Set("User-Agent", "AQL-Agent/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "search error: " + err.Error(), nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return "read error: " + err.Error(), nil
	}
	return parseSearchResults(string(body)), nil
}

type searchResult struct {
	title, url, snippet string
}

func parseSearchResults(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "failed to parse search results"
	}

	results := collectSearchResults(doc)
	if len(results) == 0 {
		// A non-empty body that yields zero results usually means the
		// upstream HTML changed shape (class names, layout) and our
		// selectors are stale — surface it rather than looking like a
		// genuine empty result set.
		if strings.TrimSpace(htmlContent) != "" {
			slog.Warn("web_search parsed zero results from non-empty response",
				"body_bytes", len(htmlContent))
		}
		return "No results found."
	}
	return formatSearchResults(results)
}

// collectSearchResults walks the HTML tree and extracts DuckDuckGo search results.
func collectSearchResults(n *html.Node) []searchResult {
	var results []searchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "result__a") {
			if r := extractSearchResult(n); r.title != "" {
				results = append(results, r)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return results
}

// extractSearchResult extracts title, URL, and snippet from a result link node.
func extractSearchResult(link *html.Node) searchResult {
	r := searchResult{title: textContent(link)}
	for _, a := range link.Attr {
		if a.Key == "href" {
			r.url = a.Val
		}
	}
	r.snippet = findSnippet(link.Parent)
	if r.snippet == "" && link.Parent != nil {
		r.snippet = findSnippet(link.Parent.Parent)
	}
	return r
}

// findSnippet looks for a result__snippet element among a node's children.
func findSnippet(parent *html.Node) string {
	if parent == nil {
		return ""
	}
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && hasClass(c, "result__snippet") {
			return textContent(c)
		}
	}
	return ""
}

func formatSearchResults(results []searchResult) string {
	var sb strings.Builder
	for i, r := range results {
		if i >= maxSearchResults {
			break
		}
		fmt.Fprintf(&sb, "%d. %s\n   %s\n", i+1, r.title, r.url)
		if r.snippet != "" {
			fmt.Fprintf(&sb, "   %s\n", r.snippet)
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			if slices.Contains(strings.Fields(a.Val), class) {
				return true
			}
		}
	}
	return false
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}
