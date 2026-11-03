package embedding

import (
	"strings"
	"testing"
)

func TestTextV1Unchanged(t *testing.T) {
	f := Film{Genres: []string{"Drama"}, Directors: []string{"Agnès Varda"}, Keywords: []string{"paris"}, Synopsis: "A singer waits."}
	if got, want := Text(f, 1), "Drama. Agnès Varda. paris. A singer waits."; got != want {
		t.Errorf("v1 = %q, want %q", got, want)
	}
}

func TestTextV2PrefixesMetadata(t *testing.T) {
	f := Film{Year: 1962, Language: "fr", Country: "FR", Genres: []string{"Drama"}, Synopsis: "A singer waits."}
	got := Text(f, 2)
	if !strings.HasPrefix(got, "French-language film from France, 1960s. Drama.") {
		t.Errorf("v2 = %q", got)
	}
}

func TestTextFallsBackToTitle(t *testing.T) {
	if got := Text(Film{Title: "Cléo", Year: 1962}, 1); got != "Cléo 1962" {
		t.Errorf("got %q", got)
	}
}

func TestVectorRoundTrip(t *testing.T) {
	v, err := ParseVector(FormatVector([]float32{0.25, -1, 3e-5}))
	if err != nil || len(v) != 3 || v[0] != 0.25 || v[1] != -1 {
		t.Fatalf("got %v, %v", v, err)
	}
}
