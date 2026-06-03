// Package crawler implements core web-crawling domain logic: URL normalization,
// robots.txt enforcement, content hashing, and crawl-job lifecycle.
package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Status values for a crawl attempt.
const (
	StatusPending  = "pending"
	StatusFetching = "fetching"
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusSkipped  = "skipped"
)

// URLEntry represents a URL in the frontier queue.
type URLEntry struct {
	ID          int64
	URL         string
	Host        string
	Priority    int
	NextFetchAt time.Time
	Status      string
	CreatedAt   time.Time
}

// PageFetch records the result of fetching one URL.
type PageFetch struct {
	URLHash     string
	URL         string
	StatusCode  int
	ContentHash string
	BodySize    int
	FetchedAt   time.Time
	Error       string
}

// RobotsRule caches a parsed robots.txt decision for one host.
type RobotsRule struct {
	Host       string
	UserAgent  string
	Disallowed []string
	CrawlDelay time.Duration
	FetchedAt  time.Time
}

// Job is a user-submitted crawl job (seed URL + depth limit).
type Job struct {
	ID        int64
	SeedURL   string
	MaxDepth  int
	Status    string
	CreatedAt time.Time
}

// NormalizeURL canonicalises a URL: lowercases scheme+host, strips fragment,
// resolves relative paths against base.
func NormalizeURL(raw string, base *url.URL) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", nil // skip non-HTTP URLs
	}
	return u.String(), nil
}

// URLHash returns a hex-encoded SHA-256 of the normalised URL.
func URLHash(normalised string) string {
	h := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(h[:])
}

// ContentHash returns a hex-encoded SHA-256 of body bytes.
func ContentHash(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

// ExtractLinks parses HTML body and returns all href link targets resolved
// against baseURL.
func ExtractLinks(body []byte, baseURL *url.URL) []string {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					norm, err := NormalizeURL(attr.Val, baseURL)
					if err == nil && norm != "" {
						links = append(links, norm)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}

// IsAllowed checks a URL path against disallowed prefixes from robots rules.
func IsAllowed(path string, rule *RobotsRule) bool {
	if rule == nil {
		return true
	}
	for _, prefix := range rule.Disallowed {
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return false
		}
	}
	return true
}

// ParseRobotsTxt parses a robots.txt body for the given user-agent (or "*").
// Returns disallowed paths and crawl-delay.
func ParseRobotsTxt(body []byte, userAgent string) (disallowed []string, crawlDelay time.Duration) {
	ua := strings.ToLower(userAgent)
	lines := strings.Split(string(body), "\n")
	active := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(strings.ToLower(line), "user-agent:") {
			val := strings.TrimSpace(line[len("user-agent:"):])
			active = val == "*" || strings.ToLower(val) == ua
			continue
		}
		if !active {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "disallow:") {
			path := strings.TrimSpace(line[len("disallow:"):])
			if path != "" {
				disallowed = append(disallowed, path)
			}
		}
		if strings.HasPrefix(strings.ToLower(line), "crawl-delay:") {
			val := strings.TrimSpace(line[len("crawl-delay:"):])
			var secs float64
			if _, err := io.ReadFull(strings.NewReader(val), nil); err == nil {
				if n, _ := io.ReadFull(strings.NewReader(val), nil); n == 0 {
					_ = val
				}
			}
			// simple integer parse
			for _, c := range val {
				if c >= '0' && c <= '9' {
					secs = secs*10 + float64(c-'0')
				} else {
					break
				}
			}
			if secs > 0 {
				crawlDelay = time.Duration(secs) * time.Second
			}
		}
	}
	return
}

// FetchResult holds the raw outcome of an HTTP GET.
type FetchResult struct {
	StatusCode  int
	Body        []byte
	ContentType string
	FinalURL    string
	Elapsed     time.Duration
	Error       error
}

// HTTPFetcher performs real HTTP fetches with a shared client.
type HTTPFetcher struct {
	client    *http.Client
	userAgent string
}

// NewHTTPFetcher creates a fetcher with a shared transport and a 15 s timeout.
func NewHTTPFetcher(userAgent string) *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		userAgent: userAgent,
	}
}

// Fetch performs an HTTP GET and returns the result.
func (f *HTTPFetcher) Fetch(rawURL string) FetchResult {
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return FetchResult{Error: err, Elapsed: time.Since(start)}
	}
	req.Header.Set("User-Agent", f.userAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return FetchResult{Error: err, Elapsed: time.Since(start)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MB cap
	return FetchResult{
		StatusCode:  resp.StatusCode,
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
		FinalURL:    resp.Request.URL.String(),
		Elapsed:     time.Since(start),
		Error:       err,
	}
}
