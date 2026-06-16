package tapas

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Match: <p class="title">MATCHPOINT</p>
	titleRE = regexp.MustCompile(`class="title">([^<]+)<`)

	// Match: series_id=329873 in href attributes
	seriesIDRE = regexp.MustCompile(`series_id=(\d+)`)

	// Match: href="/loonytwin" in creator__item link
	creatorRE = regexp.MustCompile(`class="creator__item[^"]*"[^>]*href="/([^/"]+)"`)

	// Match: genre_name=Romance in href
	genreRE = regexp.MustCompile(`genre_name=([^"&]+)`)

	// Match: data-title="3,843 subscribers"
	subscriberRE = regexp.MustCompile(`data-title="([\d,]+)\s+subscribers?"`)
)

// GetSeries fetches and parses a series info page by slug.
func (c *Client) GetSeries(ctx context.Context, slug string) (*Series, error) {
	rawURL := fmt.Sprintf("%s/series/%s/info", c.cfg.BaseURL, slug)
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return parseSeriesHTML(body, slug, rawURL), nil
}

// parseSeriesHTML extracts series metadata from an HTML info page.
func parseSeriesHTML(body []byte, slug, rawURL string) *Series {
	html := string(body)

	// Extract series ID from any series_id= parameter
	seriesID := ""
	if m := seriesIDRE.FindStringSubmatch(html); len(m) > 1 {
		seriesID = m[1]
	}
	if seriesID == "" {
		// If the slug is numeric, it is the ID
		if _, err := strconv.Atoi(slug); err == nil {
			seriesID = slug
		}
	}

	// Extract title: first class="title"> occurrence after the series header
	title := ""
	if m := titleRE.FindStringSubmatch(html); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}
	if title == "" {
		title = slug
	}

	// Extract creator username
	creator := ""
	if m := creatorRE.FindStringSubmatch(html); len(m) > 1 {
		creator = m[1]
	}

	// Extract genres (all unique genre_name= values)
	genres := []string{}
	seen := map[string]bool{}
	for _, m := range genreRE.FindAllStringSubmatch(html, -1) {
		if len(m) > 1 {
			g := m[1]
			if !seen[g] {
				seen[g] = true
				genres = append(genres, g)
			}
		}
	}

	// Extract subscriber count
	subscribers := 0
	if m := subscriberRE.FindStringSubmatch(html); len(m) > 1 {
		s := strings.ReplaceAll(m[1], ",", "")
		subscribers, _ = strconv.Atoi(s)
	}

	// Determine series type from URL or content
	seriesType := "comic"
	if strings.Contains(rawURL, "novel") || strings.Contains(html, "Novel") {
		seriesType = "novel"
	}

	infoURL := fmt.Sprintf("https://tapas.io/series/%s/info", slug)

	return &Series{
		ID:          seriesID,
		Slug:        slug,
		Title:       title,
		Creator:     creator,
		Genres:      genres,
		Subscribers: subscribers,
		Type:        seriesType,
		URL:         infoURL,
	}
}
