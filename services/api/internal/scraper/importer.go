package scraper

import "context"

type ProfileImporter interface {
	ImportProfile(ctx context.Context, handle string) ([]RawRating, error)
}
