package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestCardHistoryCommandReturnsMovements(t *testing.T) {
	dbPath, logPath, board, column, other := addTestDatabase(t)
	ctx := context.Background()
	repo, err := storage.Open(ctx, dbPath)
	require.NoError(t, err)
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Tracked", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &card))
	require.NoError(t, repo.MoveCard(ctx, card.ID, other.ID, 0))
	require.NoError(t, repo.Close())

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))
	t.Setenv(config.EnvConfig, configPath)
	t.Setenv(config.EnvMCPToken, "")

	root := New("test", "abc123", "today")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"--db", dbPath, "--log", logPath, "card", "history", card.ID})
	require.NoError(t, root.Execute())
	var movements []domain.CardMovement
	require.NoError(t, json.Unmarshal(output.Bytes(), &movements))
	require.Len(t, movements, 2)
	require.Equal(t, other.ID, movements[1].ToColumnID)
}
