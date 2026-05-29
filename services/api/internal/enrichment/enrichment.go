package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

const defaultBaseURL = "https://api.themoviedb.org/3"

func resolveBaseURL() string {
	if v := os.Getenv("TMDB_BASE_URL"); v != "" {
		return v
	}
	return defaultBaseURL
}

// RateLimitError is returned when TMDB responds with 429 Too Many Requests.
// RetryAfter is parsed from the Retry-After response header.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("TMDB rate limited: retry after %s", e.RetryAfter)
}

type TMDBClient struct {
	client  *http.Client
	apiKey  string
	baseURL string
	limiter *rate.Limiter
}

func New(apiKey string) *TMDBClient {
	return &TMDBClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// Proxy reads HTTPS_PROXY / HTTP_PROXY / NO_PROXY from the environment.
				Proxy: http.ProxyFromEnvironment,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// force IPv4 — IPv6 to CloudFront is reset by some Indian ISPs
					return (&net.Dialer{}).DialContext(ctx, "tcp4", addr)
				},
			},
		},
		apiKey:  apiKey,
		baseURL: resolveBaseURL(),
		// 30 req/s, burst 10 — well under TMDB's 40 req/s limit.
		limiter: rate.NewLimiter(rate.Limit(30), 10),
	}
}

// rateLimitFromResponse parses a 429 response and returns a RateLimitError.
// Falls back to 5s if the Retry-After header is absent or unparseable.
func rateLimitFromResponse(resp *http.Response) *RateLimitError {
	retryAfter := 5 * time.Second
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			retryAfter = time.Duration(secs) * time.Second
		}
	}
	return &RateLimitError{RetryAfter: retryAfter}
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
	if err := t.limiter.Wait(ctx); err != nil {
		return 0, fmt.Errorf("rate limiter: %w", err)
	}

	// url.QueryEscape handles titles with spaces, special characters
	// "Mulholland Drive" → "Mulholland+Drive"
	escapedTitle := url.QueryEscape(title)

	endpoint := fmt.Sprintf("%s/search/movie?api_key=%s&query=%s&year=%d",
		t.baseURL, t.apiKey, escapedTitle, year,
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

	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, rateLimitFromResponse(resp)
	}
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
	if err := t.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	endpoint := fmt.Sprintf("%s/movie/%d?api_key=%s&append_to_response=keywords,credits",
		t.baseURL, tmdbID, t.apiKey,
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

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimitFromResponse(resp)
	}
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
		// Initialize to empty slices, not nil — pgx sends nil as SQL NULL
		// which violates the NOT NULL DEFAULT '{}' constraints on these columns.
		Directors: []string{},
		Genres:    []string{},
		Keywords:  []string{},
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
