package tapas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// GetEpisodes fetches episodes for a series.
// seriesID must be the numeric ID (not slug).
func (c *Client) GetEpisodes(ctx context.Context, seriesID string, limit int) ([]Episode, error) {
	if limit <= 0 {
		limit = 20
	}

	var out []Episode
	page := 1
	for len(out) < limit {
		batch, hasNext, err := c.episodesPage(ctx, seriesID, page, limit)
		if err != nil {
			return out, err
		}
		out = append(out, batch...)
		if !hasNext || len(batch) == 0 {
			break
		}
		page++
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (c *Client) episodesPage(ctx context.Context, seriesID string, page, limit int) ([]Episode, bool, error) {
	u, err := url.Parse(fmt.Sprintf("%s/series/%s/episodes", c.cfg.BaseURL, seriesID))
	if err != nil {
		return nil, false, err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("max_limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	body, err := c.get(ctx, u.String())
	if err != nil {
		return nil, false, err
	}

	raw, pagination, err := extractEpisodesJSON(body)
	if err != nil {
		return nil, false, err
	}

	episodes := make([]Episode, len(raw))
	for i, r := range raw {
		episodes[i] = r.toEpisode(seriesID)
	}
	return episodes, pagination.HasNext, nil
}

// extractEpisodesJSON finds and parses the episodes JSON embedded in the HTML body.
// The Tapas response is an HTML page with JSON embedded at the end like:
// {..., "episodes":[{...},...], ...}
func extractEpisodesJSON(body []byte) ([]rawEpisode, rawPagination, error) {
	// Strategy 1: Look for the pattern from the full response JSON object
	// The response has "episodes":[...] inside a JSON object
	const marker = `"episodes":[`
	idx := bytes.Index(body, []byte(marker))
	if idx < 0 {
		return nil, rawPagination{}, ErrNotFound
	}

	// Walk backwards from the marker to find the opening { of the enclosing object
	start := -1
	for i := idx - 1; i >= 0; i-- {
		if body[i] == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, rawPagination{}, fmt.Errorf("could not locate JSON start")
	}

	// Try to parse the object from 'start' forward, finding the first valid complete JSON
	raw := body[start:]

	// The JSON might be followed by more HTML; find balanced JSON end
	jsonBytes := balancedJSON(raw)
	if jsonBytes == nil {
		return nil, rawPagination{}, fmt.Errorf("could not extract balanced JSON")
	}

	// Try the nested format with "data" wrapper first
	var nested struct {
		Code int `json:"code"`
		Data struct {
			Pagination rawPagination `json:"pagination"`
			Episodes   []rawEpisode  `json:"episodes"`
		} `json:"data"`
		Episodes   []rawEpisode  `json:"episodes"`
		Pagination rawPagination `json:"pagination"`
	}

	if err := json.Unmarshal(jsonBytes, &nested); err != nil {
		// Fall back: try to parse just the episodes array directly
		return nil, rawPagination{}, fmt.Errorf("parse episodes JSON: %w", err)
	}

	// Prefer the data-wrapped format
	if len(nested.Data.Episodes) > 0 {
		return nested.Data.Episodes, nested.Data.Pagination, nil
	}
	// Fall back to flat format
	if len(nested.Episodes) > 0 {
		return nested.Episodes, nested.Pagination, nil
	}

	// Try yet another format - the JSON may be the flat response object
	var flat struct {
		Code int          `json:"code"`
		Data struct {
			Pagination rawPagination `json:"pagination"`
			Body       string        `json:"body"`
			Episodes   []rawEpisode  `json:"episodes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(jsonBytes, &flat); err == nil && len(flat.Data.Episodes) > 0 {
		return flat.Data.Episodes, flat.Data.Pagination, nil
	}

	return nil, rawPagination{}, nil
}

// balancedJSON extracts the first complete JSON object from b.
func balancedJSON(b []byte) []byte {
	if len(b) == 0 || b[0] != '{' {
		return nil
	}
	depth := 0
	inStr := false
	escaped := false
	for i, c := range b {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inStr {
			escaped = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return b[:i+1]
			}
		}
	}
	return nil
}
