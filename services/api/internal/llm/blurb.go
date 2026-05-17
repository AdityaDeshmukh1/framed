package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// TasteSnapshot is the taste data passed in for one user when generating a blurb.
type TasteSnapshot struct {
	TopGenres    []string
	TopDirectors []string
	SummaryLine  string // first sentence of soul portrait, may be empty
}

const blurbSystemPrompt = `You write one-sentence match blurbs for a film-taste dating app.
The blurb explains why two people would click based on their film taste.
Tone: warm, specific, slightly witty. Second person ("You both...").
One sentence only. No more than 20 words. Be concrete — reference what they actually share.
Never be generic. Never say "great match" or "compatible". Never mention the app.`

// GenerateBlurb produces a one-sentence "why you'd click" blurb for a match pair.
func GenerateBlurb(ctx context.Context, apiKey string, a, b TasteSnapshot) (string, error) {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 80,
		System: []anthropic.TextBlockParam{{
			Text: blurbSystemPrompt,
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(buildBlurbPrompt(a, b))),
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

func buildBlurbPrompt(a, b TasteSnapshot) string {
	var sb strings.Builder
	sb.WriteString("Person A:\n")
	if len(a.TopGenres) > 0 {
		sb.WriteString("  Genres: " + strings.Join(a.TopGenres, ", ") + "\n")
	}
	if len(a.TopDirectors) > 0 {
		sb.WriteString("  Directors: " + strings.Join(a.TopDirectors, ", ") + "\n")
	}
	if a.SummaryLine != "" {
		sb.WriteString("  Portrait: " + a.SummaryLine + "\n")
	}
	sb.WriteString("\nPerson B:\n")
	if len(b.TopGenres) > 0 {
		sb.WriteString("  Genres: " + strings.Join(b.TopGenres, ", ") + "\n")
	}
	if len(b.TopDirectors) > 0 {
		sb.WriteString("  Directors: " + strings.Join(b.TopDirectors, ", ") + "\n")
	}
	if b.SummaryLine != "" {
		sb.WriteString("  Portrait: " + b.SummaryLine + "\n")
	}
	sb.WriteString("\nWrite the blurb.")
	return sb.String()
}
