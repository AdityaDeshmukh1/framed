package importer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeImporter struct{ source Source }

func (f fakeImporter) Source() Source                                  { return f.source }
func (f fakeImporter) Import(context.Context, Request) ([]Film, error) { return nil, nil }

func TestRegistryDefaultAndLookup(t *testing.T) {
	r := NewRegistry(fakeImporter{"a"}, fakeImporter{"b"})
	if r.Default() != "a" {
		t.Errorf("default = %q, want a", r.Default())
	}
	if imp, err := r.Get(""); err != nil || imp.Source() != "a" {
		t.Errorf("Get(\"\") = %v, %v; want default importer", imp, err)
	}
	if imp, err := r.Get("b"); err != nil || imp.Source() != "b" {
		t.Errorf("Get(b) = %v, %v", imp, err)
	}
	if _, err := r.Get("nope"); err == nil {
		t.Error("Get(nope) should fail")
	}
}

const sampleFeed = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:letterboxd="https://letterboxd.com" xmlns:tmdb="https://themoviedb.org">
<channel>
  <item>
    <link>https://letterboxd.com/someone/film/mulholland-drive/1/</link>
    <letterboxd:watchedDate>2026-10-01</letterboxd:watchedDate>
    <letterboxd:rewatch>Yes</letterboxd:rewatch>
    <letterboxd:filmTitle>Mulholland Drive</letterboxd:filmTitle>
    <letterboxd:filmYear>2001</letterboxd:filmYear>
    <letterboxd:memberRating>5.0</letterboxd:memberRating>
    <letterboxd:memberLike>Yes</letterboxd:memberLike>
    <tmdb:movieId>1018</tmdb:movieId>
  </item>
  <item>
    <link>https://letterboxd.com/someone/film/mulholland-drive/</link>
    <letterboxd:filmTitle>Mulholland Drive</letterboxd:filmTitle>
    <letterboxd:filmYear>2001</letterboxd:filmYear>
    <letterboxd:memberRating>4.0</letterboxd:memberRating>
    <tmdb:movieId>1018</tmdb:movieId>
  </item>
  <item>
    <link>https://letterboxd.com/someone/list/favourites/</link>
  </item>
  <item>
    <link>https://letterboxd.com/someone/film/stalker/</link>
    <letterboxd:filmTitle>Stalker</letterboxd:filmTitle>
    <letterboxd:filmYear>1979</letterboxd:filmYear>
    <tmdb:movieId>1398</tmdb:movieId>
  </item>
</channel>
</rss>`

func TestLetterboxdRSSImport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/someone/rss/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(sampleFeed))
	}))
	defer server.Close()

	rss := &LetterboxdRSS{client: server.Client(), baseURL: server.URL}
	films, err := rss.Import(context.Background(), Request{Handle: "someone"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	// The repeat diary entry and the list item are dropped.
	if len(films) != 2 {
		t.Fatalf("got %d films, want 2: %+v", len(films), films)
	}

	md := films[0]
	if md.LetterboxdSlug != "mulholland-drive" || md.TMDBMovieID != 1018 || md.Year != 2001 {
		t.Errorf("unexpected film: %+v", md)
	}
	if md.Rating == nil || *md.Rating != 5.0 || !md.Liked || !md.Rewatch || md.WatchedDate == nil {
		t.Errorf("most recent entry should win: %+v", md)
	}
	if films[1].Rating != nil {
		t.Errorf("unrated film should have nil rating, got %v", *films[1].Rating)
	}

	if _, err := rss.Import(context.Background(), Request{Handle: "missing"}); err == nil {
		t.Error("unknown user should fail")
	}
}
