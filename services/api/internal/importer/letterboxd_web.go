package scraper

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	playwright "github.com/playwright-community/playwright-go"
	"golang.org/x/net/html"
)

const chromeUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// yearSuffix matches " (YYYY)" at the end of a Letterboxd data-item-name value.
var yearSuffix = regexp.MustCompile(` \((\d{4})\)$`)

// InstallBrowser downloads the Chromium binaries required by Playwright.
// Idempotent — skips the download if already present. Call once at startup.
func InstallBrowser() error {
	return playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
		Verbose:  false,
	})
}

// PlaywrightScraper fetches a Letterboxd user's complete film history using
// a headless Chromium browser to bypass Cloudflare Bot Management.
//
// CF session isolation rule: ALL page fetches within one profile scrape must
// use the same Playwright browser page object. Switching to a raw HTTP client
// mid-session contaminates the CF session and triggers Turnstile re-challenges.
type PlaywrightScraper struct{}

func NewPlaywrightScraper() *PlaywrightScraper { return &PlaywrightScraper{} }

// ImportProfile scrapes every page of handle's Letterboxd film list and
// returns one RawRating per film. TMDBMovieID is always 0 — the caller
// (workers.go) resolves it via TMDB search using the returned Title+Year.
func (s *PlaywrightScraper) ImportProfile(_ context.Context, handle string) ([]RawRating, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright: %w", err)
	}
	defer pw.Stop() //nolint:errcheck

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args:     []string{"--disable-blink-features=AutomationControlled"},
	})
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}
	defer browser.Close()

	bCtx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(chromeUA),
	})
	if err != nil {
		return nil, fmt.Errorf("new context: %w", err)
	}
	defer bCtx.Close()

	page, err := bCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("new page: %w", err)
	}

	// Solve the CF challenge by visiting the homepage first.
	// WaitUntilStateDomcontentloaded fires on the challenge page itself;
	// .site-header appearing confirms the real Letterboxd site loaded.
	log.Printf("[scraper] handle=%s establishing CF session...", handle)
	if _, err = page.Goto("https://letterboxd.com", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(120000),
	}); err != nil {
		return nil, fmt.Errorf("navigate to letterboxd.com: %w", err)
	}
	if _, err = page.WaitForSelector(".site-header", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(60000),
	}); err != nil {
		return nil, fmt.Errorf("CF challenge did not resolve (site-header never appeared): %w", err)
	}

	// Scrape page 1, then any additional pages, all within the same browser page.
	page1URL := fmt.Sprintf("https://letterboxd.com/%s/films/", handle)
	rawFilms, totalPages, err := scrapePage(page, page1URL)
	if err != nil {
		return nil, fmt.Errorf("page 1: %w", err)
	}
	log.Printf("[scraper] handle=%s page 1/%d: %d films", handle, totalPages, len(rawFilms))

	for p := 2; p <= totalPages; p++ {
		pageURL := fmt.Sprintf("https://letterboxd.com/%s/films/page/%d/", handle, p)
		films, _, err := scrapePage(page, pageURL)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", p, err)
		}
		log.Printf("[scraper] handle=%s page %d/%d: %d films", handle, p, totalPages, len(films))
		rawFilms = append(rawFilms, films...)
	}

	log.Printf("[scraper] handle=%s done: %d films across %d pages", handle, len(rawFilms), totalPages)

	ratings := make([]RawRating, 0, len(rawFilms))
	for _, f := range rawFilms {
		title, year := parseItemName(f.itemName)
		ratings = append(ratings, RawRating{
			LetterboxdSlug: f.slug,
			Title:          title,
			Year:           year,
			Rating:         f.rating,
			Liked:          f.liked,
			// TMDBMovieID intentionally left 0 — workers resolves it via SearchFilm.
		})
	}
	return ratings, nil
}

func scrapePage(page playwright.Page, url string) ([]rawFilm, int, error) {
	if _, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return nil, 0, fmt.Errorf("navigate: %w", err)
	}
	if _, err := page.WaitForSelector(".griditem", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(20000),
	}); err != nil {
		return nil, 0, fmt.Errorf("film grid not found (CF challenge?): %w", err)
	}

	content, err := page.Content()
	if err != nil {
		return nil, 0, fmt.Errorf("get content: %w", err)
	}
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, 0, fmt.Errorf("parse html: %w", err)
	}

	return parseFilms(doc), parseTotalPages(doc), nil
}

// parseItemName splits "Film Title (2025)" into ("Film Title", 2025).
func parseItemName(s string) (title string, year int) {
	if m := yearSuffix.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		return strings.TrimSuffix(s, m[0]), y
	}
	return s, 0
}
