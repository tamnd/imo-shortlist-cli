package imoshortlist

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes imoshortlist as a kit Domain so a multi-domain host (ant)
// can enable it with a single blank import:
//
//	import _ "github.com/tamnd/imo-shortlist-cli/imoshortlist"
//
// The same Domain builds the standalone imoshortlist binary (see cli/root.go),
// so the binary and any host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the imoshortlist driver. It carries no state.
type Domain struct{}

// Info describes the scheme, accepted hostnames, and the binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "imoshortlist",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "imoshortlist",
			Short:  "Browse IMO Shortlist PDFs from the command line",
			Long: `imoshortlist lists available International Mathematical Olympiad (IMO) Shortlist
PDFs from imo-official.org. No API key is required. It uses HEAD requests
to check PDF availability and returns the URL and file size for each year.

Commands:
  list       List available IMO Shortlist PDFs (newest first)
  problem    Show a specific problem by year and code (e.g. A3, N1)
  export     Export all problems for a year (or all years)
  info       Show aggregate statistics

imoshortlist is an independent tool and is not affiliated with the IMO Board.`,
			Site: Host,
			Repo: "https://github.com/tamnd/imo-shortlist-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "list", Group: "read", List: true,
		Summary: "List available IMO Shortlist PDFs"}, listShortlists)

	kit.Handle(app, kit.OpMeta{Name: "problem", Group: "read", Single: true,
		Summary: "Show a specific problem by year and code",
		Args:    []kit.Arg{{Name: "year", Help: "IMO year, e.g. 2023"}, {Name: "code", Help: "problem code, e.g. A3 or N1"}}}, getProblem)

	kit.Handle(app, kit.OpMeta{Name: "export", Group: "read", List: true,
		Summary: "Export all problems for a year (or all years)"}, exportProblems)

	kit.Handle(app, kit.OpMeta{Name: "info", Group: "read", Single: true,
		Summary: "Show aggregate statistics about IMO Shortlist availability"}, getInfo)
}

// newClient builds the Client from the kit config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- list ---

type listShortlistsIn struct {
	Year   int     `kit:"flag"        help:"filter to a specific year (0 = all)"`
	Limit  int     `kit:"flag,inherit" help:"max results (0 = no limit)"`
	Client *Client `kit:"inject"`
}

func listShortlists(ctx context.Context, in listShortlistsIn, emit func(*ShortlistEntry) error) error {
	if in.Year > 0 {
		entry, ok, err := in.Client.ShortlistForYear(ctx, in.Year)
		if err != nil {
			return mapErr(err)
		}
		if !ok {
			return errs.NotFound("no shortlist PDF found for year %d", in.Year)
		}
		return emit(&entry)
	}
	items, err := in.Client.List(ctx, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- problem ---

type getProblemIn struct {
	Year   int     `kit:"arg"    help:"IMO year, e.g. 2023"`
	Code   string  `kit:"arg"    help:"problem code, e.g. A3 or N1"`
	Client *Client `kit:"inject"`
}

func getProblem(ctx context.Context, in getProblemIn, emit func(*Problem) error) error {
	if in.Year <= 0 {
		return errs.Usage("year must be a positive integer, e.g. 2023")
	}
	if in.Code == "" {
		return errs.Usage("code must be a letter (A/C/G/N) followed by a number, e.g. A3")
	}
	p, err := in.Client.Problem(ctx, in.Year, in.Code)
	if err != nil {
		return mapErr(err)
	}
	return emit(&p)
}

// --- export ---

type exportProblemsIn struct {
	Year   int     `kit:"flag"   help:"export problems for a specific year (0 = all years)"`
	Client *Client `kit:"inject"`
}

func exportProblems(ctx context.Context, in exportProblemsIn, emit func(*Problem) error) error {
	problems, err := in.Client.Export(ctx, in.Year)
	if err != nil {
		return mapErr(err)
	}
	for i := range problems {
		if err := emit(&problems[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- info ---

type getInfoIn struct {
	Client *Client `kit:"inject"`
}

func getInfo(ctx context.Context, in getInfoIn, emit func(*Info) error) error {
	info, err := in.Client.Info(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(&info)
}

// Classify turns an input into (type, id) — supports "year/code" format.
func (Domain) Classify(input string) (uriType, id string, err error) {
	// Accept "2023/A3" style input.
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		if _, err := strconv.Atoi(parts[0]); err == nil {
			return "problem", input, nil
		}
	}
	// Accept bare year.
	if _, err := strconv.Atoi(input); err == nil {
		return "edition", input, nil
	}
	return "", "", errs.Usage("cannot classify %q: use a year (e.g. 2023) or year/code (e.g. 2023/A3)", input)
}

// Locate is the inverse of Classify.
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "edition":
		year, err := strconv.Atoi(id)
		if err != nil {
			return "", errs.Usage("invalid year %q", id)
		}
		return fmt.Sprintf("https://%s/problems/IMO%dSL.pdf", Host, year), nil
	case "problem":
		return fmt.Sprintf("imoshortlist://problem/%s", id), nil
	default:
		return "", errs.Usage("imoshortlist has no resource type %q", uriType)
	}
}

func mapErr(err error) error {
	return err
}
