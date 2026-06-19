// Package imoshortlist is the library behind the imoshortlist command: the HTTP
// client, request shaping, and the typed data models for IMO Shortlist PDFs.
//
// The client probes PDF availability with HEAD requests at
// https://www.imo-official.org/problems/IMO{YEAR}SL.pdf
// No authentication is required. It sets a real User-Agent, paces requests,
// and retries transient 429/5xx errors with exponential back-off.
package imoshortlist

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Host is the IMO official site host.
const Host = "www.imo-official.org"

// codeRE matches a problem code like "A3", "N1", "G6", "C2" (case-insensitive).
var codeRE = regexp.MustCompile(`(?i)^([ACGN])(\d+)$`)

// categoryOf maps the letter prefix to a full category name.
var categoryOf = map[string]string{
	"A": "Algebra",
	"C": "Combinatorics",
	"G": "Geometry",
	"N": "Number Theory",
}

// problemsPerCategory is the number of problems per category per year.
// The actual count varies; 7 is a safe upper bound for recent years.
const problemsPerCategory = 7

// categories lists the four shortlist categories in the standard order.
var categories = []string{"A", "C", "G", "N"}

// Config holds constructor parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
	MinYear   int
	MaxYear   int
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://www.imo-official.org",
		UserAgent: "Mozilla/5.0 (compatible; imoshortlist-cli/dev; +https://github.com/tamnd/imo-shortlist-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
		MinYear:   2006,
		MaxYear:   2025,
	}
}

// Client talks to imo-official.org via HEAD requests to check PDF availability.
type Client struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client ready to use.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

// pdfURL returns the full URL for the shortlist PDF for the given year.
func (c *Client) pdfURL(year int) string {
	return fmt.Sprintf("%s/problems/IMO%dSL.pdf", c.cfg.BaseURL, year)
}

// List probes each year from MaxYear down to MinYear with a HEAD request
// and returns those that return HTTP 200, newest first.
// If limit > 0, at most limit items are returned.
func (c *Client) List(ctx context.Context, limit int) ([]ShortlistEntry, error) {
	var result []ShortlistEntry
	rank := 0
	for year := c.cfg.MaxYear; year >= c.cfg.MinYear; year-- {
		url := c.pdfURL(year)
		status, size, err := c.head(ctx, url)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			continue
		}
		rank++
		result = append(result, ShortlistEntry{
			Rank:      rank,
			Year:      year,
			PDFURL:    url,
			SizeBytes: size,
		})
		if limit > 0 && rank >= limit {
			break
		}
	}
	return result, nil
}

// Shortlists is an alias for List kept for backward compatibility with v0.1 tests.
func (c *Client) Shortlists(ctx context.Context, limit int) ([]ShortlistEntry, error) {
	return c.List(ctx, limit)
}

// ShortlistForYear probes the PDF for a single year.
// Returns (entry, true, nil) if found, (zero, false, nil) if not found,
// or (zero, false, err) on network error.
func (c *Client) ShortlistForYear(ctx context.Context, year int) (ShortlistEntry, bool, error) {
	url := c.pdfURL(year)
	status, size, err := c.head(ctx, url)
	if err != nil {
		return ShortlistEntry{}, false, err
	}
	if status != http.StatusOK {
		return ShortlistEntry{}, false, nil
	}
	return ShortlistEntry{Rank: 1, Year: year, PDFURL: url, SizeBytes: size}, true, nil
}

// Problem returns a single problem by year and code (e.g. "A3", "N1").
// It first checks that the shortlist PDF for that year is available, then
// derives the Problem from the validated year and code.
func (c *Client) Problem(ctx context.Context, year int, code string) (Problem, error) {
	m := codeRE.FindStringSubmatch(code)
	if m == nil {
		return Problem{}, fmt.Errorf("invalid problem code %q: use letter (A/C/G/N) followed by a number, e.g. A3", code)
	}
	letter := strings.ToUpper(m[1])
	cat, ok := categoryOf[letter]
	if !ok {
		return Problem{}, fmt.Errorf("unknown category letter %q", letter)
	}

	url := c.pdfURL(year)
	status, _, err := c.head(ctx, url)
	if err != nil {
		return Problem{}, err
	}
	if status != http.StatusOK {
		return Problem{}, fmt.Errorf("no shortlist PDF found for year %d", year)
	}

	return Problem{
		Year:     year,
		Code:     strings.ToUpper(code),
		Category: cat,
		PDFURL:   url,
	}, nil
}

// Export returns all problems for a given year (all 4 categories × problemsPerCategory).
// If year <= 0, it exports problems for all available years (MaxYear down to MinYear).
func (c *Client) Export(ctx context.Context, year int) ([]Problem, error) {
	var years []int
	if year > 0 {
		// Check a single year.
		url := c.pdfURL(year)
		status, _, err := c.head(ctx, url)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("no shortlist PDF found for year %d", year)
		}
		years = []int{year}
	} else {
		// Probe all years.
		entries, err := c.List(ctx, 0)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			years = append(years, e.Year)
		}
	}

	var problems []Problem
	for _, y := range years {
		url := c.pdfURL(y)
		for _, cat := range categories {
			for n := 1; n <= problemsPerCategory; n++ {
				code := fmt.Sprintf("%s%d", cat, n)
				problems = append(problems, Problem{
					Year:     y,
					Code:     code,
					Category: categoryOf[cat],
					PDFURL:   url,
				})
			}
		}
	}
	return problems, nil
}

// Info probes all years and returns aggregate statistics.
func (c *Client) Info(ctx context.Context) (Info, error) {
	total := 0
	avail := 0
	for year := c.cfg.MinYear; year <= c.cfg.MaxYear; year++ {
		total++
		url := c.pdfURL(year)
		status, _, err := c.head(ctx, url)
		if err != nil {
			return Info{}, err
		}
		if status == http.StatusOK {
			avail++
		}
	}
	return Info{
		MinYear:    c.cfg.MinYear,
		MaxYear:    c.cfg.MaxYear,
		YearsTotal: total,
		PDFsAvail:  avail,
	}, nil
}

// head issues a HEAD request with retry logic.
// Returns (statusCode, contentLength, error).
// contentLength is -1 if the header is absent or non-numeric.
func (c *Client) head(ctx context.Context, url string) (int, int64, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, -1, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		status, size, retry, err := c.do(ctx, url)
		if err == nil {
			return status, size, nil
		}
		lastErr = err
		if !retry {
			return status, size, err
		}
	}
	return 0, -1, fmt.Errorf("head %s: %w", url, lastErr)
}

// do issues one HEAD request without retry.
// Returns (statusCode, contentLength, retryable, error).
func (c *Client) do(ctx context.Context, url string) (int, int64, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, -1, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, -1, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return resp.StatusCode, -1, true, fmt.Errorf("http %d", resp.StatusCode)
	}

	size := int64(-1)
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
			size = n
		}
	}

	return resp.StatusCode, size, false, nil
}

// pace enforces the inter-request rate limit.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		return 5 * time.Second
	}
	return d
}
