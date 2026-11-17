package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestWrapCardTitlesShowsFullName(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := testModel(readRepository{})
	model.wrapCardTitles = true
	card := domain.Card{Title: "Сделать read-only режим для гостей доски"}
	row := cardDisplayRow{card: card}

	rendered := model.renderCardDisplayRow(row, false, 24, 10)
	require.NotContains(t, rendered, "…")
	require.Contains(t, rendered, "гостей")
	require.Greater(t, lipgloss.Height(rendered), 1)

	model.wrapCardTitles = false
	truncated := model.renderCardDisplayRow(row, false, 24, 10)
	require.Contains(t, truncated, "…")
	require.Equal(t, 1, lipgloss.Height(truncated))
}

func TestWrapCardTitlesKeepsPriorityMarkerAligned(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := testModel(readRepository{})
	model.wrapCardTitles = true
	priority := "Medium"
	card := domain.Card{Title: "Подключить hermes к доске проекта", Priority: &priority}

	rendered := model.renderCardDisplayRow(cardDisplayRow{card: card}, false, 24, 10)
	require.NotContains(t, rendered, "…")
	lines := strings.Split(rendered, "\n")
	first := strings.TrimLeft(lines[0], " ")
	require.True(t, strings.HasPrefix(first, "M "), "priority marker stays on the first line")
	indent := len(lines[0]) - len(first)
	require.True(t, strings.HasPrefix(lines[1], strings.Repeat(" ", indent+2)), "continuation lines align under the title")
	require.Contains(t, rendered, "проекта")
}
