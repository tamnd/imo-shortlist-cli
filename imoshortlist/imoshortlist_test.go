package imoshortlist_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	imoshortlist "github.com/tamnd/imo-shortlist-cli/imoshortlist"
)

func newTestClient(ts *httptest.Server, minYear, maxYear int) *imoshortlist.Client {
	cfg := imoshortlist.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.MinYear = minYear
	cfg.MaxYear = maxYear
	return imoshortlist.NewClient(cfg)
}

// mockHandler returns 200 for 2024 and 2023 with Content-Length, 404 for 2022.
func mockHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/problems/IMO2024SL.pdf":
			w.Header().Set("Content-Length", "1831599")
			w.WriteHeader(http.StatusOK)
		case "/problems/IMO2023SL.pdf":
			w.Header().Set("Content-Length", "1552982")
			w.WriteHeader(http.StatusOK)
		case "/problems/IMO2022SL.pdf":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestShortlistsSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Length", "1831599")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2024, 2024)
	_, err := c.Shortlists(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestShortlistsParsesItems(t *testing.T) {
	srv := httptest.NewServer(mockHandler(t))
	defer srv.Close()

	c := newTestClient(srv, 2022, 2024)
	items, err := c.Shortlists(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	if items[0].Year != 2024 {
		t.Errorf("items[0].Year = %d, want 2024", items[0].Year)
	}
	if items[0].Rank != 1 {
		t.Errorf("items[0].Rank = %d, want 1", items[0].Rank)
	}
	if items[0].SizeBytes != 1831599 {
		t.Errorf("items[0].SizeBytes = %d, want 1831599", items[0].SizeBytes)
	}
	if items[1].Year != 2023 {
		t.Errorf("items[1].Year = %d, want 2023", items[1].Year)
	}
	if items[1].Rank != 2 {
		t.Errorf("items[1].Rank = %d, want 2", items[1].Rank)
	}
	if items[1].SizeBytes != 1552982 {
		t.Errorf("items[1].SizeBytes = %d, want 1552982", items[1].SizeBytes)
	}
	want := fmt.Sprintf("%s/problems/IMO2024SL.pdf", srv.URL)
	if items[0].URL != want {
		t.Errorf("items[0].URL = %q, want %q", items[0].URL, want)
	}
}

func TestShortlistsLimitRespected(t *testing.T) {
	srv := httptest.NewServer(mockHandler(t))
	defer srv.Close()

	c := newTestClient(srv, 2022, 2024)
	items, err := c.Shortlists(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Year != 2024 {
		t.Errorf("items[0].Year = %d, want 2024", items[0].Year)
	}
}

func TestShortlistsRetriesOn503(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := imoshortlist.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 3
	cfg.MinYear = 2024
	cfg.MaxYear = 2024
	c := imoshortlist.NewClient(cfg)

	items, err := c.Shortlists(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].SizeBytes != 100 {
		t.Errorf("SizeBytes = %d, want 100", items[0].SizeBytes)
	}
}
