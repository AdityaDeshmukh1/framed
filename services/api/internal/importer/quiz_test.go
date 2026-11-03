package importer

import (
	"context"
	"errors"
	"testing"
)

func f32(v float32) *float32 { return &v }

func catalogue(ids ...int) CatalogueLookup {
	return func(_ context.Context, want []int) (map[int]CatalogueFilm, error) {
		out := map[int]CatalogueFilm{}
		for _, id := range ids {
			out[id] = CatalogueFilm{Title: "Film", Year: 2000}
		}
		return out, nil
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		answers []Submitted
		ok      bool
	}{
		{"empty", nil, false},
		{"good", []Submitted{{TMDBMovieID: 1, Rating: f32(4.5), Liked: true}}, true},
		{"unrated is fine", []Submitted{{TMDBMovieID: 1}}, true},
		{"bad id", []Submitted{{TMDBMovieID: 0, Rating: f32(3)}}, false},
		{"rating too high", []Submitted{{TMDBMovieID: 1, Rating: f32(5.5)}}, false},
		{"not a half star", []Submitted{{TMDBMovieID: 1, Rating: f32(3.3)}}, false},
		{"too many", make([]Submitted, MaxQuizRatings+1), false},
	}
	for _, c := range cases {
		err := Validate(c.answers)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
		if err != nil && !errors.Is(err, ErrInvalidSubmission) {
			t.Errorf("%s: error should wrap ErrInvalidSubmission", c.name)
		}
	}
}

func TestQuizKeepsOnlyCatalogueFilmsOnce(t *testing.T) {
	q := NewQuiz(catalogue(10, 20))
	films, err := q.Import(context.Background(), Request{Submitted: []Submitted{
		{TMDBMovieID: 10, Rating: f32(5), Liked: true},
		{TMDBMovieID: 99, Rating: f32(4)}, // not in catalogue
		{TMDBMovieID: 10, Rating: f32(1)}, // duplicate: first answer wins
		{TMDBMovieID: 20, Rating: f32(2)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(films) != 2 || films[0].TMDBMovieID != 10 || *films[0].Rating != 5 || films[1].TMDBMovieID != 20 {
		t.Fatalf("got %+v", films)
	}
	if films[0].Title != "Film" || films[0].Year != 2000 {
		t.Errorf("title/year not filled from catalogue: %+v", films[0])
	}
}

func TestQuizRejectsAllUnknown(t *testing.T) {
	_, err := NewQuiz(catalogue()).Import(context.Background(), Request{Submitted: []Submitted{{TMDBMovieID: 5, Rating: f32(3)}}})
	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("err = %v, want ErrInvalidSubmission", err)
	}
}
