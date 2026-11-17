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

func TestSummarizeFlowThroughputCycleAndAging(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	archived := now.AddDate(0, 0, -2)
	created := now.AddDate(0, 0, -6)
	oldArchived := now.AddDate(0, 0, -20)
	cards := []domain.Card{
		{ID: "done-recent", Title: "Recent", CreatedAt: created, DeletedAt: &archived, ColumnEnteredAt: created},
		{ID: "done-old", Title: "Old", CreatedAt: oldArchived.AddDate(0, 0, -4), DeletedAt: &oldArchived, ColumnEnteredAt: oldArchived},
		{ID: "aging", Title: "Stuck card", ColumnID: "doing", CreatedAt: now.AddDate(0, 0, -9), ColumnEnteredAt: now.AddDate(0, 0, -9)},
		{ID: "fresh", Title: "Fresh card", ColumnID: "doing", CreatedAt: now.AddDate(0, 0, -1), ColumnEnteredAt: now.AddDate(0, 0, -1)},
	}
	summary := summarizeFlow(cards, now)
	require.Equal(t, 2, summary.archivedTotal)
	require.InDelta(t, 4.0, summary.avgArchiveDays, 0.01, "(4d + 4d) / 2")
	require.Len(t, summary.throughput, 8)
	require.Equal(t, 1, summary.throughput[7], "recent archive lands in the current week")
	require.Equal(t, 1, summary.throughput[5], "three-week-old archive lands in its week")
	require.NotEmpty(t, summary.aging)
	require.Equal(t, "Stuck card", summary.aging[0].title, "oldest active card first")
	require.Equal(t, 9, summary.aging[0].days)
}

func TestFlowLinesRenderSparklineAndAging(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	archived := now.AddDate(0, 0, -1)
	cards := []domain.Card{
		{ID: "done", Title: "Shipped", CreatedAt: now.AddDate(0, 0, -3), DeletedAt: &archived},
		{ID: "wip", Title: "Aging card", ColumnID: "doing", ColumnEnteredAt: now.AddDate(0, 0, -5)},
	}
	lines := flowLines(summarizeFlow(cards, now), map[string]string{"doing": "Doing"})
	joined := joinLines(lines)
	require.Contains(t, joined, "Throughput")
	require.Contains(t, joined, "Average days to archive: 2.0")
	require.Contains(t, joined, "Aging card")
	require.Contains(t, joined, "Doing")
	require.Contains(t, joined, "5d")
}

func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result
}

func TestFlowCommandOpensFlowPopup(t *testing.T) {
	ctx := context.Background()
	repo, err := storagesqlite.Open(ctx, filepath.Join(t.TempDir(), "flow.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Doing", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Aging task", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &card))

	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.project = &project
	model.board = &board
	model.columns = []domain.Column{column}
	model.cards[column.ID] = []domain.Card{card}

	_, command := model.executeCommand("flow")
	require.NotNil(t, command)
	runCommands(model, command)
	require.NotNil(t, model.detail)
	require.Equal(t, "flow", model.detail.kind)
	require.Contains(t, joinLines(model.detail.lines), "Aging task")
	require.Contains(t, joinLines(model.detail.lines), "Throughput")
}
