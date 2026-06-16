package tapas

import (
	"context"
	"encoding/xml"
	"path"
	"strings"
)

// xmlURL holds one entry from the sitemap XML.
type xmlURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// xmlURLSet is the top-level sitemap element.
type xmlURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []xmlURL `xml:"url"`
}

// FetchSitemap downloads and parses a Tapas sitemap.
// seriesType should be "comic" or "novel".
func (c *Client) FetchSitemap(ctx context.Context, seriesType string) ([]SeriesStub, error) {
	sitemapURL := c.cfg.BaseURL + "/sitemap-" + seriesType + ".xml"
	body, err := c.get(ctx, sitemapURL)
	if err != nil {
		return nil, err
	}
	return parseSitemap(body, seriesType)
}

// parseSitemap parses the sitemap XML and returns SeriesStub records.
func parseSitemap(data []byte, seriesType string) ([]SeriesStub, error) {
	var idx xmlURLSet
	if err := xml.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	out := make([]SeriesStub, 0, len(idx.URLs))
	for _, u := range idx.URLs {
		// Tapas series URLs: https://tapas.io/series/<slug>
		if !strings.Contains(u.Loc, "/series/") {
			continue
		}
		slug := path.Base(u.Loc)
		out = append(out, SeriesStub{
			Slug:    slug,
			URL:     u.Loc,
			LastMod: u.LastMod,
			Type:    seriesType,
		})
	}
	return out, nil
}

// SearchSitemap returns stubs whose slug contains the query (case-insensitive).
func SearchSitemap(stubs []SeriesStub, query string, limit int) []SeriesStub {
	q := strings.ToLower(query)
	var out []SeriesStub
	for _, s := range stubs {
		if strings.Contains(strings.ToLower(s.Slug), q) {
			out = append(out, s)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}
