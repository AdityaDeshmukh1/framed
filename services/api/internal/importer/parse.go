package importer

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type rawFilm struct {
	slug     string
	rating   *float32
	liked    bool
	itemName string // data-item-name e.g. "Dhurandhar (2025)"
}

func parseFilms(doc *html.Node) []rawFilm {
	var films []rawFilm
	var walk func(*html.Node)
	walk = func(n *html.Node) {
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

		// Slug + name: <div class="react-component" data-item-slug="..." data-item-name="...">
		if node.Data == "div" && hasClass(node, "react-component") {
			if s := getAttr(node, "data-item-slug"); s != "" {
				f.slug = s
			}
			if name := getAttr(node, "data-item-name"); name != "" {
				f.itemName = name
			}
		}

		// Rating + liked: <p class="poster-viewingdata">
		//   <span class="rating rated-N"> — N is raw score 1-10; stars = N/2
		//   <span class="icon-liked ..."> — present only when liked
		if node.Data == "p" && hasClass(node, "poster-viewingdata") {
			var walkVD func(*html.Node)
			walkVD = func(vn *html.Node) {
				if vn.Type == html.ElementNode && vn.Data == "span" {
					cls := getAttr(vn, "class")
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
			if a := firstChild(n, "a"); a != nil {
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

func firstChild(n *html.Node, tag string) *html.Node {
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
