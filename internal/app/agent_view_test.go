package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	storagesqlite "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestAgentStatusMarkerOnCardLabel(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := testModel(readRepository{})
	for status, marker := range map[string]string{
		domain.AgentQueued:  "⏳",
		domain.AgentRunning: "⚙",
		domain.AgentFailed:  "✗",
	} {
		card := domain.Card{Title: "Task", AgentStatus: status}
		require.Contains(t, model.cardLabel(card, 40, false), marker, status)
	}
	done := domain.Card{Title: "Task", AgentStatus: domain.AgentDone}
	require.NotContains(t, model.cardLabel(done, 40, false), "⚙")
}

func TestInspectorAndDetailShowAgentRun(t *testing.T) {
	card := domain.Card{
		ID: "c", ColumnID: "doing", Title: "Task",
		AgentStatus: domain.AgentRunning, AgentName: "tester", AgentResult: "agent output line",
	}
	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "doing", BoardID: "board", Name: "Doing"}}
	model.cards["doing"] = []domain.Card{card}

	inspector := strings.Join(model.inspectorLines(60), "\n")
	require.Contains(t, inspector, "Agent: running (tester)")
	require.Contains(t, inspector, "agent output line")

	detail := cardDetail(card, "Doing")
	require.Contains(t, strings.Join(detail.lines, "\n"), "Agent: running (tester)")
}

func TestQueueCardCommandQueuesSelectedCard(t *testing.T) {
	ctx := context.Background()
	repo, err := storagesqlite.Open(ctx, filepath.Join(t.TempDir(), "agent.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Queue", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Task", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &card))

	model := testModel(repo)
	model.loading = false
	model.screen = boardScreen
	model.project = &project
	model.board = &board
	model.columns = []domain.Column{column}
	model.cards[column.ID] = []domain.Card{card}

	_, command := model.executeCommand("card-queue")
	require.NotNil(t, command)
	runCommands(model, command)
	require.Equal(t, "Card queued for agents", model.notice)
	require.Equal(t, domain.AgentQueued, model.cards[column.ID][0].AgentStatus)

	stored, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentQueued, stored.AgentStatus)
}
