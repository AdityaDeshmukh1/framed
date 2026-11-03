// Package importer pulls a user's film history from an external source.
//
// Each source (Letterboxd RSS, the Letterboxd website, and whatever comes
// next) implements Importer and returns the same []Film shape. The onboarding
// pipeline only ever talks to the Importer interface, so adding a source means
// writing one type and registering it — nothing downstream changes.
package importer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Source names an import source. It is stored on scrape_jobs.source, so
// values must stay stable once in use.
type Source string

const (
	// SourceLetterboxdRSS reads the official Letterboxd RSS feed: reliable, but
	// only the most recent ~50 diary entries.
	SourceLetterboxdRSS Source = "letterboxd_rss"
	// SourceLetterboxdWeb drives headless Chromium over the profile's film
	// pages for the full history. Currently blocked by Cloudflare beyond page 1.
	SourceLetterboxdWeb Source = "letterboxd_web"
)

// Request describes what to import. Handle identifies the account on the
// source; new sources can add fields (tokens, uploaded files) here.
type Request struct {
	Handle string
	// Submitted carries answers for sources where the client sends the
	// ratings itself (the quiz).
	Submitted []Submitted
}

// Film is one film from a user's history, as reported by a source.
type Film struct {
	// TMDBMovieID is 0 when the source doesn't know it; the pipeline then
	// resolves it via TMDB search on Title and Year.
	TMDBMovieID    int
	LetterboxdSlug string // bare slug, e.g. "mulholland-drive"; may be empty
	Title          string
	Year           int
	Rating         *float32 // 0.5–5.0, nil when unrated
	Liked          bool
	WatchedDate    *time.Time
	Rewatch        bool
}

// Importer fetches a user's film history from one source.
type Importer interface {
	Source() Source
	Import(ctx context.Context, req Request) ([]Film, error)
}

// Registry holds the enabled importers and which one onboarding uses by default.
type Registry struct {
	importers map[Source]Importer
	def       Source
}

// NewRegistry returns a registry of the given importers. The first one is the
// default for requests that don't name a source.
func NewRegistry(importers ...Importer) *Registry {
	r := &Registry{importers: make(map[Source]Importer, len(importers))}
	for i, imp := range importers {
		if i == 0 {
			r.def = imp.Source()
		}
		r.importers[imp.Source()] = imp
	}
	return r
}

// Default is the source used when a request doesn't specify one.
func (r *Registry) Default() Source { return r.def }

// Get returns the importer for source, or the default when source is empty.
func (r *Registry) Get(source Source) (Importer, error) {
	if source == "" {
		source = r.def
	}
	imp, ok := r.importers[source]
	if !ok {
		return nil, fmt.Errorf("unknown or disabled import source %q (enabled: %s)", source, r.enabled())
	}
	return imp, nil
}

func (r *Registry) enabled() string {
	names := make([]string, 0, len(r.importers))
	for s := range r.importers {
		names = append(names, string(s))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
