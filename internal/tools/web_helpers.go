package tools

import (
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	defaultWebTimeout  = 20 * time.Second
	defaultWebMaxBytes = 512 * 1024
	defaultWebUA       = "claw-go-code-tools/0.1"
)

var (
	htmlScriptStyleRe = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	htmlTagRe         = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlTitleRe       = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

type webSearchHit struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func buildHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultWebTimeout
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
}

func normalizeFetchURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("relative URL without a base")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s", parsed.Scheme)
	}
	if parsed.Scheme == "http" && !isLocalHost(parsed.Hostname()) {
		parsed.Scheme = "https"
	}
	return parsed.String(), nil
}

func buildSearchURL(query string) (string, error) {
	base := os.Getenv("CLAW_WEB_SEARCH_BASE_URL")
	if strings.TrimSpace(base) == "" {
		base = "https://html.duckduckgo.com/html/"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("relative URL without a base")
	}
	values := parsed.Query()
	values.Set("q", query)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func readLimitedBody(resp *http.Response, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultWebMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func normalizeFetchedContent(body string, contentType string) string {
	if strings.Contains(strings.ToLower(contentType), "html") {
		return htmlToText(body)
	}
	return strings.TrimSpace(body)
}

func summarizeWebFetch(finalURL string, prompt string, content string, rawBody string, contentType string) string {
	lowerPrompt := strings.ToLower(prompt)
	compact := collapseWhitespace(content)

	var detail string
	switch {
	case strings.Contains(lowerPrompt, "title"):
		if title := extractTitle(content, rawBody, contentType); title != "" {
			detail = "Title: " + title
		} else {
			detail = previewText(compact, 600)
		}
	case strings.Contains(lowerPrompt, "summary"), strings.Contains(lowerPrompt, "summarize"):
		detail = previewText(compact, 900)
	default:
		detail = fmt.Sprintf("Prompt: %s\nContent preview:\n%s", prompt, previewText(compact, 900))
	}

	return fmt.Sprintf("Fetched %s\n%s", finalURL, detail)
}

func extractTitle(content string, rawBody string, contentType string) string {
	if strings.Contains(strings.ToLower(contentType), "html") {
		if match := htmlTitleRe.FindStringSubmatch(rawBody); len(match) == 2 {
			title := collapseWhitespace(html.UnescapeString(match[1]))
			if title != "" {
				return title
			}
		}
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func htmlToText(raw string) string {
	withoutScripts := htmlScriptStyleRe.ReplaceAllString(raw, " ")
	withoutTags := htmlTagRe.ReplaceAllString(withoutScripts, " ")
	return collapseWhitespace(html.UnescapeString(withoutTags))
}

func collapseWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func previewText(input string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = 900
	}
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= maxChars {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:maxChars])) + "..."
}

func extractSearchHits(htmlBody string) []webSearchHit {
	hits := make([]webSearchHit, 0)
	remaining := htmlBody
	for {
		anchorStart := strings.Index(remaining, "result__a")
		if anchorStart < 0 {
			break
		}
		afterClass := remaining[anchorStart:]
		hrefIdx := strings.Index(afterClass, "href=")
		if hrefIdx < 0 {
			remaining = afterClass[1:]
			continue
		}
		urlValue, rest, ok := extractQuotedValue(afterClass[hrefIdx+5:])
		if !ok {
			remaining = afterClass[1:]
			continue
		}
		closeTagIdx := strings.Index(rest, ">")
		if closeTagIdx < 0 {
			remaining = afterClass[1:]
			continue
		}
		afterTag := rest[closeTagIdx+1:]
		endAnchorIdx := strings.Index(strings.ToLower(afterTag), "</a>")
		if endAnchorIdx < 0 {
			remaining = afterTag
			continue
		}
		title := htmlToText(afterTag[:endAnchorIdx])
		if decoded, ok := decodeDuckDuckGoRedirect(urlValue); ok {
			hits = append(hits, webSearchHit{Title: strings.TrimSpace(title), URL: decoded})
		}
		remaining = afterTag[endAnchorIdx+4:]
	}
	return hits
}

func extractSearchHitsFromGenericLinks(htmlBody string) []webSearchHit {
	hits := make([]webSearchHit, 0)
	remaining := htmlBody
	for {
		anchorStart := strings.Index(strings.ToLower(remaining), "<a")
		if anchorStart < 0 {
			break
		}
		afterAnchor := remaining[anchorStart:]
		hrefIdx := strings.Index(afterAnchor, "href=")
		if hrefIdx < 0 {
			if len(afterAnchor) <= 2 {
				break
			}
			remaining = afterAnchor[2:]
			continue
		}
		urlValue, rest, ok := extractQuotedValue(afterAnchor[hrefIdx+5:])
		if !ok {
			if len(afterAnchor) <= 2 {
				break
			}
			remaining = afterAnchor[2:]
			continue
		}
		closeTagIdx := strings.Index(rest, ">")
		if closeTagIdx < 0 {
			if len(afterAnchor) <= 2 {
				break
			}
			remaining = afterAnchor[2:]
			continue
		}
		afterTag := rest[closeTagIdx+1:]
		endAnchorIdx := strings.Index(strings.ToLower(afterTag), "</a>")
		if endAnchorIdx < 0 {
			if len(afterAnchor) <= 2 {
				break
			}
			remaining = afterAnchor[2:]
			continue
		}
		title := strings.TrimSpace(htmlToText(afterTag[:endAnchorIdx]))
		if title != "" {
			decoded := urlValue
			if maybeDecoded, ok := decodeDuckDuckGoRedirect(urlValue); ok {
				decoded = maybeDecoded
			}
			if strings.HasPrefix(decoded, "http://") || strings.HasPrefix(decoded, "https://") {
				hits = append(hits, webSearchHit{Title: title, URL: decoded})
			}
		}
		remaining = afterTag[endAnchorIdx+4:]
	}
	return hits
}

func extractQuotedValue(input string) (string, string, bool) {
	if input == "" {
		return "", "", false
	}
	quote := input[0]
	if quote != '\'' && quote != '"' {
		return "", "", false
	}
	rest := input[1:]
	end := strings.IndexByte(rest, quote)
	if end < 0 {
		return "", "", false
	}
	return rest[:end], rest[end+1:], true
}

func decodeDuckDuckGoRedirect(raw string) (string, bool) {
	decoded := html.UnescapeString(raw)
	if strings.HasPrefix(decoded, "http://") || strings.HasPrefix(decoded, "https://") {
		return decoded, true
	}
	var joined string
	switch {
	case strings.HasPrefix(decoded, "//"):
		joined = "https:" + decoded
	case strings.HasPrefix(decoded, "/"):
		joined = "https://duckduckgo.com" + decoded
	default:
		return "", false
	}
	parsed, err := url.Parse(joined)
	if err != nil {
		return "", false
	}
	if parsed.Path == "/l" || parsed.Path == "/l/" {
		if uddg := parsed.Query().Get("uddg"); uddg != "" {
			return html.UnescapeString(uddg), true
		}
	}
	return joined, true
}

func hostMatchesList(raw string, domains []string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, domain := range domains {
		normalized := normalizeDomainFilter(domain)
		if normalized == "" {
			continue
		}
		if host == normalized || strings.HasSuffix(host, "."+normalized) {
			return true
		}
	}
	return false
}

func normalizeDomainFilter(domain string) string {
	trimmed := strings.TrimSpace(domain)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Hostname() != "" {
		trimmed = parsed.Hostname()
	}
	return strings.ToLower(strings.Trim(strings.TrimSpace(trimmed), "./"))
}

func dedupeSearchHits(hits []webSearchHit) []webSearchHit {
	seen := make(map[string]struct{}, len(hits))
	out := make([]webSearchHit, 0, len(hits))
	for _, hit := range hits {
		key := strings.TrimSpace(hit.URL)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, hit)
	}
	return out
}

func isLocalHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
