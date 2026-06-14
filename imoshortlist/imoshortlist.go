// Package imoshortlist is the library behind the imoslx command: the HTTP
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
	"strconv"
	"sync"
	"time"
)

// Host is the IMO official site host.
const Host = "www.imo-official.org"

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
		UserAgent: "Mozilla/5.0 (compatible; imo-shortlist-cli/dev; +https://github.com/tamnd/imo-shortlist-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
		MinYear:   2006,
		MaxYear:   2024,
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

// Shortlists probes each year from MaxYear down to MinYear with a HEAD request
// and returns those that return HTTP 200, newest first.
// If limit > 0, at most limit items are returned.
func (c *Client) Shortlists(ctx context.Context, limit int) ([]Shortlist, error) {
	var result []Shortlist
	rank := 0
	for year := c.cfg.MaxYear; year >= c.cfg.MinYear; year-- {
		url := fmt.Sprintf("%s/problems/IMO%dSL.pdf", c.cfg.BaseURL, year)
		status, size, err := c.head(ctx, url)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			continue
		}
		rank++
		result = append(result, Shortlist{
			Rank:      rank,
			Year:      year,
			URL:       url,
			SizeBytes: size,
		})
		if limit > 0 && rank >= limit {
			break
		}
	}
	return result, nil
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
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}
