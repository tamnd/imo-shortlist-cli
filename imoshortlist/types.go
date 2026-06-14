package imoshortlist

// Shortlist is one available IMO Shortlist PDF.
type Shortlist struct {
	Rank      int    `json:"rank"`
	Year      int    `json:"year"`
	URL       string `json:"url"`
	SizeBytes int64  `json:"size_bytes"` // -1 if Content-Length absent
}
