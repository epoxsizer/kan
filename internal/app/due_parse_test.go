package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	storagesqlite "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestParseNaturalDue(t *testing.T) {
	now := time.Date(2026, 11, 17, 15, 0, 0, 0, time.UTC)
	daysUntil := func(target time.Weekday) string {
		delta := (int(target) - int(now.Weekday()) + 7) % 7
		if delta == 0 {
			delta = 7
		}
		return now.AddDate(0, 0, delta).Format("2006-01-02")
	}
	for _, testCase := range []struct{ input, want string }{
		{"today", "2026-11-17"},
		{"tomorrow", "2026-11-18"},
		{"+3d", "2026-11-20"},
		{"+2w", "2026-12-01"},
		{"fri", daysUntil(time.Friday)},
		{"monday", daysUntil(time.Monday)},
		{"next mon", daysUntil(time.Monday)},
		{"2026-12-01", "2026-12-01"},
		{"", ""},
	} {
		got, err := parseNaturalDue(testCase.input, now)
		require.NoError(t, err, testCase.input)
		if testCase.want == "" {
			require.Nil(t, got)
			continue
		}
		require.Equal(t, testCase.want, got.Format("2006-01-02"), testCase.input)
	}
	_, err := parseNaturalDue("someday", now)
	require.Error(t, err)
}

func TestCardFormAcceptsNaturalDueDate(t *testing.T) {
	ctx := context.Background()
	repo, err := storagesqlite.Open(ctx, filepath.Join(t.TempDir(), "due.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Todo", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))

	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.project = &project
	model.board = &board
	model.columns = []domain.Column{column}
	_ = model.startCardForm(false)
	model.form.fields[0].value = "Natural due"
	model.form.focus = 4
	for _, character := range "tomorrow" {
		model.Update(key(string(character)))
	}
	require.Equal(t, "tomorrow", model.form.fields[4].value)

	model.Update(key("enter"))
	want := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	require.Equal(t, want, model.form.fields[4].value, "Enter parses the typed date in place")

	_, command := model.Update(key("ctrl+s"))
	runCommands(model, command)
	cards, err := repo.ListCards(ctx, board.ID)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.NotNil(t, cards[0].DueDate)
	require.Equal(t, want, cards[0].DueDate.Format("2006-01-02"))
}
