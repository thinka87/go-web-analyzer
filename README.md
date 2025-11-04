# Web Page Analyzer (Golang)

A tiny Go web app that analyzes a URL and reports:

- HTML version
- Page title
- Count of headings (H1..H6)
- Count of internal vs external links + number of inaccessible links
- Whether a login form is present
- Friendly error page with HTTP status code + description when the URL can't be fetched

> Built with idiomatic Go; simple, readable UI; concurrency for link checks; structured logging with `slog`.

## Quickstart

### Prerequisites
- Go 1.22+
- (optional) Docker

### Run locally

```bash
make run
# open http://localhost:8080
```

### Build a container

```bash
make docker
docker run --rm -p 8080:8080 web-analyzer:local
```

### Binary build

```bash
make build
./web-analyzer
```

## Assumptions & Decisions

- **HTML version**: We inspect the presence of a doctype. If present, we label it as *"HTML5 (with doctype)"*. If absent, we report *"Unknown (no doctype)"*. The standard library `golang.org/x/net/html` doesn't expose public/system IDs on the doctype, so detecting older HTML 4 variants would require a custom tokenizer.
- **Internal vs external links**: A link is *internal* if its hostname matches the analyzed page's hostname (relative links are resolved against the page); otherwise *external*. `mailto:`, `javascript:`, hash-only and empty `href`s are ignored.
- **Inaccessible links**: Checked concurrently with a worker limit (10). We try `HEAD` first, then `GET` if HEAD fails. A link is considered inaccessible if it never returns `2xx/3xx`.
- **Login form detection**: A form containing an `<input type="password">` is considered a login form. As a fallback, we also accept a form with a likely user field (username/email) plus a submit button.
- **Timeouts**: 15s for the initial page fetch and 8s per link check.
- **Error display**: If the upstream server returns a non-2xx, we surface that HTTP status and up to 1KB of the body as the message, per requirements.

These choices aim to keep the solution robust without excessive complexity for a test task. See the code for clear extension points.

## Project Layout

```
.
├── analyzer.go        # core analysis logic (pure-ish and testable)
├── handlers.go        # HTTP handlers & HTML templates rendering
├── main.go            # server boot, logging, graceful shutdown
├── templates/
│   ├── index.html     # input form
│   └── result.html    # result or error view
├── go.mod
├── Dockerfile
├── Makefile
├── README.md
└── internal/
```

(We left `internal/` empty in this small app. For bigger projects, move packages there.)

## Logging, Concurrency & Code Quality

- Uses `log/slog` for structured logs.
- Concurrency via goroutines, channels (semaphore), and a `WaitGroup` for link checks.
- Clean error wrapping with an `AnalyzeError` type that carries HTTP status codes.

We followed a compact, idiomatic Go style instead of "Java-style" patterns, kept JS to a minimum, and focused on correct backend behavior and a working UI.
See the included *Checklist for Golang Test* and how this project addresses it in the notes below.

## Tests

Run tests:

```bash
make test
```

- Unit tests use `httptest` servers; no real HTTP calls are made.
- We cover: URL normalization, heading counts, internal/external classification, login form detection, and inaccessible link counting using stub servers.

## Possible Improvements

- Parse and expose legacy HTML public/system IDs by adding a doctype-aware tokenizer.
- Show a sample of broken link URLs and their statuses (capped list).
- Add pprof endpoints and Prometheus metrics (counters/timers for analysis duration, links scanned).
- Cache results per URL temporarily to avoid re-checking on repeated submissions.
- Add client-side validation for URLs and nicer styling.
- Internationalization for the UI.
- Add integration tests for end-to-end flow.
- CI pipeline with `golangci-lint`, tests, and container build.

## Deployment

- **Docker**: build with the provided `Dockerfile` (distroless final image), then run on any container platform.
- **Binary**: a single static binary, no runtime deps; just copy `templates/` alongside it.
- Add environment variable `ADDR` to change the listen address (default `:8080`).

## Checklist mapping

- **Code Organization & Go standards**: idiomatic Go, small packages, no over-engineered patterns.
- **Documentation**: detailed README with overview, setup, usage, assumptions, and improvements.
- **Tools & Quality**: `slog` for logs; robust error handling; appropriate concurrency; no unnecessary JS.
- **Deployment**: Dockerfile + Makefile targets.
- **Testing**: Unit tests use `httptest` (no real external requests).

'''
