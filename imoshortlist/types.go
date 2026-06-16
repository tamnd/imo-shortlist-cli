package imoshortlist

// ShortlistEntry is one available IMO Shortlist PDF, returned by List.
type ShortlistEntry struct {
	Rank      int    `json:"rank"`
	Year      int    `json:"year"`
	PDFURL    string `json:"pdf_url"`
	SizeBytes int64  `json:"size_bytes"` // -1 if Content-Length absent
}

// Problem is one problem inside a shortlist PDF.
// Problems are derived from the year and a code like "A3" or "N1".
// The PDF covers all problems for that year; there is no per-problem URL.
type Problem struct {
	Year     int    `json:"year"`
	Code     string `json:"code"`     // e.g. "A3", "N1"
	Category string `json:"category"` // Algebra, Combinatorics, Geometry, Number Theory
	PDFURL   string `json:"pdf_url"`  // URL to the shortlist PDF for that year
}

// Info summarises what years are available.
type Info struct {
	MinYear    int `json:"min_year"`
	MaxYear    int `json:"max_year"`
	YearsTotal int `json:"years_total"` // number of probed years
	PDFsAvail  int `json:"pdfs_avail"`  // years where the PDF is reachable
}
