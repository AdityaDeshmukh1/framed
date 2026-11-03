// Command quizseed builds the film pool for the rating-quiz prototype.
//
// It pulls a few hundred widely-seen films from TMDB (global hits, top rated,
// Indian and other regional cinema, a handful of genres), enriches each one
// exactly like the pipeline does, embeds it with the same text the embed
// worker uses, and writes apps/web/public/quiz/films.json.
//
// With DATABASE_URL set it also exports the current users' taste vectors to
// apps/web/public/quiz/people.local.json so the quiz can show real match
// scores. Both files are gitignored: regenerate them with
//
//	go run ./cmd/quizseed
//
// Needs TMDB_API_KEY (+ TMDB_BASE_URL if proxied) and the ML service running.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/framed-app/api/internal/enrichment"
	"github.com/jackc/pgx/v5"
)

// list is one TMDB listing to sample from.
type list struct {
	path  string
	query string
	pages int
}

var lists = []list{
	{"/discover/movie", "sort_by=vote_count.desc", 5},                                     // what most people have seen
	{"/movie/top_rated", "", 3},                                                           // canon
	{"/discover/movie", "sort_by=vote_count.desc&with_original_language=hi", 3},           // Hindi
	{"/discover/movie", "sort_by=vote_count.desc&with_original_language=ta", 1},           // Tamil
	{"/discover/movie", "sort_by=vote_count.desc&with_original_language=te", 1},           // Telugu
	{"/discover/movie", "sort_by=vote_count.desc&with_original_language=ml", 1},           // Malayalam
	{"/discover/movie", "sort_by=vote_count.desc&with_original_language=ko", 1},           // Korean
	{"/discover/movie", "sort_by=vote_count.desc&with_original_language=ja", 1},           // Japanese
	{"/discover/movie", "sort_by=vote_count.desc&with_original_language=fr", 1},           // French
	{"/discover/movie", "sort_by=vote_count.desc&with_genres=27", 1},                      // horror
	{"/discover/movie", "sort_by=vote_count.desc&with_genres=10749", 1},                   // romance
	{"/discover/movie", "sort_by=vote_count.desc&with_genres=16", 1},                      // animation
	{"/discover/movie", "sort_by=vote_count.desc&with_genres=35", 1},                      // comedy
	{"/discover/movie", "sort_by=vote_count.desc&with_genres=99", 1},                      // documentary
	{"/discover/movie", "sort_by=vote_count.desc&primary_release_date.gte=2022-01-01", 2}, // recent
}

type listItem struct {
	ID               int    `json:"id"`
	VoteCount        int    `json:"vote_count"`
	OriginalLanguage string `json:"original_language"`
	OriginalTitle    string `json:"original_title"`
	PosterPath       string `json:"poster_path"`
	Overview         string `json:"overview"`
}

// Film is one entry in films.json.
type Film struct {
	TMDBID int    `json:"tmdbId"`
	Title  string `json:"title"`
	// OriginalTitle is set only when it differs and is in Latin script, e.g.
	// "Taare Zameen Par" for "Like Stars on Earth".
	OriginalTitle string    `json:"originalTitle,omitempty"`
	Year          int       `json:"year"`
	Directors     []string  `json:"directors"`
	Genres        []string  `json:"genres"`
	Language      string    `json:"language"`
	PosterPath    string    `json:"posterPath"`
	VoteCount     int       `json:"voteCount"`
	Embedding     []float64 `json:"embedding"`
}

// Person is one entry in people.local.json.
type Person struct {
	Handle    string    `json:"handle"`
	Films     int       `json:"films"`
	TopGenres []string  `json:"topGenres"`
	Vector    []float64 `json:"vector"`
}

func main() {
	out := flag.String("out", "../../apps/web/public/quiz", "output directory")
	minVotes := flag.Int("min-votes", 150, "skip films with fewer TMDB votes")
	flag.Parse()

	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		log.Fatal("TMDB_API_KEY is not set")
	}
	base := os.Getenv("TMDB_BASE_URL")
	if base == "" {
		base = "https://api.themoviedb.org/3"
	}
	mlURL := os.Getenv("ML_SERVICE_URL")
	if mlURL == "" {
		mlURL = "http://localhost:8001"
	}
	ctx := context.Background()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	// 1. Collect candidates from the listings.
	seen := map[int]listItem{}
	var order []int
	for _, l := range lists {
		for page := 1; page <= l.pages; page++ {
			items, err := fetchList(ctx, base, apiKey, l, page)
			if err != nil {
				log.Fatalf("list %s?%s page %d: %v", l.path, l.query, page, err)
			}
			for _, it := range items {
				if _, dup := seen[it.ID]; dup || it.VoteCount < *minVotes || it.PosterPath == "" || it.Overview == "" {
					continue
				}
				seen[it.ID] = it
				order = append(order, it.ID)
			}
		}
	}
	log.Printf("[quizseed] %d candidate films", len(order))

	// 2. Enrich + embed, exactly as the pipeline does.
	tmdb := enrichment.New(apiKey)
	var films []Film
	for i, id := range order {
		d, err := tmdb.GetFilmDetails(ctx, id)
		if err != nil {
			log.Printf("[quizseed] skip %d: %v", id, err)
			continue
		}
		// Must match handleEmbedFilm in internal/jobs/workers.go.
		text := strings.Join(d.Genres, " ") + ". " +
			strings.Join(d.Directors, " ") + ". " +
			strings.Join(d.Keywords, " ") + ". " +
			d.Synopsis
		emb, err := embed(ctx, mlURL, text)
		if err != nil {
			log.Fatalf("embed %q: %v (is the ML service running?)", d.Title, err)
		}
		films = append(films, Film{
			TMDBID:        d.TMDBId,
			Title:         d.Title,
			OriginalTitle: latinOriginal(seen[id].OriginalTitle, d.Title),
			Year:          d.Year,
			Directors:     d.Directors,
			Genres:        d.Genres,
			Language:      seen[id].OriginalLanguage,
			PosterPath:    d.PosterPath,
			VoteCount:     seen[id].VoteCount,
			Embedding:     round(emb, 4),
		})
		if (i+1)%50 == 0 {
			log.Printf("[quizseed] %d/%d", i+1, len(order))
		}
	}
	writeJSON(filepath.Join(*out, "films.json"), films)
	log.Printf("[quizseed] wrote %d films", len(films))

	// 3. Optional: current users, for real match scores.
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		people, err := exportPeople(ctx, dsn)
		if err != nil {
			log.Printf("[quizseed] skipping people export: %v", err)
			return
		}
		writeJSON(filepath.Join(*out, "people.local.json"), people)
		log.Printf("[quizseed] wrote %d people", len(people))
	}
}

func fetchList(ctx context.Context, base, apiKey string, l list, page int) ([]listItem, error) {
	q, _ := url.ParseQuery(l.query)
	q.Set("api_key", apiKey)
	q.Set("page", fmt.Sprint(page))
	q.Set("include_adult", "false")
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+l.path+"?"+q.Encode(), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var body struct {
		Results []listItem `json:"results"`
	}
	return body.Results, json.NewDecoder(resp.Body).Decode(&body)
}

func embed(ctx context.Context, mlURL, text string) ([]float64, error) {
	b, _ := json.Marshal(map[string]string{"text": text})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, mlURL+"/embed", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var body struct {
		Embedding []float64 `json:"embedding"`
	}
	return body.Embedding, json.NewDecoder(resp.Body).Decode(&body)
}

// exportPeople reads each user's taste vector, one row per Letterboxd handle
// (the most recent user wins when test data has duplicates).
func exportPeople(ctx context.Context, dsn string) ([]Person, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, `
		SELECT DISTINCT ON (j.letterboxd_handle)
		       j.letterboxd_handle, t.vector::text, t.top_genres,
		       (SELECT count(*) FROM user_ratings r WHERE r.user_id = t.user_id)
		FROM taste_vectors t
		JOIN scrape_jobs j ON j.user_id = t.user_id
		ORDER BY j.letterboxd_handle, j.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var people []Person
	for rows.Next() {
		var p Person
		var vec string
		if err := rows.Scan(&p.Handle, &vec, &p.TopGenres, &p.Films); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(vec), &p.Vector); err != nil {
			return nil, fmt.Errorf("parse vector for %s: %w", p.Handle, err)
		}
		p.Vector = round(p.Vector, 4)
		people = append(people, p)
	}
	sort.Slice(people, func(i, j int) bool { return people[i].Handle < people[j].Handle })
	return people, rows.Err()
}

func latinOriginal(original, title string) string {
	if original == "" || strings.EqualFold(original, title) {
		return ""
	}
	for _, r := range original {
		if r > unicode.MaxLatin1 && !unicode.Is(unicode.Latin, r) && unicode.IsLetter(r) {
			return ""
		}
	}
	return original
}

func round(v []float64, places int) []float64 {
	f := math.Pow(10, float64(places))
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = math.Round(x*f) / f
	}
	return out
}

func writeJSON(path string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		log.Fatal(err)
	}
}
