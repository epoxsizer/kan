package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func agentTestCards(t *testing.T, dbPath string, board domain.Board, column domain.Column) []domain.Card {
	t.Helper()
	ctx := context.Background()
	repo, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	cards := []domain.Card{
		{BoardID: board.ID, ColumnID: column.ID, Title: "First", Description: "Do it", Position: 1024, AgentStatus: domain.AgentQueued},
		{BoardID: board.ID, ColumnID: column.ID, Title: "Second", Position: 2048, AgentStatus: domain.AgentQueued},
		{BoardID: board.ID, ColumnID: column.ID, Title: "Idle", Position: 3072},
	}
	for index := range cards {
		require.NoError(t, repo.CreateCard(ctx, &cards[index]))
	}
	require.NoError(t, repo.Close())
	return cards
}

func TestAgentRunProcessesQueuedCards(t *testing.T) {
	dbPath, logPath, board, column, other := addTestDatabase(t)
	cards := agentTestCards(t, dbPath, board, column)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))
	t.Setenv(config.EnvConfig, configPath)
	t.Setenv(config.EnvMCPToken, "")

	var notified []string
	previous := sendNotification
	sendNotification = func(title, body string) { notified = append(notified, title+"|"+body) }
	t.Cleanup(func() { sendNotification = previous })

	root := New("test", "abc123", "today")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "agent", "run",
		"--command", "echo processed", "--column", column.ID, "--agent", "tester",
		"--limit", "1", "--done-column", other.ID})
	require.NoError(t, root.Execute())

	ctx := context.Background()
	repo, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	defer repo.Close()
	first, err := repo.GetCard(ctx, cards[0].ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentDone, first.AgentStatus)
	require.Equal(t, "tester", first.AgentName)
	require.Contains(t, first.AgentResult, "processed")
	require.Equal(t, other.ID, first.ColumnID, "successful cards move to the done column")

	second, err := repo.GetCard(ctx, cards[1].ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentQueued, second.AgentStatus, "limit leaves the rest queued")
	idle, err := repo.GetCard(ctx, cards[2].ID)
	require.NoError(t, err)
	require.Empty(t, idle.AgentStatus, "unqueued cards are untouched")

	require.Len(t, notified, 1)
	require.Contains(t, notified[0], "First")
}

func TestAgentRunMarksFailures(t *testing.T) {
	dbPath, logPath, board, column, _ := addTestDatabase(t)
	cards := agentTestCards(t, dbPath, board, column)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[notifications]\nenabled = false\n"), 0o600))
	t.Setenv(config.EnvConfig, configPath)
	t.Setenv(config.EnvMCPToken, "")

	root := New("test", "abc123", "today")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "agent", "run",
		"--command", "echo boom >&2; exit 3", "--column", column.ID})
	require.NoError(t, root.Execute())

	repo, err := storage.Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer repo.Close()
	first, err := repo.GetCard(context.Background(), cards[0].ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentFailed, first.AgentStatus)
	require.Contains(t, first.AgentResult, "boom")
	require.Equal(t, column.ID, first.ColumnID, "without a failed column the card stays put")
}

func TestAgentRunRequiresColumn(t *testing.T) {
	dbPath, logPath, _, _, _ := addTestDatabase(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))
	t.Setenv(config.EnvConfig, configPath)
	t.Setenv(config.EnvMCPToken, "")

	root := New("test", "abc123", "today")
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "agent", "run", "--command", "true"})
	require.ErrorContains(t, root.Execute(), "column")
}

func TestAgentPromptIncludesCardContext(t *testing.T) {
	card := domain.Card{
		Title: "Fix login", Description: "Steps to reproduce",
		Checklist: []domain.ChecklistItem{{Text: "Reproduce", Done: true}, {Text: "Patch"}},
	}
	prompt := agentPrompt(card)
	require.Contains(t, prompt, "# Fix login")
	require.Contains(t, prompt, "Steps to reproduce")
	require.Contains(t, prompt, "- [x] Reproduce")
	require.Contains(t, prompt, "- [ ] Patch")
}
