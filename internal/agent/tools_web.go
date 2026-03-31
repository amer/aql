package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	// httpTimeout is the timeout for outbound HTTP requests.
	httpTimeout = 30 * time.Second

	// maxResponseBodyBytes is the max bytes read from HTTP response bodies.
	maxResponseBodyBytes = 512 * 1024 // 512 KB

	// maxExtractedTextLen is the max character length for extracted HTML text.
	maxExtractedTextLen = 50_000

	// maxSearchResults is the max number of search results to return.
	maxSearchResults = 10
)

var httpClient = &http.Client{
	Timeout: httpTimeout,
}

func execWebFetch(ctx context.Context, input json.RawMessage) (string, error) {
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
	resp, err := httpClient.Do(req)
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

func extractText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript") {
			return
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "br", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr":
				sb.WriteString("\n")
			}
		}
	}
	walk(doc)
	result := sb.String()
	if len(result) > maxExtractedTextLen {
		result = result[:maxExtractedTextLen] + "\n... (truncated)"
	}
	return strings.TrimSpace(result)
}

func execWebSearch(ctx context.Context, input json.RawMessage) (string, error) {
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
	resp, err := httpClient.Do(req)
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

func parseSearchResults(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "failed to parse search results"
	}
	type result struct {
		title, url, snippet string
	}
	var results []result
	var findResults func(*html.Node)
	findResults = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "result__a") {
			r := result{title: textContent(n)}
			for _, a := range n.Attr {
				if a.Key == "href" {
					r.url = a.Val
				}
			}
			if p := n.Parent; p != nil {
				for c := p.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && hasClass(c, "result__snippet") {
						r.snippet = textContent(c)
					}
				}
				if pp := p.Parent; pp != nil {
					for c := pp.FirstChild; c != nil; c = c.NextSibling {
						if c.Type == html.ElementNode && hasClass(c, "result__snippet") {
							r.snippet = textContent(c)
						}
					}
				}
			}
			if r.title != "" {
				results = append(results, r)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findResults(c)
		}
	}
	findResults(doc)
	if len(results) == 0 {
		return "No results found."
	}
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
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
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
