package importer

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LetterboxdRSS imports from https://letterboxd.com/<handle>/rss/, the
// official public feed. It carries ratings, likes, watch dates and TMDB IDs,
// but only for the most recent ~50 diary entries.
type LetterboxdRSS struct {
	client  *http.Client
	baseURL string
}

func NewLetterboxdRSS() *LetterboxdRSS {
	return &LetterboxdRSS{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: "https://letterboxd.com",
	}
}

func (r *LetterboxdRSS) Source() Source { return SourceLetterboxdRSS }

type rssItem struct {
	Link         string  `xml:"link"`
	WatchedDate  string  `xml:"https://letterboxd.com watchedDate"`
	Rewatch      string  `xml:"https://letterboxd.com rewatch"`
	FilmTitle    string  `xml:"https://letterboxd.com filmTitle"`
	FilmYear     int     `xml:"https://letterboxd.com filmYear"`
	MemberRating float32 `xml:"https://letterboxd.com memberRating"`
	MemberLike   string  `xml:"https://letterboxd.com memberLike"`
	TMDBMovieID  int     `xml:"https://themoviedb.org movieId"`
}

type rssFeed struct {
	Items []rssItem `xml:"channel>item"`
}

func (r *LetterboxdRSS) Import(ctx context.Context, req Request) ([]Film, error) {
	url := fmt.Sprintf("%s/%s/rss/", r.baseURL, req.Handle)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; framed-app/1.0)")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("letterboxd user %q not found", req.Handle)
	}
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	var feed rssFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}
	return parseRSSItems(feed.Items), nil
}

// parseRSSItems converts feed items to Films. Non-film items (lists, reviews
// of lists) are skipped, and a film logged several times keeps only its most
// recent entry — the feed is newest-first.
func parseRSSItems(items []rssItem) []Film {
	seen := make(map[string]bool)
	var films []Film
	for _, item := range items {
		if item.FilmTitle == "" {
			continue
		}
		slug := slugFromFilmURL(item.Link)
		key := slug
		if key == "" {
			key = fmt.Sprintf("%s|%d", item.FilmTitle, item.FilmYear)
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		var rating *float32
		if item.MemberRating > 0 {
			v := item.MemberRating
			rating = &v
		}
		var watched *time.Time
		if t, err := time.Parse("2006-01-02", item.WatchedDate); err == nil {
			watched = &t
		}

		films = append(films, Film{
			TMDBMovieID:    item.TMDBMovieID,
			LetterboxdSlug: slug,
			Title:          item.FilmTitle,
			Year:           item.FilmYear,
			Rating:         rating,
			Liked:          item.MemberLike == "Yes",
			Rewatch:        item.Rewatch == "Yes",
			WatchedDate:    watched,
		})
	}
	return films
}

// slugFromFilmURL extracts "mulholland-drive" from diary links such as
// https://letterboxd.com/user/film/mulholland-drive/ or .../film/mulholland-drive/1/.
func slugFromFilmURL(link string) string {
	parts := strings.Split(strings.Trim(link, "/"), "/")
	for i, p := range parts {
		if p == "film" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
