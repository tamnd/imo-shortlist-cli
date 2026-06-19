package imoshortlist_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// mockHandler returns 200 for 2024 and 404 for others.
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
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestListSendsUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Length", "1831599")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2024, 2024)
	_, err := c.List(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListParsesItems(t *testing.T) {
	srv := httptest.NewServer(mockHandler(t))
	defer srv.Close()

	c := newTestClient(srv, 2023, 2024)
	items, err := c.List(context.Background(), 0)
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
	if items[1].SizeBytes != 1552982 {
		t.Errorf("items[1].SizeBytes = %d, want 1552982", items[1].SizeBytes)
	}
	want := fmt.Sprintf("%s/problems/IMO2024SL.pdf", srv.URL)
	if items[0].PDFURL != want {
		t.Errorf("items[0].PDFURL = %q, want %q", items[0].PDFURL, want)
	}
}

func TestListLimitRespected(t *testing.T) {
	srv := httptest.NewServer(mockHandler(t))
	defer srv.Close()

	c := newTestClient(srv, 2022, 2024)
	items, err := c.List(context.Background(), 1)
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

func TestListRetriesOn503(t *testing.T) {
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

	items, err := c.List(context.Background(), 0)
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

func TestShortlistsBackcompat(t *testing.T) {
	// Shortlists() is an alias for List(); verify it still works.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "999")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2024, 2024)
	items, err := c.Shortlists(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Year != 2024 {
		t.Fatalf("Shortlists() back-compat broken: %+v", items)
	}
}

func TestShortlistForYear_found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1831599")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2020, 2025)
	entry, ok, err := c.ShortlistForYear(context.Background(), 2024)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true for a 200 response")
	}
	if entry.Year != 2024 {
		t.Errorf("Year = %d, want 2024", entry.Year)
	}
}

func TestShortlistForYear_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2020, 2025)
	_, ok, err := c.ShortlistForYear(context.Background(), 2024)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected ok=false for a 404 response")
	}
}

func TestProblem_valid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1234")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2020, 2025)
	p, err := c.Problem(context.Background(), 2023, "A3")
	if err != nil {
		t.Fatal(err)
	}
	if p.Year != 2023 {
		t.Errorf("Year = %d, want 2023", p.Year)
	}
	if p.Code != "A3" {
		t.Errorf("Code = %q, want A3", p.Code)
	}
	if p.Category != "Algebra" {
		t.Errorf("Category = %q, want Algebra", p.Category)
	}
	if !strings.Contains(p.PDFURL, "IMO2023SL.pdf") {
		t.Errorf("PDFURL %q does not contain IMO2023SL.pdf", p.PDFURL)
	}
}

func TestProblem_lowercaseCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1234")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2020, 2025)
	p, err := c.Problem(context.Background(), 2023, "n1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Code != "N1" {
		t.Errorf("Code = %q, want N1 (uppercase)", p.Code)
	}
	if p.Category != "Number Theory" {
		t.Errorf("Category = %q, want Number Theory", p.Category)
	}
}

func TestProblem_invalidCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1234")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2020, 2025)
	_, err := c.Problem(context.Background(), 2023, "X9")
	if err == nil {
		t.Fatal("expected error for invalid code X9")
	}
}

func TestProblem_pdfNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2020, 2025)
	_, err := c.Problem(context.Background(), 2023, "A1")
	if err == nil {
		t.Fatal("expected error when PDF not found")
	}
}

func TestExport_oneYear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv, 2020, 2025)
	problems, err := c.Export(context.Background(), 2023)
	if err != nil {
		t.Fatal(err)
	}
	// 4 categories × 7 problems = 28
	if len(problems) != 28 {
		t.Errorf("got %d problems, want 28 (4 cats x 7)", len(problems))
	}
	if problems[0].Year != 2023 {
		t.Errorf("problems[0].Year = %d, want 2023", problems[0].Year)
	}
	if problems[0].Code != "A1" {
		t.Errorf("problems[0].Code = %q, want A1", problems[0].Code)
	}
	if problems[0].Category != "Algebra" {
		t.Errorf("problems[0].Category = %q, want Algebra", problems[0].Category)
	}
}

func TestExport_twoYears(t *testing.T) {
	srv := httptest.NewServer(mockHandler(t))
	defer srv.Close()

	c := newTestClient(srv, 2023, 2024)
	problems, err := c.Export(context.Background(), 0) // 0 = all years
	if err != nil {
		t.Fatal(err)
	}
	// 2 years × 4 cats × 7 problems = 56
	if len(problems) != 56 {
		t.Errorf("got %d problems, want 56 (2 years x 28)", len(problems))
	}
}

func TestInfo_basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/problems/IMO2024SL.pdf", "/problems/IMO2023SL.pdf":
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, 2022, 2024)
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.MinYear != 2022 {
		t.Errorf("MinYear = %d, want 2022", info.MinYear)
	}
	if info.MaxYear != 2024 {
		t.Errorf("MaxYear = %d, want 2024", info.MaxYear)
	}
	if info.YearsTotal != 3 {
		t.Errorf("YearsTotal = %d, want 3", info.YearsTotal)
	}
	if info.PDFsAvail != 2 {
		t.Errorf("PDFsAvail = %d, want 2", info.PDFsAvail)
	}
}
