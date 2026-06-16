package tapas

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient returns a Client pointed at the test server with no pacing.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	cfg.Timeout = 5 * time.Second
	return NewClient(cfg), srv
}

// --- basic HTTP client tests ---

func TestGet(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	})
	defer srv.Close()

	body, err := c.get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	handler := func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	cfg.Timeout = 10 * time.Second
	c := NewClient(cfg)

	start := time.Now()
	body, err := c.get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetNotFound(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	_, err := c.get(context.Background(), srv.URL)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- sitemap tests ---

func fixtureSitemap(seriesType string, slugs ...string) []byte {
	type xmlURLEntry struct {
		Loc string `xml:"loc"`
	}
	type xmlURLSet struct {
		XMLName xml.Name      `xml:"urlset"`
		URLs    []xmlURLEntry `xml:"url"`
	}
	us := xmlURLSet{}
	for _, slug := range slugs {
		us.URLs = append(us.URLs, xmlURLEntry{
			Loc: fmt.Sprintf("https://tapas.io/series/%s", slug),
		})
	}
	b, _ := xml.Marshal(us)
	return b
}

func TestFetchSitemap(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sitemap-comic.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(fixtureSitemap("comic", "MATCHPOINT", "LORE-OLYMPUS"))
	})
	defer srv.Close()

	stubs, err := c.FetchSitemap(context.Background(), "comic")
	if err != nil {
		t.Fatal(err)
	}
	if len(stubs) != 2 {
		t.Fatalf("want 2 stubs, got %d", len(stubs))
	}
	if stubs[0].Slug != "MATCHPOINT" {
		t.Errorf("stubs[0].Slug = %q, want MATCHPOINT", stubs[0].Slug)
	}
	if stubs[0].Type != "comic" {
		t.Errorf("stubs[0].Type = %q, want comic", stubs[0].Type)
	}
}

func TestSearchSitemap(t *testing.T) {
	stubs := []SeriesStub{
		{Slug: "MATCHPOINT", Type: "comic"},
		{Slug: "LORE-OLYMPUS", Type: "comic"},
		{Slug: "romance-novel", Type: "novel"},
	}
	got := SearchSitemap(stubs, "match", 10)
	if len(got) != 1 || got[0].Slug != "MATCHPOINT" {
		t.Errorf("SearchSitemap = %v, want [MATCHPOINT]", got)
	}
}

// --- series HTML tests ---

func fixtureSeriesHTML(slug, title, creator string, subscribers int) string {
	return fmt.Sprintf(`<html><body>
<div class="series-header">
  <p class="title">%s</p>
  <a class="creator__item" href="/%s">%s</a>
  <a href="?genre_name=Romance">Romance</a>
  <a href="?genre_name=Drama">Drama</a>
  <span data-title="%d subscribers"></span>
  <a href="?series_id=329873">Episodes</a>
</div>
</body></html>`, title, creator, creator, subscribers)
}

func TestGetSeries(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/series/MATCHPOINT/info" {
			_, _ = fmt.Fprint(w, fixtureSeriesHTML("MATCHPOINT", "MATCHPOINT", "loonytwin", 3843))
			return
		}
		http.NotFound(w, r)
	})
	defer srv.Close()

	s, err := c.GetSeries(context.Background(), "MATCHPOINT")
	if err != nil {
		t.Fatal(err)
	}
	if s.Slug != "MATCHPOINT" {
		t.Errorf("Slug = %q, want MATCHPOINT", s.Slug)
	}
	if s.Title != "MATCHPOINT" {
		t.Errorf("Title = %q, want MATCHPOINT", s.Title)
	}
	if s.Creator != "loonytwin" {
		t.Errorf("Creator = %q, want loonytwin", s.Creator)
	}
	if s.Subscribers != 3843 {
		t.Errorf("Subscribers = %d, want 3843", s.Subscribers)
	}
	if len(s.Genres) != 2 {
		t.Errorf("Genres = %v, want 2 genres", s.Genres)
	}
	if s.ID != "329873" {
		t.Errorf("ID = %q, want 329873", s.ID)
	}
}

func TestGetSeriesNotFound(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	_, err := c.GetSeries(context.Background(), "NONEXISTENT")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- episodes JSON tests ---

func fixtureEpisodesResponse(episodes []rawEpisode, hasNext bool) []byte {
	resp := map[string]interface{}{
		"code": 200,
		"data": map[string]interface{}{
			"pagination": map[string]interface{}{
				"page":     1,
				"has_next": hasNext,
				"total":    len(episodes),
			},
			"episodes": episodes,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestGetEpisodes(t *testing.T) {
	eps := []rawEpisode{
		{ID: 1001, Title: "Episode 1", Scene: 1, Free: true, ViewCnt: 5000},
		{ID: 1002, Title: "Episode 2", Scene: 2, Free: false, ViewCnt: 3000},
	}
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/series/329873/episodes" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixtureEpisodesResponse(eps, false))
			return
		}
		http.NotFound(w, r)
	})
	defer srv.Close()

	got, err := c.GetEpisodes(context.Background(), "329873", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 episodes, got %d", len(got))
	}
	if got[0].Title != "Episode 1" {
		t.Errorf("got[0].Title = %q, want Episode 1", got[0].Title)
	}
	if got[0].Free != true {
		t.Errorf("got[0].Free = %v, want true", got[0].Free)
	}
	if got[0].Views != 5000 {
		t.Errorf("got[0].Views = %d, want 5000", got[0].Views)
	}
	if got[0].URL != "https://tapas.io/episode/1001" {
		t.Errorf("got[0].URL = %q, want episode/1001", got[0].URL)
	}
}

func TestGetEpisodesLimitRespected(t *testing.T) {
	eps := make([]rawEpisode, 5)
	for i := range eps {
		eps[i] = rawEpisode{ID: 1000 + i, Title: fmt.Sprintf("Episode %d", i+1), Scene: i + 1}
	}
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixtureEpisodesResponse(eps, false))
	})
	defer srv.Close()

	got, err := c.GetEpisodes(context.Background(), "329873", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 episodes (limit), got %d", len(got))
	}
}

func TestExtractEpisodesJSON(t *testing.T) {
	eps := []rawEpisode{{ID: 42, Title: "Test", Scene: 1}}
	body := fixtureEpisodesResponse(eps, false)

	got, pagination, err := extractEpisodesJSON(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 42 {
		t.Errorf("got = %v, want [{ID:42}]", got)
	}
	if pagination.HasNext {
		t.Error("HasNext should be false")
	}
}
