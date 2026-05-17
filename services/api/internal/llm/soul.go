package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const soulSystemPrompt = `You write Cinematic Soul portraits for a film-taste dating app called Framed.
  Write 2-3 sentences. One flowing paragraph.
  Tone: deadpan, witty, slightly cutting. Like a horoscope written by someone who has seen too many
  movies.
  Each sentence escalates — first observational, second an inference, third the uncomfortable truth.
  Plain language. No film jargon. You may reference well-known films, directors, or actors that a
  casual moviegoer would recognize (think Nolan, SRK, The Devil Wears Prada — not Tarkovsky or
  Chantal Akerman). No obscure references. If in doubt, leave the name out.
  Never be generic. Never mention the app.
  Like: 'Watches everything twice. Probably cried at a film they'd never admit to. The kind of
  person who has opinions about directors most people haven't heard of, but somehow makes it feel
  like a personality rather than a problem.'`

// GenerateSoul calls the Anthropic API to produce a Cinematic Soul portrait
// from a user's film taste data. Returns the portrait text.
func GenerateSoul(
	ctx context.Context,
	apiKey string,
	topGenres []string,
	topDirectors []string,
	highRatedFilms []string,
	lowRatedFilms []string,
	rewatchedFilms []string,
) (string, error) {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{{
			Text: soulSystemPrompt,
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(buildSoulPrompt(topGenres, topDirectors, highRatedFilms, lowRatedFilms, rewatchedFilms))),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}

	for _, block := range msg.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			return strings.TrimSpace(tb.Text), nil
		}
	}

	return "", fmt.Errorf("no text block in response")
}

// buildSoulPrompt formats the taste data into a user prompt.
func buildSoulPrompt(topGenres, topDirectors, highRatedFilms, lowRatedFilms, rewatchedFilms []string) string {
	var sb strings.Builder
	if len(topGenres) > 0 {
		sb.WriteString("Top genres: " + strings.Join(topGenres, ", ") + "\n")
	}
	if len(topDirectors) > 0 {
		sb.WriteString("Favourite directors: " + strings.Join(topDirectors, ", ") + "\n")
	}
	if len(highRatedFilms) > 0 {
		sb.WriteString("Loved (highest rated): " + strings.Join(highRatedFilms, ", ") + "\n")
	}
	if len(lowRatedFilms) > 0 {
		sb.WriteString("Disliked (lowest rated): " + strings.Join(lowRatedFilms, ", ") + "\n")
	}
	if len(rewatchedFilms) > 0 {
		sb.WriteString("Rewatched: " + strings.Join(rewatchedFilms, ", ") + "\n")
	}
	sb.WriteString("\nWrite their Cinematic Soul portrait.")
	return sb.String()
}
