package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCardMarkdownRendersShareableDocument(t *testing.T) {
	priority := "High"
	due := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	card := Card{
		ID: "card-1", Title: "Ship editor", Description: "Use **Markdown**.\n\nSecond paragraph.",
		Priority: &priority, DueDate: &due, Tags: []string{"tui", "release"},
		Checklist: []ChecklistItem{
			{ID: "i1", Text: "Write tests", Done: true, Position: 1024},
			{ID: "i2", Text: "Ship it", Position: 2048},
		},
		Fields: map[string]FieldValue{
			"link":     {Type: FieldURL, Value: "https://example.com"},
			"estimate": {Type: FieldNumber, Value: 3},
		},
	}
	markdown := CardMarkdown(card, "Doing")
	require.Contains(t, markdown, "# Ship editor")
	require.Contains(t, markdown, "Status: Doing")
	require.Contains(t, markdown, "Priority: High")
	require.Contains(t, markdown, "Due: 2026-11-20")
	require.Contains(t, markdown, "Tags: tui, release")
	require.Contains(t, markdown, "Use **Markdown**.")
	require.Contains(t, markdown, "Second paragraph.")
	require.Contains(t, markdown, "- [x] Write tests")
	require.Contains(t, markdown, "- [ ] Ship it")
	require.Contains(t, markdown, "- estimate [number]: 3")
	require.Contains(t, markdown, "- link [url]: \"https://example.com\"")
	require.Contains(t, markdown, "Card ID: card-1")
	require.Less(t, strings.Index(markdown, "estimate"), strings.Index(markdown, "link"), "fields render in sorted order")
}

func TestCardMarkdownMinimalCard(t *testing.T) {
	markdown := CardMarkdown(Card{ID: "bare", Title: "Bare"}, "")
	require.Contains(t, markdown, "# Bare")
	require.Contains(t, markdown, "Card ID: bare")
	require.NotContains(t, markdown, "Status:")
	require.NotContains(t, markdown, "## Checklist")
	require.NotContains(t, markdown, "## Fields")
}
