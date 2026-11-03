// importcheck runs one import source for a handle and prints a summary,
// without touching the database or queue. Use it when a source breaks:
//
//	go run ./cmd/importcheck -handle adityadeshmukh
//	go run ./cmd/importcheck -handle adityadeshmukh -source letterboxd_web
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/framed-app/api/internal/importer"
)

func main() {
	handle := flag.String("handle", "", "account handle on the source")
	source := flag.String("source", string(importer.SourceLetterboxdRSS), "import source")
	flag.Parse()
	if *handle == "" {
		log.Fatal("-handle is required")
	}

	registry := importer.NewRegistry(importer.NewLetterboxdRSS(), importer.NewLetterboxdWeb())
	imp, err := registry.Get(importer.Source(*source))
	if err != nil {
		log.Fatal(err)
	}

	films, err := imp.Import(context.Background(), importer.Request{Handle: *handle})
	if err != nil {
		log.Fatalf("import failed: %v", err)
	}

	var rated, liked, withTMDB int
	for _, f := range films {
		if f.Rating != nil {
			rated++
		}
		if f.Liked {
			liked++
		}
		if f.TMDBMovieID != 0 {
			withTMDB++
		}
	}
	fmt.Printf("%s: %d films (%d rated, %d liked, %d with TMDB ID)\n", imp.Source(), len(films), rated, liked, withTMDB)
	for i, f := range films {
		if i == 5 {
			break
		}
		fmt.Printf("  %s (%d) slug=%s rating=%v liked=%v\n", f.Title, f.Year, f.LetterboxdSlug, deref(f.Rating), f.Liked)
	}
}

func deref(f *float32) any {
	if f == nil {
		return "-"
	}
	return *f
}
