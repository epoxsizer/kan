package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestMigrateSeedAndVersionCommands(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "kan.db")
	logPath := filepath.Join(dir, "kan.log")

	for _, command := range [][]string{
		{"--db", dbPath, "--log", logPath, "migrate"},
		{"--db", dbPath, "--log", logPath, "seed"},
		{"--db", dbPath, "--log", logPath, "seed"},
	} {
		root := New("test", "abc123", "today")
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs(command)
		require.NoError(t, root.Execute())
		require.NotEmpty(t, output.String())
	}

	root := New("test", "abc123", "today")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"--version"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), "commit abc123")
}

func TestRemovedSyncCommandIsUnavailable(t *testing.T) {
	root := New("test", "abc123", "today")
	root.SetArgs([]string{"sync"})
	require.ErrorContains(t, root.Execute(), `unknown command "sync"`)
}

func TestFirstRunCreatesConfigBesideBinary(t *testing.T) {
	directory := t.TempDir()
	executable, err := os.Executable()
	require.NoError(t, err)
	configPath := filepath.Join(filepath.Dir(executable), "config.toml")
	previousConfig, readErr := os.ReadFile(configPath)
	require.True(t, readErr == nil || os.IsNotExist(readErr))
	if readErr == nil {
		require.NoError(t, os.Remove(configPath))
	}
	t.Cleanup(func() {
		_ = os.Remove(configPath)
		if readErr == nil {
			require.NoError(t, os.WriteFile(configPath, previousConfig, 0o600))
		}
	})

	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(directory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	root := New("test", "abc123", "today")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"migrate"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), "migrations applied")
	require.FileExists(t, configPath)
	require.NoFileExists(t, filepath.Join(directory, "config.toml"))
	require.FileExists(t, filepath.Join(directory, "kan.db"))
	require.FileExists(t, filepath.Join(directory, "kan.log"))
}

func executeJSONCommand(t *testing.T, arguments []string, target any) error {
	t.Helper()
	root := New("test", "abc123", "today")
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs(arguments)
	if err := root.Execute(); err != nil {
		return err
	}
	if target != nil {
		require.NoError(t, json.Unmarshal(output.Bytes(), target), output.String())
	}
	return nil
}

func TestAutomationCLIWorkflow(t *testing.T) {
	directory := t.TempDir()
	dbPath := filepath.Join(directory, "kan.db")
	logPath := filepath.Join(directory, "kan.log")
	base := []string{"--db", dbPath, "--log", logPath}
	args := func(values ...string) []string { return append(append([]string{}, base...), values...) }

	var project domain.Project
	require.NoError(t, executeJSONCommand(t, args("project", "create", "--name", "Agent Project", "--comment", "Created by automation"), &project))
	require.NotEmpty(t, project.ID)
	require.Equal(t, float64(1024), project.Position)

	var projects []domain.Project
	require.NoError(t, executeJSONCommand(t, args("project", "list"), &projects))
	require.Len(t, projects, 1)

	var board domain.Board
	require.NoError(t, executeJSONCommand(t, args("board", "create", "--project", project.ID, "--name", "Delivery"), &board))
	var backlog, done domain.Column
	require.NoError(t, executeJSONCommand(t, args("column", "create", "--board", board.ID, "--name", "Backlog"), &backlog))
	require.NoError(t, executeJSONCommand(t, args("column", "create", "--board", board.ID, "--name", "Done"), &done))
	require.Greater(t, done.Position, backlog.Position)
	require.Equal(t, 10, *backlog.WIPLimit)
	require.Equal(t, "Blue", *backlog.Color)

	fields := `{"source":{"type":"text","value":"agent"}}`
	var card domain.Card
	require.NoError(t, executeJSONCommand(t, args("card", "create", "--board", board.ID, "--column", backlog.ID, "--title", "Automate release", "--comment", "Created over CLI", "--tags", "agent,release,agent", "--due", "2026-07-01", "--fields", fields), &card))
	require.Equal(t, []string{"agent", "release"}, card.Tags)
	require.Equal(t, "agent", card.Fields["source"].Value)
	var related domain.Card
	checklist := `[{"id":"verify","text":"Verify package","done":false,"position":1024}]`
	require.NoError(t, executeJSONCommand(t, args("card", "create", "--board", board.ID, "--column", backlog.ID, "--title", "Publish notes", "--checklist", checklist), &related))
	require.Equal(t, "Medium", *related.Priority)
	require.Nil(t, related.DueDate)
	require.Equal(t, "Verify package", related.Checklist[0].Text)

	var searchResults []domain.Card
	require.NoError(t, executeJSONCommand(t, args("card", "search", "--board", board.ID, "--query", "Automate"), &searchResults))
	require.Len(t, searchResults, 1)

	var updated domain.Card
	require.NoError(t, executeJSONCommand(t, args("card", "update", card.ID, "--column", done.ID, "--title", "Release automated", "--priority", "high", "--links", related.ID), &updated))
	require.Equal(t, done.ID, updated.ColumnID)
	require.Equal(t, "high", *updated.Priority)
	require.Equal(t, []string{related.ID}, updated.RelatedCardIDs)
	var relatedLoaded domain.Card
	require.NoError(t, executeJSONCommand(t, args("card", "get", related.ID), &relatedLoaded))
	require.Equal(t, []string{card.ID}, relatedLoaded.RelatedCardIDs)

	var doneCards []domain.Card
	require.NoError(t, executeJSONCommand(t, args("card", "list", "--board", board.ID, "--column", done.ID), &doneCards))
	require.Len(t, doneCards, 1)

	var archived map[string]string
	require.NoError(t, executeJSONCommand(t, args("card", "archive", card.ID), &archived))
	require.Equal(t, card.ID, archived["archived"])
	var archivedCards []domain.Card
	require.NoError(t, executeJSONCommand(t, args("card", "archived", "--board", board.ID, "--column", done.ID), &archivedCards))
	require.Len(t, archivedCards, 1)
	require.NotNil(t, archivedCards[0].DeletedAt)
	var restored domain.Card
	require.NoError(t, executeJSONCommand(t, args("card", "restore", card.ID), &restored))
	require.Equal(t, card.ID, restored.ID)
	require.Nil(t, restored.DeletedAt)

	require.ErrorContains(t, executeJSONCommand(t, args("card", "delete", card.ID), nil), "requires --yes")

	var deleted map[string]string
	require.NoError(t, executeJSONCommand(t, args("card", "delete", card.ID, "--yes"), &deleted))
	require.Equal(t, card.ID, deleted["deleted"])
	require.NoError(t, executeJSONCommand(t, args("card", "delete", related.ID, "--yes"), &deleted))
	require.NoError(t, executeJSONCommand(t, args("card", "list", "--board", board.ID), &doneCards))
	require.Empty(t, doneCards)
}

func TestBackupCommandWritesToWorkingDirectory(t *testing.T) {
	directory := t.TempDir()
	dbPath := filepath.Join(directory, "data", "kan.db")
	logPath := filepath.Join(directory, "state", "kan.log")

	seedCommand := New("test", "abc123", "today")
	seedCommand.SetArgs([]string{"--db", dbPath, "--log", logPath, "seed"})
	require.NoError(t, seedCommand.Execute())

	workingDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(directory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(workingDirectory)) })

	backupCommand := New("test", "abc123", "today")
	var output bytes.Buffer
	backupCommand.SetOut(&output)
	backupCommand.SetArgs([]string{"--db", dbPath, "--log", logPath, "backup", "before-upgrade"})
	require.NoError(t, backupCommand.Execute())
	require.Contains(t, output.String(), "backup created: backup/before-upgrade-")

	backups, err := filepath.Glob(filepath.Join(directory, "backup", "before-upgrade-*.db"))
	require.NoError(t, err)
	require.Len(t, backups, 1)

	invalidCommand := New("test", "abc123", "today")
	invalidCommand.SetArgs([]string{"--db", dbPath, "--log", logPath, "backup", "../escape"})
	require.ErrorContains(t, invalidCommand.Execute(), "backup name must")

	helpCommand := New("test", "abc123", "today")
	var help bytes.Buffer
	helpCommand.SetOut(&help)
	helpCommand.SetArgs([]string{"backup", "--help"})
	require.NoError(t, helpCommand.Execute())
	require.NotContains(t, help.String(), "--storage")
	require.NotContains(t, help.String(), "--s3-")
}

func TestExportCommandIncludesCompleteHierarchy(t *testing.T) {
	directory := t.TempDir()
	dbPath := filepath.Join(directory, "kan.db")
	logPath := filepath.Join(directory, "kan.log")
	base := []string{"--db", dbPath, "--log", logPath}
	args := func(values ...string) []string { return append(append([]string{}, base...), values...) }

	seedCommand := New("test", "abc123", "today")
	seedCommand.SetOut(&bytes.Buffer{})
	seedCommand.SetArgs(args("seed"))
	require.NoError(t, seedCommand.Execute())

	deleteCommand := New("test", "abc123", "today")
	deleteCommand.SetOut(&bytes.Buffer{})
	deleteCommand.SetArgs(args("card", "delete", "00000000-0000-4000-8000-000000000030", "--yes"))
	require.NoError(t, deleteCommand.Execute())

	var document domain.ExportDocument
	require.NoError(t, executeJSONCommand(t, args("export"), &document))
	require.Equal(t, "kan", document.Format)
	require.Equal(t, domain.ExportVersion, document.Version)
	require.False(t, document.ExportedAt.IsZero())
	require.Len(t, document.Projects, 3)
	require.Len(t, document.Projects[0].Boards, 2)
	board := document.Projects[0].Boards[0]
	require.Len(t, board.FieldDefs, 2)
	require.Len(t, board.Columns, 3)
	require.Equal(t, "Backlog", board.Columns[0].Name)
	cardCount, deletedCount := 0, 0
	for _, column := range board.Columns {
		cardCount += len(column.Cards)
		for _, card := range column.Cards {
			if card.DeletedAt != nil {
				deletedCount++
			}
		}
	}
	require.Equal(t, 3, cardCount)
	require.Equal(t, 1, deletedCount)

	outputPath := filepath.Join(directory, "exports", "all.json")
	fileCommand := New("test", "abc123", "today")
	fileCommand.SetOut(&bytes.Buffer{})
	fileCommand.SetArgs(args("export", "--out", outputPath))
	require.NoError(t, fileCommand.Execute())
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	contents, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	var fileDocument domain.ExportDocument
	require.NoError(t, json.Unmarshal(contents, &fileDocument))
	require.Len(t, fileDocument.Projects, 3)

	conflictCommand := New("test", "abc123", "today")
	conflictCommand.SetArgs(args("export", "--out", outputPath))
	require.ErrorContains(t, conflictCommand.Execute(), "already exists")
	forceCommand := New("test", "abc123", "today")
	forceCommand.SetOut(&bytes.Buffer{})
	forceCommand.SetArgs(args("export", "--out", outputPath, "--force"))
	require.NoError(t, forceCommand.Execute())
}

func TestExportEmptyDatabaseUsesEmptyArrays(t *testing.T) {
	directory := t.TempDir()
	var document domain.ExportDocument
	require.NoError(t, executeJSONCommand(t, []string{"--db", filepath.Join(directory, "empty.db"), "--log", filepath.Join(directory, "kan.log"), "export"}, &document))
	require.NotNil(t, document.Projects)
	require.Empty(t, document.Projects)
}

func TestJSONExportImportRoundTripAndReplaceGuard(t *testing.T) {
	directory := t.TempDir()
	oldWorkingDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(directory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWorkingDirectory)) })

	sourceDB := filepath.Join(directory, "source.db")
	targetDB := filepath.Join(directory, "target.db")
	logPath := filepath.Join(directory, "kan.log")
	exportPath := filepath.Join(directory, "roundtrip.json")
	sourceArgs := []string{"--db", sourceDB, "--log", logPath}
	targetArgs := []string{"--db", targetDB, "--log", logPath}

	seedCommand := New("test", "commit", "date")
	seedCommand.SetOut(&bytes.Buffer{})
	seedCommand.SetArgs(append(append([]string{}, sourceArgs...), "seed"))
	require.NoError(t, seedCommand.Execute())
	linkCommand := New("test", "commit", "date")
	linkCommand.SetOut(&bytes.Buffer{})
	linkCommand.SetArgs(append(append([]string{}, sourceArgs...), "card", "update", "00000000-0000-4000-8000-000000000030", "--links", "00000000-0000-4000-8000-000000000031"))
	require.NoError(t, linkCommand.Execute())
	deleteCommand := New("test", "commit", "date")
	deleteCommand.SetOut(&bytes.Buffer{})
	deleteCommand.SetArgs(append(append([]string{}, sourceArgs...), "card", "delete", "00000000-0000-4000-8000-000000000032", "--yes"))
	require.NoError(t, deleteCommand.Execute())
	exportCommand := New("test", "commit", "date")
	exportCommand.SetOut(&bytes.Buffer{})
	exportCommand.SetArgs(append(append([]string{}, sourceArgs...), "export", "--out", exportPath))
	require.NoError(t, exportCommand.Execute())

	importCommand := New("test", "commit", "date")
	var importOutput bytes.Buffer
	importCommand.SetOut(&importOutput)
	importCommand.SetArgs(append(append([]string{}, targetArgs...), "import", exportPath))
	require.NoError(t, importCommand.Execute())
	require.Contains(t, importOutput.String(), "import complete: 3 projects, 6 boards, 18 cards")

	var sourceDocument, targetDocument domain.ExportDocument
	require.NoError(t, executeJSONCommand(t, append(append([]string{}, sourceArgs...), "export"), &sourceDocument))
	require.NoError(t, executeJSONCommand(t, append(append([]string{}, targetArgs...), "export"), &targetDocument))
	require.Equal(t, sourceDocument.Projects, targetDocument.Projects)

	conflictCommand := New("test", "commit", "date")
	conflictCommand.SetArgs(append(append([]string{}, targetArgs...), "import", exportPath))
	require.ErrorContains(t, conflictCommand.Execute(), "database is not empty")
	guardCommand := New("test", "commit", "date")
	guardCommand.SetArgs(append(append([]string{}, targetArgs...), "import", exportPath, "--replace"))
	require.ErrorContains(t, guardCommand.Execute(), "requires --yes")
	replaceCommand := New("test", "commit", "date")
	replaceCommand.SetOut(&bytes.Buffer{})
	replaceCommand.SetArgs(append(append([]string{}, targetArgs...), "import", exportPath, "--replace", "--yes"))
	require.NoError(t, replaceCommand.Execute())
	backups, err := filepath.Glob(filepath.Join(directory, "backup", "kan-pre-import-*.db"))
	require.NoError(t, err)
	require.Len(t, backups, 1)
}
