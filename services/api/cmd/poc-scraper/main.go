// POC: validates the Playwright → CF bypass → multi-page Letterboxd scrape chain.
//
// # Validated findings (all confirmed end-to-end)
//
//  1. CF bypass works: Playwright with --disable-blink-features=AutomationControlled
//     solves the CF JS challenge and yields a valid cf_clearance cookie (~4s on a
//     trusted IP, ~50s on first contact). No cf_clearance on clean IPs (CF skips
//     the challenge entirely and issues only session cookies).
//
//  2. HTML parsing works: Letterboxd changed structure. Old → new:
//     - li.poster-container → li.griditem
//     - div[data-film-slug] → div.react-component[data-item-slug]
//     - data-owner-rating on <li> → span.rated-N inside p.poster-viewingdata
//     - data-owner-liked on <li> → presence of span.icon-liked inside p.poster-viewingdata
//     All fields parsed correctly (72/72 films on page 1, 47/47 on page 2).
//
//  3. Pagination detection works: li.paginate-page structure is unchanged.
//
//  4. Multi-page scraping works: 119 total films across 2 pages in a single run.
//     Key constraint: ALL page fetches must stay within the same Playwright browser
//     page — harvest letterboxd.com, then navigate page 1, page 2, ..., close.
//
// # Critical constraint: CF session isolation
//
// CF Bot Management triggers Turnstile (managed mode) on page 2+ when the same
// cf_clearance is used across different TLS clients (e.g. req for page 1, Playwright
// for page 2). This contaminates the session and causes persistent re-challenges.
//
// Rule: never mix req and Playwright for the same user's film pages. Use a single
// unbroken Playwright page object for the entire profile scrape.
//
// imroc/req + Chrome TLS impersonation remains valid for single-request endpoints
// (e.g. /csi/ stats) where there is no multi-page session to contaminate.
//
// # Run modes
//
//	go run ./cmd/poc-scraper <handle>              # full run: harvest + scrape
//	LB_COOKIES="cf_clearance=..." go run ./cmd/poc-scraper <handle>  # inject existing cookies
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	playwright "github.com/playwright-community/playwright-go"
	"golang.org/x/net/html"
)

// chromeUA is pinned and used in all Playwright contexts. cf_clearance is bound
// to the UA that was active when the CF challenge was solved, so it must be
// identical across every context that reuses the same clearance cookie.
const chromeUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

func main() {
	handle := "dave" // Dave Letterboxd — large public profile, good test case
	if len(os.Args) > 1 {
		handle = os.Args[1]
	}

	log.Printf("=== POC scraper | handle: %s ===", handle)

	// ── Step 1: get CF session ────────────────────────────────────────────────
	//
	// Two modes:
	//   a) LB_COOKIES set: inject cookies into a fresh Playwright context (no
	//      CF challenge needed, cookies already valid). This lets us test the
	//      multi-page navigation approach without triggering harvest.
	//   b) No LB_COOKIES: run the full Playwright CF challenge harvest.
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
			"--disable-blink-features=AutomationControlled",
		},
	})
	if err != nil {
		log.Fatalf("launch browser: %v", err)
	}
	defer browser.Close()

	var sess *cfSession
	if raw := os.Getenv("LB_COOKIES"); raw != "" {
		log.Println("LB_COOKIES set — injecting into Playwright context (skipping CF challenge)")
		var injectErr error
		sess, injectErr = injectCookies(browser, raw)
		if injectErr != nil {
			log.Fatalf("inject cookies: %v", injectErr)
		}
	} else {
		log.Println("harvesting CF cookies via Playwright...")
		start := time.Now()
		var harvestErr error
		sess, harvestErr = harvest(browser)
		if harvestErr != nil {
			log.Fatalf("harvest: %v", harvestErr)
		}
		log.Printf("session ready (%.1fs) — expires %s", time.Since(start).Seconds(), sess.expiresAt.Format(time.RFC3339))
	}
	// Context stays open for multi-page navigation — close when main returns.
	defer sess.bCtx.Close()
	log.Printf("cookies: %s", sess.cookieHeader)

	// ── Steps 2-4: scrape all pages via Playwright (pure browser session) ──────
	//
	// We do NOT use req for any film page fetches. Using req for page 1 then
	// Playwright for page 2 cross-contaminates the CF session (two different TLS
	// clients for the same cf_clearance → Turnstile escalation).
	//
	// Pure Playwright path: navigate all pages within one browser page object.
	// CF sees a single unbroken browser session → no re-challenge.
	//
	// req is still used for single-request endpoints (e.g. /csi/ stats) where
	// there is no multi-page session to contaminate.

	page1URL := fmt.Sprintf("https://letterboxd.com/%s/films/", handle)
	log.Printf("loading page 1 via Playwright: %s", page1URL)

	page1Films, err := fetchPageViaPlaywright(sess.navPage, page1URL)
	if err != nil {
		log.Fatalf("page 1: %v", err)
	}

	// Also get the total page count from what Playwright rendered.
	page1Content, _ := sess.navPage.Content()
	page1Doc, _ := html.Parse(strings.NewReader(page1Content))
	totalPages := parseTotalPages(page1Doc)

	raw := page1Content
	log.Printf("griditem occurrences in HTML:        %d", strings.Count(raw, "griditem"))
	log.Printf("data-item-slug occurrences in HTML:  %d", strings.Count(raw, "data-item-slug"))
	log.Printf("poster-viewingdata occurrences:      %d", strings.Count(raw, "poster-viewingdata"))
	log.Printf("parsed %d films from page 1 | total pages: %d", len(page1Films), totalPages)

	fmt.Println()
	for i, f := range page1Films {
		if i >= 15 {
			fmt.Printf("  ... and %d more\n", len(page1Films)-15)
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

	if len(page1Films) == 0 {
		_ = os.WriteFile("/tmp/letterboxd-debug.html", []byte(raw), 0o644)
		log.Println("HTML saved to /tmp/letterboxd-debug.html — inspect for correct selectors")
		fmt.Println("✗ 0 films parsed — HTML structure may have changed, check selectors")
		return
	}
	fmt.Printf("✓ page 1: %d films (Playwright)\n", len(page1Films))

	allFilms := append([]rawFilm{}, page1Films...)

	for p := 2; p <= totalPages; p++ {
		pageURL := fmt.Sprintf("https://letterboxd.com/%s/films/page/%d/", handle, p)
		log.Printf("  page %d/%d: %s", p, totalPages, pageURL)
		pageFilms, err := fetchPageViaPlaywright(sess.navPage, pageURL)
		if err != nil {
			log.Fatalf("page %d failed: %v", p, err)
		}
		fmt.Printf("  ✓ page %d: %d films\n", p, len(pageFilms))
		allFilms = append(allFilms, pageFilms...)
	}

	fmt.Println()
	fmt.Printf("=== RESULT: %d total films across %d pages ===\n", len(allFilms), totalPages)
	if totalPages > 1 {
		fmt.Println("✓ POC VALIDATED — full multi-page scrape via single Playwright session")
	} else {
		fmt.Println("✓ POC VALIDATED — single-page profile scraped")
	}
}

// ── Cookie harvesting ─────────────────────────────────────────────────────────

type cfSession struct {
	cookieHeader string
	expiresAt    time.Time
	// bCtx and navPage are the live Playwright context and its single kept-alive
	// page. Non-nil when Playwright is running; nil in LB_COOKIES mode.
	// Reusing the same page for all navigation avoids CF re-challenging on new tabs.
	// The caller must Close() bCtx (which also closes navPage).
	bCtx    playwright.BrowserContext
	navPage playwright.Page
}

// harvest navigates to letterboxd.com, waits for the CF challenge to resolve,
// extracts all cookies, and returns the session. The browser context is kept
// alive (not closed here) so the caller can reuse it for page navigation —
// CF does not re-challenge within the same context.
func harvest(browser playwright.Browser) (*cfSession, error) {
	bCtx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(chromeUA),
	})
	if err != nil {
		return nil, fmt.Errorf("new context: %w", err)
	}

	page, err := bCtx.NewPage()
	if err != nil {
		bCtx.Close()
		return nil, fmt.Errorf("new page: %w", err)
	}

	// Use Load (not NetworkIdle) — CF challenge scripts keep firing beacon
	// requests indefinitely, so NetworkIdle never triggers.
	if _, err = page.Goto("https://letterboxd.com", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(120000),
	}); err != nil {
		bCtx.Close()
		return nil, fmt.Errorf("navigate: %w", err)
	}

	// .site-header appearing confirms the CF challenge resolved and we're on
	// the real site, not the interstitial.
	if _, err = page.WaitForSelector(".site-header", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(30000),
	}); err != nil {
		bCtx.Close()
		return nil, fmt.Errorf("CF challenge did not resolve (site-header never appeared): %w", err)
	}
	// Keep page alive — reused for all subsequent page navigation so CF
	// never sees a new tab/session.

	cookies, err := bCtx.Cookies("https://letterboxd.com")
	if err != nil {
		bCtx.Close()
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
		expiresAt = time.Now().Add(30 * time.Minute)
		log.Println("no cf_clearance expiry found — defaulting to 30min TTL")
	}

	return &cfSession{
		cookieHeader: strings.Join(parts, "; "),
		expiresAt:    expiresAt,
		bCtx:         bCtx,
		navPage:      page,
	}, nil
}

// injectCookies creates a Playwright context with the provided cookie string
// pre-loaded, then opens a navigation page. No CF challenge is triggered — the
// valid cf_clearance cookie from a prior harvest is used directly.
func injectCookies(browser playwright.Browser, cookieHeader string) (*cfSession, error) {
	bCtx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(chromeUA),
	})
	if err != nil {
		return nil, fmt.Errorf("new context: %w", err)
	}

	// Parse "key=val; key2=val2" into Playwright cookie objects.
	var cookies []playwright.OptionalCookie
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		cookies = append(cookies, playwright.OptionalCookie{
			Name:   strings.TrimSpace(k),
			Value:  v,
			Domain: playwright.String(".letterboxd.com"),
			Path:   playwright.String("/"),
		})
	}
	if err := bCtx.AddCookies(cookies); err != nil {
		bCtx.Close()
		return nil, fmt.Errorf("add cookies: %w", err)
	}

	page, err := bCtx.NewPage()
	if err != nil {
		bCtx.Close()
		return nil, fmt.Errorf("new page: %w", err)
	}

	log.Println("cookies injected into Playwright context — ready for navigation")
	return &cfSession{
		cookieHeader: cookieHeader,
		expiresAt:    time.Now().Add(24 * time.Hour),
		bCtx:         bCtx,
		navPage:      page,
	}, nil
}

// fetchPageViaPlaywright navigates the kept-alive page to url and returns the
// parsed films. Reusing the same page object means CF sees continuous browsing
// within one session — no new-tab re-challenge.
func fetchPageViaPlaywright(page playwright.Page, url string) ([]rawFilm, error) {
	if _, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	// Wait for the film grid to confirm we're on the real page, not a CF interstitial.
	if _, err := page.WaitForSelector(".griditem", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(20000),
	}); err != nil {
		// Save content for debugging if the selector is missing.
		if content, cerr := page.Content(); cerr == nil {
			_ = os.WriteFile("/tmp/lb-page2-debug.html", []byte(content), 0o644)
			log.Printf("griditem count in content: %d (saved to /tmp/lb-page2-debug.html)",
				strings.Count(content, "griditem"))
		}
		return nil, fmt.Errorf("film grid not found (CF block or structure change): %w", err)
	}

	content, err := page.Content()
	if err != nil {
		return nil, fmt.Errorf("get content: %w", err)
	}

	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	return parseFilms(doc), nil
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
		// New structure: <li class="griditem"> (was "poster-container")
		if n.Type == html.ElementNode && n.Data == "li" && hasClass(n, "griditem") {
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
	var f rawFilm

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type != html.ElementNode {
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			return
		}

		// Slug: <div class="react-component" data-item-slug="...">
		// (was data-film-slug on a generic div)
		if node.Data == "div" && hasClass(node, "react-component") {
			if s := getAttr(node, "data-item-slug"); s != "" {
				f.slug = s
			}
		}

		// Rating + liked: <p class="poster-viewingdata">
		//   <span class="rating ... rated-N"> — N is raw score, stars = N/2
		//   <span class="like liked-micro ... icon-liked ..."> — present only if liked
		if node.Data == "p" && hasClass(node, "poster-viewingdata") {
			var walkVD func(*html.Node)
			walkVD = func(vn *html.Node) {
				if vn.Type == html.ElementNode && vn.Data == "span" {
					cls := getAttr(vn, "class")
					// extract rated-N
					if f.rating == nil {
						for _, c := range strings.Fields(cls) {
							if strings.HasPrefix(c, "rated-") {
								if v, err := strconv.ParseFloat(strings.TrimPrefix(c, "rated-"), 32); err == nil && v > 0 {
									r := float32(v) / 2.0
									f.rating = &r
								}
							}
						}
					}
					// liked when icon-liked class is present
					if strings.Contains(cls, "icon-liked") {
						f.liked = true
					}
				}
				for c := vn.FirstChild; c != nil; c = c.NextSibling {
					walkVD(c)
				}
			}
			walkVD(node)
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)

	return f, f.slug != ""
}

func parseTotalPages(doc *html.Node) int {
	var maxPage int
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && hasClass(n, "paginate-page") {
			if a := firstChildElement(n, "a"); a != nil {
				if v, err := strconv.Atoi(strings.TrimSpace(textContent(a))); err == nil && v > maxPage {
					maxPage = v
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if maxPage == 0 {
		return 1
	}
	return maxPage
}

func firstChildElement(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
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
