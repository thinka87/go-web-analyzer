package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// AnalyzeError wraps network/HTTP issues to present status code and description.
type AnalyzeError struct {
	StatusCode int
	Err        error
}

func (e *AnalyzeError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("HTTP %d: %v", e.StatusCode, e.Err)
	}
	return e.Err.Error()
}

type AnalyzeResult struct {
	HTMLVersion  string
	Title        string
	Headings     map[string]int // h1..h6
	Links        LinkSummary
	HasLoginForm bool
	Inaccessible int
}

type LinkSummary struct {
	Internal int
	External int
	Total    int
}

// AnalyzeURL fetches and analyzes a page.
func AnalyzeURL(ctx context.Context, raw string) (*AnalyzeResult, error) {
	parsed, err := normalizeURL(raw)
	if err != nil {
		return nil, &AnalyzeError{Err: fmt.Errorf("invalid URL: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed, nil)
	if err != nil {
		return nil, &AnalyzeError{Err: err}
	}
	// custom client with timeout and redirect limit
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &AnalyzeError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &AnalyzeError{StatusCode: resp.StatusCode, Err: fmt.Errorf("%s", strings.TrimSpace(string(b)))}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &AnalyzeError{Err: err}
	}
	rootNode, err := html.Parse(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, &AnalyzeError{Err: err}
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, &AnalyzeError{Err: err}
	}

	result := &AnalyzeResult{
		HTMLVersion: detectHTMLVersion(rootNode),
		Title:       strings.TrimSpace(doc.Find("title").First().Text()),
		Headings:    countHeadings(doc),
	}
	
	internal, external, allLinks := classifyLinks(doc, req.URL)
	result.Links = LinkSummary{Internal: internal, External: external, Total: internal + external}

	// Check accessibility of links with limited concurrency
	result.Inaccessible = checkLinksAccessibility(ctx, allLinks, 10)

	// Detect login form
	result.HasLoginForm = hasLoginForm(doc)

	return result, nil
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty URL")
	}
	// If scheme missing, default to https://
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", errors.New("missing host")
	}
	return u.String(), nil
}

func detectHTMLVersion(root *html.Node) string {
	// html.Parse gives us the Document node; the doctype is a child of it.
	for n := root.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.DoctypeNode {
			// The go net/html DoctypeNode stores Data (e.g., "html"); public/system ids are not exposed.
			// We'll infer: if Doctype present -> HTML5 unless we later extend with tokenizer peeking.
			return "HTML5 (with doctype)"
		}
	}
	// No doctype found. Some legacy pages (HTML 2/3/4) might omit, but modern HTML5 also allows omission.
	// We'll treat missing doctype as "Unknown/Quirks" to be precise.
	return "Unknown (no doctype)"
}

func countHeadings(doc *goquery.Document) map[string]int {
	h := map[string]int{"h1": 0, "h2": 0, "h3": 0, "h4": 0, "h5": 0, "h6": 0}
	for level := 1; level <= 6; level++ {
		selector := fmt.Sprintf("h%d", level)
		h[selector] = doc.Find(selector).Length()
	}
	return h
}

func classifyLinks(doc *goquery.Document, base *url.URL) (internal, external int, all []string) {
	seen := make(map[string]struct{})
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "#") {
			return
		}
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		if !u.IsAbs() {
			u = base.ResolveReference(u)
		}
		// normalize
		u.Fragment = ""
		u.RawQuery = "" // ignore query for classification
		final := u.String()
		if _, ok := seen[final]; ok {
			return
		}
		seen[final] = struct{}{}

		if sameHost(u, base) {
			internal++
		} else {
			external++
		}
		all = append(all, u.String())
	})
	return
}

func sameHost(a, b *url.URL) bool {
	return strings.EqualFold(a.Hostname(), b.Hostname())
}

func hasLoginForm(doc *goquery.Document) bool {
	found := false
	doc.Find("form").EachWithBreak(func(i int, f *goquery.Selection) bool {
		pass := f.Find(`input[type="password"]`).Length() > 0
		if pass {
			found = true
			return false
		}
		// Heuristic: presence of inputs with typical login names plus submit
		hasUser := f.Find(`input[name="username"], input[name="email"], input[id*="user"], input[id*="email"]`).Length() > 0
		hasSubmit := f.Find(`button[type="submit"], input[type="submit"]`).Length() > 0
		if hasUser && hasSubmit {
			found = true
			return false
		}
		return true
	})
	return found
}

func checkLinksAccessibility(ctx context.Context, links []string, maxConcurrent int) int {
	if len(links) == 0 {
		return 0
	}
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	inaccessible := 0

	client := &http.Client{Timeout: 8 * time.Second}

	for _, link := range links {
		link := link
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}         // acquire
			defer func() { <-sem }()  // release

			ok := linkAccessible(ctx, client, link)
			if !ok {
				mu.Lock()
				inaccessible++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return inaccessible
}

func linkAccessible(ctx context.Context, client *http.Client, link string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400 {
		resp.Body.Close()
		return true
	}
	// fallback to GET in case HEAD not allowed
	reqGet, err2 := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err2 != nil {
		return false
	}
	resp, err = client.Do(reqGet)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// ---- Metrics (optional extension points) ----
// In a real system we'd wire Prometheus counters/histograms; left minimal to avoid complexity in a test task.
