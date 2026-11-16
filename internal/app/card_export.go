package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
)

// cardExportFinishedMsg reports the Markdown file written for a card.
type cardExportFinishedMsg struct {
	path string
	err  error
}

// exportSelectedCardMarkdown writes the selected card as a Markdown file in
// the working directory, off the UI goroutine.
func (model *Model) exportSelectedCardMarkdown() tea.Cmd {
	card := model.selectedCard()
	column := model.columns[model.columnIndex]
	markdown := domain.CardMarkdown(card, column.Name)
	name := cardExportFileName(card)
	return func() tea.Msg {
		path, err := filepath.Abs(name)
		if err != nil {
			return cardExportFinishedMsg{err: fmt.Errorf("resolve export path: %w", err)}
		}
		if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
			return cardExportFinishedMsg{err: fmt.Errorf("write markdown %q: %w", path, err)}
		}
		return cardExportFinishedMsg{path: path}
	}
}

// cardExportFileName builds a readable, collision-free file name from the
// card title and a short ID suffix.
func cardExportFileName(card domain.Card) string {
	slug := strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			return character
		case character >= 'A' && character <= 'Z':
			return character + ('a' - 'A')
		default:
			return '-'
		}
	}, card.Title)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	id := card.ID
	if len(id) > 8 {
		id = id[:8]
	}
	if slug == "" {
		return "kan-card-" + id + ".md"
	}
	return "kan-card-" + slug + "-" + id + ".md"
}
