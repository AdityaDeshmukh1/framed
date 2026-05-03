package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

const baseURL = "https://api.themoviedb.org/3"

type TMDBClient struct {
	client *http.Client
	apiKey string
}

func New(apiKey string) *TMDBClient {
	return &TMDBClient{
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// force IPv4 — IPv6 connections to CloudFront are
					// reset by some ISPs (particularly in India)
					return (&net.Dialer{}).DialContext(ctx, "tcp4", addr)
				},
			},
		},
		apiKey: apiKey,
	}
}

// tmdbSearchResult maps the TMDB search endpoint response
type tmdbSearchResult struct {
	Results []struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		ReleaseDate string  `json:"release_date"`
		Overview    string  `json:"overview"`
		Popularity  float64 `json:"popularity"`
	} `json:"results"`
}

// tmdbFilmDetail maps the TMDB movie detail endpoint response
// This is the full metadata we store in the films table
type tmdbFilmDetail struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	ReleaseDate string  `json:"release_date"`
	Runtime     int     `json:"runtime"`
	Popularity  float64 `json:"popularity"`
	PosterPath  string  `json:"poster_path"`
	Genres      []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCountries []struct {
		Code string `json:"iso_3166_1"`
		Name string `json:"name"`
	} `json:"production_countries"`
	SpokenLanguages []struct {
		Code string `json:"iso_639_1"`
		Name string `json:"name"`
	} `json:"spoken_languages"`
	Keywords struct {
		Keywords []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"keywords"`
	} `json:"keywords"`
	Credits struct {
		Crew []struct {
			Job  string `json:"job"`
			Name string `json:"name"`
		} `json:"crew"`
	} `json:"credits"`
	VoteAverage float64 `json:"vote_average"`
}

// FilmData is the cleaned, application-ready struct we return from enrichment.
// This is what gets written to the films table.
// Notice we translate TMDB's raw shape into our domain language here —
// the rest of the application never needs to know about TMDB's API shape.
type FilmData struct {
	TMDBId     int
	Title      string
	Year       int
	Synopsis   string
	Directors  []string
	Genres     []string
	Keywords   []string
	Runtime    int
	Language   string
	Country    string
	PosterPath string
	Popularity float64
	AvgRating  float64
}

// SearchFilm finds a film on TMDB by title and year, returns its TMDB ID.
// You wrote this — it lives here now.
func (t *TMDBClient) SearchFilm(ctx context.Context, title string, year int) (int, error) {
	// url.QueryEscape handles titles with spaces, special characters
	// "Mulholland Drive" → "Mulholland+Drive"
	escapedTitle := url.QueryEscape(title)

	endpoint := fmt.Sprintf("%s/search/movie?api_key=%s&query=%s&year=%d",
		baseURL, t.apiKey, escapedTitle, year,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result tmdbSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Results) == 0 {
		return 0, fmt.Errorf("no results found for %s (%d)", title, year)
	}

	return result.Results[0].ID, nil
}

// GetFilmDetails fetches full metadata for a film by TMDB ID.
// We append_to_response to get keywords and credits in one request
// instead of three — saves API quota.
func (t *TMDBClient) GetFilmDetails(ctx context.Context, tmdbID int) (*FilmData, error) {
	endpoint := fmt.Sprintf("%s/movie/%d?api_key=%s&append_to_response=keywords,credits",
		baseURL, tmdbID, t.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("film %d not found on TMDB", tmdbID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var detail tmdbFilmDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return toFilmData(&detail), nil
}

// toFilmData translates the raw TMDB response into our domain struct.
// Keeping this translation in one place means if TMDB changes their API,
// we fix it here and nowhere else — this is the boundary between
// their world and ours.
func toFilmData(d *tmdbFilmDetail) *FilmData {
	film := &FilmData{
		TMDBId:     d.ID,
		Title:      d.Title,
		Synopsis:   d.Overview,
		Runtime:    d.Runtime,
		PosterPath: d.PosterPath,
		Popularity: d.Popularity,
		AvgRating:  d.VoteAverage,
	}

	// extract year from "2001-09-08" → 2001
	if len(d.ReleaseDate) >= 4 {
		year := 0
		fmt.Sscanf(d.ReleaseDate[:4], "%d", &year)
		film.Year = year
	}

	// extract genre names
	for _, g := range d.Genres {
		film.Genres = append(film.Genres, g.Name)
	}

	// extract keyword names (themes)
	for _, k := range d.Keywords.Keywords {
		film.Keywords = append(film.Keywords, k.Name)
	}

	// extract directors from crew
	for _, c := range d.Credits.Crew {
		if c.Job == "Director" {
			film.Directors = append(film.Directors, c.Name)
		}
	}

	// primary language
	if len(d.SpokenLanguages) > 0 {
		film.Language = d.SpokenLanguages[0].Code
	}

	// primary country
	if len(d.ProductionCountries) > 0 {
		film.Country = d.ProductionCountries[0].Code
	}

	return film
}
