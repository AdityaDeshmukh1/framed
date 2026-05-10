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
	LetterboxdSlug string  `xml:"link"`
	WatchedDate    string  `xml:"https://letterboxd.com watchedDate"`
	Rewatch        string  `xml:"https://letterboxd.com rewatch"`
	FilmTitle      string  `xml:"https://letterboxd.com filmTitle"`
	FilmYear       int     `xml:"https://letterboxd.com filmYear"`
	MemberRating   float32 `xml:"https://letterboxd.com memberRating"`
	MemberLike     string  `xml:"https://letterboxd.com memberLike"`
	TMDBMovieID    int     `xml:"https://themoviedb.org movieId"`
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
		var rating *float32
		if item.MemberRating > 0 {
			v := item.MemberRating
			rating = &v
		}

		var watchedDate *time.Time
		if item.WatchedDate != "" {
			t, err := time.Parse("2006-01-02", item.WatchedDate)
			if err == nil {
				watchedDate = &t
			}
		}

		ratings = append(ratings, RawRating{
			TMDBMovieID:    item.TMDBMovieID,
			Title:          item.FilmTitle,
			Year:           item.FilmYear,
			Rating:         rating,
			Liked:          item.MemberLike == "Yes",
			Rewatch:        item.Rewatch == "Yes",
			LetterboxdSlug: item.LetterboxdSlug,
			WatchedDate:    watchedDate,
		})
	}
	return ratings, nil
}
