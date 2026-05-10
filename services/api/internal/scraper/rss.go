package scraper

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"time"
)

type RSSImporter struct {
	client *http.Client
}

type rssItem struct {
	Title          string  `xml:"title"`
	WatchedDate    string  `xml:"watchedDate"`
	Rewatch        string  `xml:"rewatch"`
	FilmTitle      string  `xml:"filmTitle"`
	FilmYear       int     `xml:"filmYear"`
	MemberRating   float32 `xml:"memberRating"`
	TMDBMovieID    int     `xml:"movieId"`
	MemberLike     string  `xml:"memberLike"`
	LetterboxdSlug string  `xml:"link"`
}

type rssFeed struct {
	Items []rssItem `xml:"channel>item"`
}

func NewRSSImporter() *RSSImporter {
	return &RSSImporter{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (r *RSSImporter) ImportProfile(ctx context.Context, handle string) ([]RawRating, error) {
	url := fmt.Sprintf("https://letterboxd.com/%s/rss/", handle)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	// Set a real User-Agent — some sites block the default Go user-agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; framed-app/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	var feed rssFeed
	err = xml.NewDecoder(resp.Body).Decode(&feed)
	if err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	var ratings []RawRating
	for _, item := range feed.Items {
		ratings = append(ratings, RawRating{
			Title:          item.FilmTitle,
			Year:           item.FilmYear,
			Rating:         item.MemberRating,
			Liked:          item.MemberLike == "Yes",
			Rewatch:        item.Rewatch == "Yes",
			LetterboxdSlug: item.LetterboxdSlug,
		})
	}
	return ratings, nil
}
