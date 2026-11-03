// Package embedding turns films into vectors: it builds the text we embed for
// each film and calls the ML service (services/ml, all-MiniLM-L6-v2).
//
// Everything that embeds films (the pipeline worker, the catalogue loader,
// the evaluation harness) goes through Text, so a film's vector never depends
// on which code path produced it. Changing the text means bumping
// TextVersion and re-embedding (go run ./cmd/catalogue -reembed).
package embedding

import (
	"fmt"
	"strings"
)

// TextVersion is stored on films.embedding_version.
//
//	v1: genres. directors. keywords. synopsis
//	v2: v1 prefixed with language, country and decade. TMDB text is English
//	    for every film, so without this the embedding cannot tell a French
//	    1960s drama from an American 2010s one with a similar plot.
const TextVersion = 2

// Film is what the text is built from. Language is an ISO 639-1 code and
// Country an ISO 3166-1 code, as TMDB returns them.
type Film struct {
	Title     string
	Year      int
	Genres    []string
	Directors []string
	Keywords  []string
	Synopsis  string
	Language  string
	Country   string
}

// Text returns the embedding text for f at the given version.
func Text(f Film, version int) string {
	core := strings.Join(f.Genres, " ") + ". " +
		strings.Join(f.Directors, " ") + ". " +
		strings.Join(f.Keywords, " ") + ". " +
		f.Synopsis
	if strings.TrimSpace(strings.ReplaceAll(core, ".", "")) == "" {
		// Nothing from TMDB: title and year still beat an empty string.
		core = fmt.Sprintf("%s %d", f.Title, f.Year)
	}
	if version < 2 {
		return core
	}
	// Metadata goes first: MiniLM truncates long inputs at 256 tokens, and a
	// long keyword list must not push it out.
	return describe(f) + " " + core
}

// describe renders e.g. "French-language film from France, 1960s."
func describe(f Film) string {
	var b strings.Builder
	if name := languageNames[f.Language]; name != "" {
		b.WriteString(name + "-language film")
	} else {
		b.WriteString("Film")
	}
	if name := countryNames[f.Country]; name != "" {
		b.WriteString(" from " + name)
	} else if f.Country != "" {
		b.WriteString(" from " + f.Country)
	}
	if f.Year > 0 {
		fmt.Fprintf(&b, ", %ds", f.Year/10*10)
	}
	b.WriteString(".")
	return b.String()
}

var languageNames = map[string]string{
	"en": "English", "fr": "French", "de": "German", "it": "Italian", "es": "Spanish",
	"pt": "Portuguese", "ja": "Japanese", "ko": "Korean", "zh": "Mandarin", "cn": "Cantonese",
	"hi": "Hindi", "ru": "Russian", "sv": "Swedish", "da": "Danish", "no": "Norwegian",
	"fi": "Finnish", "nl": "Dutch", "pl": "Polish", "cs": "Czech", "hu": "Hungarian",
	"tr": "Turkish", "fa": "Persian", "ar": "Arabic", "he": "Hebrew", "th": "Thai",
	"el": "Greek", "ro": "Romanian", "ta": "Tamil", "te": "Telugu", "ml": "Malayalam",
	"id": "Indonesian", "vi": "Vietnamese", "tl": "Tagalog", "uk": "Ukrainian", "is": "Icelandic",
}

var countryNames = map[string]string{
	"US": "the United States", "GB": "the United Kingdom", "FR": "France", "DE": "Germany",
	"IT": "Italy", "ES": "Spain", "JP": "Japan", "KR": "South Korea", "CN": "China",
	"HK": "Hong Kong", "TW": "Taiwan", "IN": "India", "CA": "Canada", "AU": "Australia",
	"NZ": "New Zealand", "IE": "Ireland", "SE": "Sweden", "DK": "Denmark", "NO": "Norway",
	"FI": "Finland", "NL": "the Netherlands", "BE": "Belgium", "PL": "Poland", "RU": "Russia",
	"SU": "the Soviet Union", "MX": "Mexico", "BR": "Brazil", "AR": "Argentina", "IR": "Iran",
	"TR": "Turkey", "IL": "Israel", "CZ": "Czechia", "HU": "Hungary", "AT": "Austria",
	"CH": "Switzerland", "GR": "Greece", "RO": "Romania", "TH": "Thailand", "ZA": "South Africa",
}
