package tapas

// SeriesStub is a lightweight record from the sitemap - just slug and URL.
type SeriesStub struct {
	Slug    string `json:"slug"`
	URL     string `json:"url"`
	LastMod string `json:"last_modified"`
	Type    string `json:"type"` // "comic" or "novel"
}

// Series is a full series record including scraped HTML metadata.
type Series struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"         kit:"id"`
	Title       string   `json:"title"`
	Creator     string   `json:"creator"`
	Genres      []string `json:"genres"`
	Subscribers int      `json:"subscribers"`
	Type        string   `json:"type"`
	URL         string   `json:"url"          kit:"body"`
}

// Episode is one episode from the /series/<id>/episodes endpoint.
type Episode struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	EpisodeNum  int    `json:"episode_num"`
	Free        bool   `json:"free"`
	EarlyAccess bool   `json:"early_access"`
	PublishDate string `json:"publish_date"`
	Views       int    `json:"views"`
	Likes       int    `json:"likes"`
	Comments    int    `json:"comments"`
	URL         string `json:"url"`
}

// --- raw JSON types from the episodes endpoint ---

type rawEpisode struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Scene           int    `json:"scene"`
	PendingScene    int    `json:"pending_scene"`
	Free            bool   `json:"free"`
	EarlyAccess     bool   `json:"early_access"`
	PublishDate     string `json:"publish_date"`
	ViewCnt         int    `json:"view_cnt"`
	LikeCnt         int    `json:"like_cnt"`
	CommentCnt      int    `json:"comment_cnt"`
	RelPublishDate  string `json:"relative_publish_date"`
	EscapeTitle     string `json:"escape_title"`
}

type rawPagination struct {
	Page    int  `json:"page"`
	HasNext bool `json:"has_next"`
	Total   int  `json:"total"`
}

type rawEpisodesBody struct {
	Pagination rawPagination `json:"pagination"`
	Episodes   []rawEpisode  `json:"episodes"`
}

type rawEpisodesResponse struct {
	Code int             `json:"code"`
	Data rawEpisodesBody `json:"data"`
	// Also handle flat format without data wrapper
	Episodes []rawEpisode `json:"episodes"`
}

func (r rawEpisode) toEpisode(seriesID string) Episode {
	num := r.Scene
	if num == 0 {
		num = r.PendingScene
	}
	return Episode{
		ID:          r.ID,
		Title:       r.Title,
		EpisodeNum:  num,
		Free:        r.Free,
		EarlyAccess: r.EarlyAccess,
		PublishDate: r.PublishDate,
		Views:       r.ViewCnt,
		Likes:       r.LikeCnt,
		Comments:    r.CommentCnt,
		URL:         "https://tapas.io/episode/" + itoa(r.ID),
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 12)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
