package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"example.com", true},
		{"https://example.com", true},
		{"", false},
		{"http://", false},
	}
	for _, c := range cases {
		_, err := normalizeURL(c.in)
		if c.ok && err != nil {
			t.Fatalf("expected ok for %q, got %v", c.in, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("expected error for %q", c.in)
		}
	}
}

func TestCountHeadings(t *testing.T) {
	html := ` + "`" + `<!doctype html><title>x</title><h1>a</h1><h2>b</h2><h2>c</h2><h3>d</h3>` + "`" + `
	doc := mustDoc(html, t)
	h := countHeadings(doc)
	if h["h1"] != 1 || h["h2"] != 2 || h["h3"] != 1 {
		t.Fatalf("unexpected counts: %#v", h)
	}
}

func TestClassifyLinks(t *testing.T) {
	base := mustURL("https://example.com/page")
	html := `<a href="/x">a</a><a href="https://example.com/y">b</a><a href="https://other.com/">c</a>`
	doc := mustDoc(html, t)
	in, ex, all := classifyLinks(doc, base)
	if in != 2 || ex != 1 || len(all) != 3 {
		t.Fatalf("got in=%d ex=%d all=%d", in, ex, len(all))
	}
}

func TestHasLoginForm(t *testing.T) {
	html := `<form><input type="password"/><button type="submit">Go</button></form>`
	doc := mustDoc(html, t)
	if !hasLoginForm(doc) {
		t.Fatal("expected login form")
	}
}

func TestCheckLinksAccessibility(t *testing.T) {
	// good server
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer okSrv.Close()
	// bad server
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer badSrv.Close()

	links := []string{okSrv.URL, badSrv.URL}
	n := checkLinksAccessibility(context.Background(), links, 5)
	if n != 1 {
		t.Fatalf("expected 1 inaccessible, got %d", n)
	}
}

// helpers
func mustDoc(s string, t *testing.T) *goquery.Document {
	t.Helper()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func mustURL(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}
