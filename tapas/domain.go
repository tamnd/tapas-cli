package tapas

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// init registers the Domain so a blank import in a multi-domain host enables
// the tapas:// driver.
func init() { kit.Register(Domain{}) }

// Domain is the tapas.io driver.
type Domain struct{}

// Info describes the scheme, hosts, and identity for the kit framework.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "tapas",
		Hosts:  []string{Host, "www.tapas.io"},
		Identity: kit.Identity{
			Binary: "tapas",
			Short:  "Read Tapas.io comics and novel series",
			Long: `tapas reads public Tapas.io data: series metadata and episode lists.

No account is needed for public content. Discovery is via the public sitemaps
at tapas.io/sitemap-comic.xml and tapas.io/sitemap-novel.xml.

Quick start:
  tapas top -n 5                   top 5 comic series (by recency)
  tapas top --type novel -n 5      top 5 novel series
  tapas search romance -n 5        series with "romance" in the slug
  tapas series MATCHPOINT          fetch series details
  tapas episodes 329873 -n 10      list 10 episodes of series 329873`,
			Site: Host,
			Repo: "https://github.com/tamnd/tapas-cli",
		},
	}
}

// Register installs the client factory and all operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "series",
		List:    true,
		Summary: "Search series by slug keyword",
		Args:    []kit.Arg{{Name: "query", Help: "keyword to search in slugs"}},
	}, searchSeries)

	kit.Handle(app, kit.OpMeta{
		Name:    "top",
		Group:   "series",
		List:    true,
		Summary: "List top (most recently updated) series from the sitemap",
	}, topSeries)

	kit.Handle(app, kit.OpMeta{
		Name:     "series",
		Group:    "series",
		Single:   true,
		Resolver: true,
		URIType:  "series",
		Summary:  "Fetch series details by slug or id",
		Args:     []kit.Arg{{Name: "ref", Help: "series slug, numeric id, or URL"}},
	}, getSeries)

	kit.Handle(app, kit.OpMeta{
		Name:    "episodes",
		Group:   "episodes",
		List:    true,
		Summary: "List episodes for a series",
		Args:    []kit.Arg{{Name: "id", Help: "series numeric id or slug"}},
	}, listEpisodes)
}

// newClient builds a Client from the kit-resolved Config.
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

// --- inputs ---

type searchInput struct {
	Query  string  `kit:"arg" help:"keyword to search in series slugs"`
	Type   string  `kit:"flag" help:"series type: comic or novel"`
	Limit  int     `kit:"flag,inherit" help:"max results" default:"10"`
	Client *Client `kit:"inject"`
}

type topInput struct {
	Type   string  `kit:"flag" help:"series type: comic or novel" default:"comic"`
	Limit  int     `kit:"flag,inherit" help:"max results" default:"20"`
	Client *Client `kit:"inject"`
}

type seriesInput struct {
	Ref    string  `kit:"arg" help:"series slug, numeric id, or URL"`
	Client *Client `kit:"inject"`
}

type episodesInput struct {
	ID     string  `kit:"arg" help:"series numeric id or slug"`
	Limit  int     `kit:"flag,inherit" help:"max episodes" default:"20"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchSeries(ctx context.Context, in searchInput, emit func(SeriesStub) error) error {
	stubs, err := fetchSitemapsByType(ctx, in.Client, in.Type)
	if err != nil {
		return mapErr(err)
	}
	results := SearchSitemap(stubs, in.Query, in.Limit)
	if len(results) == 0 {
		return errs.NotFound("no series found for %q", in.Query)
	}
	for _, s := range results {
		if err := emit(s); err != nil {
			return err
		}
	}
	return nil
}

func topSeries(ctx context.Context, in topInput, emit func(SeriesStub) error) error {
	seriesType := in.Type
	if seriesType == "" {
		seriesType = "comic"
	}
	stubs, err := in.Client.FetchSitemap(ctx, seriesType)
	if err != nil {
		return mapErr(err)
	}
	if len(stubs) == 0 {
		return errs.NotFound("no series found in sitemap")
	}
	limit := in.Limit
	if limit > 0 && len(stubs) > limit {
		stubs = stubs[:limit]
	}
	for _, s := range stubs {
		if err := emit(s); err != nil {
			return err
		}
	}
	return nil
}

func getSeries(ctx context.Context, in seriesInput, emit func(*Series) error) error {
	slug := seriesSlug(in.Ref)
	s, err := in.Client.GetSeries(ctx, slug)
	if err != nil {
		return mapErr(err)
	}
	return emit(s)
}

func listEpisodes(ctx context.Context, in episodesInput, emit func(Episode) error) error {
	episodes, err := in.Client.GetEpisodes(ctx, in.ID, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	if len(episodes) == 0 {
		return errs.NotFound("no episodes found for series %q", in.ID)
	}
	for _, ep := range episodes {
		if err := emit(ep); err != nil {
			return err
		}
	}
	return nil
}

// fetchSitemapsByType fetches comic and/or novel sitemaps based on the type flag.
func fetchSitemapsByType(ctx context.Context, c *Client, seriesType string) ([]SeriesStub, error) {
	switch strings.ToLower(seriesType) {
	case "comic":
		return c.FetchSitemap(ctx, "comic")
	case "novel":
		return c.FetchSitemap(ctx, "novel")
	default:
		comics, err := c.FetchSitemap(ctx, "comic")
		if err != nil {
			return nil, err
		}
		novels, err := c.FetchSitemap(ctx, "novel")
		if err != nil {
			return comics, nil
		}
		return append(comics, novels...), nil
	}
}

// Classify turns any accepted input into the canonical (uriType, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("tapas: empty input")
	}
	// Full URL: https://tapas.io/series/MATCHPOINT/info or https://tapas.io/series/MATCHPOINT
	if u, parseErr := url.Parse(input); parseErr == nil && (u.Scheme == "http" || u.Scheme == "https") {
		slug := seriesSlugFromPath(u.Path)
		if slug != "" {
			return "series", slug, nil
		}
		return "", "", errs.Usage("tapas: could not extract series slug from URL: %q", input)
	}
	// Bare slug or numeric id
	return "series", input, nil
}

// Locate returns the canonical URL for a (uriType, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "series":
		return fmt.Sprintf("https://tapas.io/series/%s/info", id), nil
	}
	return "", errs.Usage("tapas has no resource type %q", uriType)
}

// seriesSlug extracts the slug from any input (URL, path, or bare slug).
func seriesSlug(input string) string {
	input = strings.TrimSpace(input)
	if u, err := url.Parse(input); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		slug := seriesSlugFromPath(u.Path)
		if slug != "" {
			return slug
		}
	}
	return input
}

// seriesSlugFromPath extracts the series slug from a URL path like /series/SLUG or /series/SLUG/info.
func seriesSlugFromPath(p string) string {
	segs := strings.Split(strings.Trim(p, "/"), "/")
	for i, s := range segs {
		if s == "series" && i+1 < len(segs) {
			candidate := segs[i+1]
			if candidate != "info" && candidate != "episodes" {
				return candidate
			}
		}
	}
	return path.Base(p)
}

// mapErr converts library errors into kit error kinds with appropriate exit codes.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return errs.NotFound("%s", err.Error())
	}
	if errors.Is(err, ErrRateLimited) {
		return errs.RateLimited("%s", err.Error())
	}
	return err
}
