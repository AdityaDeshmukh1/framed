package scraper

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// RawRating is what we extract from Letterboxd before TMDB enrichment.
// It's intentionally minimal — just what the HTML gives us.
type RawRating struct {
	LetterboxdSlug string
	Title          string
	Year           int
	Rating         float32 // 0 if not rated (just logged)
	Liked          bool
	WatchedDate    *time.Time
	Rewatch        bool
}

// Scraper fetches a Letterboxd user's film history.
type Scraper struct {
	client  *http.Client
	delayMS int
	maxConc int
}

// New creates a Scraper with configurable request delay and concurrency.
// delayMS: milliseconds to wait between requests (be polite)
// maxConcurrent: max parallel page fetches
func New(delayMS, maxConcurrent int) *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		delayMS: delayMS,
		maxConc: maxConcurrent,
	}
}

// ScrapeProfile fetches all rated films for a Letterboxd user.
// Returns a slice of RawRatings and the total count found.
//
// This is the entry point the job worker calls.
func (s *Scraper) ScrapeProfile(ctx context.Context, handle string, progressFn func(found, processed int)) ([]RawRating, error) {
	// Step 1: find out how many pages there are
	firstPage, totalPages, err := s.fetchPage(ctx, handle, 1)
	if err != nil {
		return nil, fmt.Errorf("fetch first page: %w", err)
	}

	if progressFn != nil {
		progressFn(len(firstPage), len(firstPage))
	}

	if totalPages == 1 {
		return firstPage, nil
	}

	// Step 2: fetch remaining pages concurrently, bounded by maxConcurrent
	// This is the Go concurrency pattern: semaphore via buffered channel
	results := make([][]RawRating, totalPages)
	results[0] = firstPage

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, s.maxConc) // semaphore
	)

	for page := 2; page <= totalPages; page++ {
		// check context cancellation before spawning more goroutines
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		go func(p int) {
			defer wg.Done()

			// acquire semaphore slot
			sem <- struct{}{}
			defer func() { <-sem }()

			// polite delay between requests
			time.Sleep(time.Duration(s.delayMS) * time.Millisecond)

			ratings, _, err := s.fetchPage(ctx, handle, p)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("page %d: %w", p, err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			results[p-1] = ratings
			mu.Unlock()

			if progressFn != nil {
				progressFn(0, len(ratings))
			}
		}(page)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// flatten results
	var all []RawRating
	for _, page := range results {
		all = append(all, page...)
	}

	return all, nil
}

// fetchPage fetches a single page of a user's film ratings.
// Returns the ratings on that page and the total number of pages.
func (s *Scraper) fetchPage(ctx context.Context, handle string, page int) ([]RawRating, int, error) {
	url := fmt.Sprintf("https://letterboxd.com/%s/films/page/%d/", handle, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}

	// Set a real User-Agent — some sites block the default Go user-agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; framed-app/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, 0, fmt.Errorf("letterboxd user %q not found", handle)
	}
	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("parse html: %w", err)
	}

	ratings := parseFilmsFromDoc(doc)
	totalPages := parseTotalPages(doc)

	log.Printf("scraped %s page %d/%d — found %d films", handle, page, totalPages, len(ratings))

	return ratings, totalPages, nil
}

// ── HTML Parsing ──────────────────────────────────────────────────────────────
// Letterboxd's film grid: each film is an <li> with class "poster-container"
// containing data attributes with the slug, rating, and liked status.
//
// This is the part most likely to break if Letterboxd changes their markup.
// Keep it isolated here so it's easy to update.

func parseFilmsFromDoc(doc *html.Node) []RawRating {
	var ratings []RawRating
	var traverse func(*html.Node)

	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			if hasClass(n, "poster-container") {
				if r, ok := parseFilmNode(n); ok {
					ratings = append(ratings, r)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return ratings
}

func parseFilmNode(n *html.Node) (RawRating, bool) {
	// find the <div data-film-slug="..."> child
	var filmDiv *html.Node
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "div" {
			if getAttr(node, "data-film-slug") != "" {
				filmDiv = node
				return
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)

	if filmDiv == nil {
		return RawRating{}, false
	}

	slug := getAttr(filmDiv, "data-film-slug")
	if slug == "" {
		return RawRating{}, false
	}

	r := RawRating{
		LetterboxdSlug: slug,
	}

	// rating: stored as integer 1-10 (half-stars), divide by 2
	if ratingStr := getAttr(n, "data-owner-rating"); ratingStr != "" {
		if ratingInt, err := strconv.Atoi(ratingStr); err == nil && ratingInt > 0 {
			r.Rating = float32(ratingInt) / 2.0
		}
	}

	// liked: "data-owner-liked" = "1"
	r.Liked = getAttr(n, "data-owner-liked") == "1"

	return r, true
}

func parseTotalPages(doc *html.Node) int {
	// look for the last pagination link: <li class="paginate-page"><a href="...page/N/">N</a>
	var maxPage int
	var traverse func(*html.Node)

	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			if hasClass(n, "paginate-page") {
				// get the text content of the <a> inside
				if a := findFirstElement(n, "a"); a != nil {
					if text := textContent(a); text != "" {
						if page, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
							if page > maxPage {
								maxPage = page
							}
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	if maxPage == 0 {
		return 1 // single page profile
	}
	return maxPage
}

// ── HTML helpers ──────────────────────────────────────────────────────────────

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func findFirstElement(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)
	return sb.String()
}
