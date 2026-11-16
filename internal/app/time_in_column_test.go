package app

import (
	"strings"
	"testing"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestInspectorShowsTimeInColumn(t *testing.T) {
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "doing", BoardID: "board", Name: "Doing"}}
	model.cards["doing"] = []domain.Card{{ID: "aged", ColumnID: "doing", Title: "Aged", ColumnEnteredAt: time.Now().Add(-4 * 24 * time.Hour)}}

	lines := model.inspectorLines(60)
	require.Contains(t, strings.Join(lines, "\n"), "In column: 4d")
}

func TestCardDetailShowsTimeInColumn(t *testing.T) {
	card := domain.Card{ID: "aged", Title: "Aged", ColumnEnteredAt: time.Now().Add(-24 * time.Hour)}
	detail := cardDetail(card, "Doing")
	require.Contains(t, strings.Join(detail.lines, "\n"), "In column: 1d")
}

func TestTimeInColumnLabels(t *testing.T) {
	now := time.Now()
	for _, testCase := range []struct {
		entered time.Time
		want    string
	}{
		{now.Add(-2 * time.Hour), "<1d"},
		{now.Add(-24 * time.Hour), "1d"},
		{now.Add(-4 * 24 * time.Hour), "4d"},
		{now.Add(-40 * 24 * time.Hour), "40d"},
	} {
		card := domain.Card{ColumnEnteredAt: testCase.entered}
		require.Equal(t, testCase.want, timeInColumnLabel(card, now))
	}

	// Falls back to the creation time when the column timestamp is missing.
	card := domain.Card{CreatedAt: now.Add(-3 * 24 * time.Hour)}
	require.Equal(t, "3d", timeInColumnLabel(card, now))

	// Unsaved cards without timestamps get no label.
	require.Empty(t, timeInColumnLabel(domain.Card{}, now))
}
