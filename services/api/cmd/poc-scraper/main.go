// POC: validates the Playwright → CF cookie harvest → imroc/req Chrome TLS chain.
// Run: go run ./cmd/poc-scraper <letterboxd-handle>
// First run downloads Chromium (~170MB) — subsequent runs reuse the cache.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	playwright "github.com/playwright-community/playwright-go"
	"golang.org/x/net/html"
)

// chromeUA is pinned here and used in both the Playwright browser context and
// the req client. They must be identical — cf_clearance is bound to the UA
// that was active when the challenge was solved.
const chromeUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

func main() {
	handle := "dave" // Dave Letterboxd — large public profile, good test case
	if len(os.Args) > 1 {
		handle = os.Args[1]
	}

	log.Printf("=== POC scraper | handle: %s ===", handle)

	// Install Chromium if not already present. Idempotent, ~170MB on first run.
	log.Println("checking playwright install...")
	if err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
		Verbose:  false,
	}); err != nil {
		log.Fatalf("playwright install: %v", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("playwright run: %v", err)
	}
	defer pw.Stop() //nolint

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			// Remove the webdriver flag that CF uses to detect headless automation.
			"--disable-blink-features=AutomationControlled",
		},
	})
	if err != nil {
		log.Fatalf("launch browser: %v", err)
	}
	defer browser.Close()

	// ── Step 1: harvest CF cookies ────────────────────────────────────────────
	log.Println("harvesting CF cookies via Playwright...")
	start := time.Now()
	sess, err := harvest(browser)
	if err != nil {
		log.Fatalf("harvest: %v", err)
	}
	log.Printf("session ready (%.1fs) — expires %s", time.Since(start).Seconds(), sess.expiresAt.Format(time.RFC3339))
	log.Printf("cookies: %s", sess.cookieHeader)

	// ── Step 2: scrape page 1 with Chrome TLS impersonation ──────────────────
	log.Printf("fetching /%s/films/ ...", handle)

	// ImpersonateChrome sets utls Chrome TLS fingerprint.
	// SetUserAgent then overrides the UA to the exact one we pinned above —
	// this must match what Playwright used when solving the CF challenge.
	client := req.C().
		ImpersonateChrome().
		SetUserAgent(chromeUA).
		SetCommonHeader("Cookie", sess.cookieHeader).
		SetCommonHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		SetCommonHeader("Accept-Language", "en-US,en;q=0.9").
		SetCommonHeader("Referer", "https://letterboxd.com/")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("https://letterboxd.com/%s/films/", handle)
	resp, err := client.R().SetContext(ctx).Get(url)
	if err != nil {
		log.Fatalf("fetch: %v", err)
	}
	log.Printf("response: status=%d  size=%d bytes", resp.StatusCode, len(resp.Bytes()))
	if resp.StatusCode != 200 {
		log.Fatalf("unexpected status %d — CF bypass may have failed\nbody preview: %s",
			resp.StatusCode, resp.String()[:min(500, len(resp.String()))])
	}

	// ── Step 3: parse films from HTML ─────────────────────────────────────────
	doc, err := html.Parse(strings.NewReader(resp.String()))
	if err != nil {
		log.Fatalf("parse html: %v", err)
	}
	films := parseFilms(doc)
	log.Printf("parsed %d films from page 1", len(films))

	fmt.Println()
	for i, f := range films {
		if i >= 15 {
			fmt.Printf("  ... and %d more\n", len(films)-15)
			break
		}
		rating := "—"
		if f.rating != nil {
			rating = fmt.Sprintf("%.1f★", *f.rating)
		}
		liked := " "
		if f.liked {
			liked = "♥"
		}
		fmt.Printf("  %2d. [%s] %s  %-40s\n", i+1, liked, rating, f.slug)
	}

	fmt.Println()
	if len(films) > 0 {
		fmt.Printf("✓ POC SUCCESS — CF bypass works, %d films on page 1\n", len(films))
	} else {
		fmt.Println("✗ 0 films parsed — HTML structure may have changed, check selectors")
	}
}

// ── Cookie harvesting ─────────────────────────────────────────────────────────

type cfSession struct {
	cookieHeader string
	expiresAt    time.Time
}

func harvest(browser playwright.Browser) (*cfSession, error) {
	// Pin the UA in the browser context so CF sees a consistent UA across
	// the challenge solve and all subsequent requests.
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

	// Navigate to letterboxd.com. Playwright auto-solves the CF JS challenge.
	// NetworkIdle waits for the challenge redirect + final page load to settle.
	if _, err = page.Goto("https://letterboxd.com", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(45000),
	}); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	cookies, err := bCtx.Cookies("https://letterboxd.com")
	if err != nil {
		return nil, fmt.Errorf("get cookies: %w", err)
	}

	var parts []string
	var expiresAt time.Time
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
		if c.Name == "cf_clearance" && c.Expires > 0 {
			expiresAt = time.Unix(int64(c.Expires), 0)
			log.Printf("cf_clearance expires: %s (TTL %.0fm)",
				expiresAt.Format(time.RFC3339), time.Until(expiresAt).Minutes())
		}
	}
	if expiresAt.IsZero() {
		// CF challenge wasn't triggered (clean IP) or cookie has session TTL.
		// Assume 30min conservative default so the refresh ticker works correctly.
		expiresAt = time.Now().Add(30 * time.Minute)
		log.Println("no cf_clearance expiry found — defaulting to 30min TTL")
	}

	return &cfSession{
		cookieHeader: strings.Join(parts, "; "),
		expiresAt:    expiresAt,
	}, nil
}

// ── HTML parsing ──────────────────────────────────────────────────────────────

type rawFilm struct {
	slug   string
	rating *float32
	liked  bool
}

func parseFilms(doc *html.Node) []rawFilm {
	var films []rawFilm
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && hasClass(n, "poster-container") {
			if f, ok := parseFilmNode(n); ok {
				films = append(films, f)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return films
}

func parseFilmNode(n *html.Node) (rawFilm, bool) {
	var filmDiv *html.Node
	var find func(*html.Node)
	find = func(node *html.Node) {
		if filmDiv != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "div" && getAttr(node, "data-film-slug") != "" {
			filmDiv = node
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(n)
	if filmDiv == nil {
		return rawFilm{}, false
	}
	f := rawFilm{
		slug:  getAttr(filmDiv, "data-film-slug"),
		liked: getAttr(n, "data-owner-liked") == "1",
	}
	if s := getAttr(n, "data-owner-rating"); s != "" {
		if v, err := strconv.ParseFloat(s, 32); err == nil && v > 0 {
			r := float32(v) / 2.0
			f.rating = &r
		}
	}
	return f, f.slug != ""
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
