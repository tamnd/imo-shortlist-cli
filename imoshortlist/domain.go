package imoshortlist

import (
	"context"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes imoshortlist as a kit Domain so a multi-domain host (ant)
// can enable it with a single blank import:
//
//	import _ "github.com/tamnd/imo-shortlist-cli/imoshortlist"
//
// The same Domain builds the standalone imoslx binary (see cli/root.go),
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
			Binary: "imoslx",
			Short:  "Browse IMO Shortlist PDFs",
			Long: `imoslx lists available International Mathematical Olympiad (IMO) Shortlist
PDFs from imo-official.org. No API key is required. It uses HEAD requests
to check availability and returns the URL and file size for each year.

imoslx is an independent tool and is not affiliated with the IMO Board.`,
			Site: Host,
			Repo: "https://github.com/tamnd/imo-shortlist-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "shortlists", Group: "read", List: true,
		Summary: "List available IMO Shortlist PDFs"}, listShortlists)
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

type listShortlistsIn struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

func listShortlists(ctx context.Context, in listShortlistsIn, emit func(*Shortlist) error) error {
	items, err := in.Client.Shortlists(ctx, in.Limit)
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

// Classify turns an input into (type, id) — not meaningful for this domain.
func (Domain) Classify(input string) (uriType, id string, err error) {
	return "", "", errs.Usage("imoshortlist:// URIs are not supported for direct resolution")
}

// Locate is the inverse of Classify.
func (Domain) Locate(uriType, id string) (string, error) {
	return "", errs.Usage("imoshortlist has no resource type %q", uriType)
}

func mapErr(err error) error {
	return err
}
